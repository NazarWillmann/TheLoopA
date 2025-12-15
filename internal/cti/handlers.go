package cti

import (
	"encoding/json"
	"fmt"
	"time"

	"TheLoopA/internal/domain"
	"TheLoopA/internal/ws"
	"github.com/rs/zerolog"
)

type HandlerContext struct {
	StateManager    *domain.StateManager
	WSClient        *ws.Client
	AsteriskClient  AsteriskClient
	Logger          zerolog.Logger
	CurrentAgentID  string
}

// AsteriskClient interface for dependency injection
type AsteriskClient interface {
	Originate(endpoint, callerId string, variables map[string]string) (string, error)
}

type Handlers struct {
	ctx *HandlerContext
}

func NewHandlers(ctx *HandlerContext) *Handlers {
	return &Handlers{ctx: ctx}
}

func (h *Handlers) RegisterHandlers() {
	h.ctx.WSClient.RegisterHandler("agentRegister", ws.MessageHandlerFunc(h.handleAgentRegister))
	h.ctx.WSClient.RegisterHandler("agentLogin", ws.MessageHandlerFunc(h.handleAgentLogin))
	h.ctx.WSClient.RegisterHandler("agentChangeState", ws.MessageHandlerFunc(h.handleAgentChangeState))
	h.ctx.WSClient.RegisterHandler("agentLogout", ws.MessageHandlerFunc(h.handleAgentLogout))
	h.ctx.WSClient.RegisterHandler("lineMakeCall", ws.MessageHandlerFunc(h.handleLineMakeCall))
	h.ctx.WSClient.RegisterHandler("lineAnswerCall", ws.MessageHandlerFunc(h.handleLineAnswerCall))
	h.ctx.WSClient.RegisterHandler("lineSetupTransfer", ws.MessageHandlerFunc(h.handleLineSetupTransfer))
	h.ctx.WSClient.RegisterHandler("lineCompleteSPYcalls", ws.MessageHandlerFunc(h.handleLineCompleteSPYcalls))
}

func (h *Handlers) handleAgentRegister(message ws.Message) error {
	h.ctx.Logger.Info().Msg("Handling agentRegister")

	var msg AgentRegisterMessage
	if err := h.parseMessage(message.Body, &msg); err != nil {
		return err
	}

	// Create agent in memory
	agent := h.ctx.StateManager.AgentManager.CreateAgent()
	h.ctx.CurrentAgentID = agent.ID

	h.ctx.Logger.Info().
		Str("agent_id", agent.ID).
		Str("state", string(agent.State)).
		Msg("Agent registered successfully")

	// Send success response if refId is provided
	if msg.RefID != "" {
		return h.sendSuccessResponse(msg.RefID, "Agent registered successfully")
	}

	return nil
}

func (h *Handlers) handleAgentLogin(message ws.Message) error {
	h.ctx.Logger.Info().Msg("Handling agentLogin")

	var msg AgentLoginMessage
	if err := h.parseMessage(message.Body, &msg); err != nil {
		return err
	}

	agentID := h.getAgentID(msg.AgentID)
	if agentID == "" {
		return h.sendErrorResponse(msg.RefID, "No agent registered", 400)
	}

	// Update agent state to LOGGED_IN
	if err := h.ctx.StateManager.AgentManager.UpdateAgentState(agentID, domain.AgentStateLoggedIn); err != nil {
		h.ctx.Logger.Error().Err(err).Msg("Failed to update agent state")
		return h.sendErrorResponse(msg.RefID, "Failed to login agent", 500)
	}

	h.ctx.Logger.Info().
		Str("agent_id", agentID).
		Msg("Agent logged in successfully")

	if msg.RefID != "" {
		return h.sendSuccessResponse(msg.RefID, "Agent logged in successfully")
	}

	return nil
}

func (h *Handlers) handleAgentChangeState(message ws.Message) error {
	h.ctx.Logger.Info().Msg("Handling agentChangeState")

	var msg AgentChangeStateMessage
	if err := h.parseMessage(message.Body, &msg); err != nil {
		return err
	}

	agentID := h.getAgentID(msg.AgentID)
	if agentID == "" {
		return h.sendErrorResponse(msg.RefID, "No agent registered", 400)
	}

	var newState domain.AgentState
	switch msg.State {
	case AgentStateCodeReady:
		newState = domain.AgentStateReady
	case AgentStateCodeBusy:
		newState = domain.AgentStateBusy
	default:
		return h.sendErrorResponse(msg.RefID, fmt.Sprintf("Unknown state code: %d", msg.State), 400)
	}

	if err := h.ctx.StateManager.AgentManager.UpdateAgentState(agentID, newState); err != nil {
		h.ctx.Logger.Error().Err(err).Msg("Failed to update agent state")
		return h.sendErrorResponse(msg.RefID, "Failed to change agent state", 500)
	}

	h.ctx.Logger.Info().
		Str("agent_id", agentID).
		Str("new_state", string(newState)).
		Msg("Agent state changed successfully")

	if msg.RefID != "" {
		return h.sendSuccessResponse(msg.RefID, "Agent state changed successfully")
	}

	return nil
}

func (h *Handlers) handleAgentLogout(message ws.Message) error {
	h.ctx.Logger.Info().Msg("Handling agentLogout")

	var msg AgentLogoutMessage
	if err := h.parseMessage(message.Body, &msg); err != nil {
		return err
	}

	agentID := h.getAgentID(msg.AgentID)
	if agentID == "" {
		return h.sendErrorResponse(msg.RefID, "No agent registered", 400)
	}

	// Cleanup agent and calls
	h.ctx.StateManager.CleanupAgent(agentID)

	h.ctx.Logger.Info().
		Str("agent_id", agentID).
		Msg("Agent logged out successfully")

	if msg.RefID != "" {
		return h.sendSuccessResponse(msg.RefID, "Agent logged out successfully")
	}

	return nil
}

func (h *Handlers) handleLineMakeCall(message ws.Message) error {
	h.ctx.Logger.Info().Msg("Handling lineMakeCall")

	var msg LineMakeCallMessage
	if err := h.parseMessage(message.Body, &msg); err != nil {
		return err
	}

	agentID := h.getAgentID(msg.AgentID)
	if agentID == "" {
		return h.sendErrorResponse(msg.RefID, "No agent registered", 400)
	}

	// Check if agent is ready
	if !h.ctx.StateManager.IsAgentReady(agentID) {
		return h.sendErrorResponse(msg.RefID, "Agent is not ready", 400)
	}

	// Start call in state manager
	call, err := h.ctx.StateManager.StartCall(agentID, msg.RefID, msg.DestinationNumber, msg.UUI)
	if err != nil {
		h.ctx.Logger.Error().Err(err).Msg("Failed to start call")
		return h.sendErrorResponse(msg.RefID, "Failed to start call", 500)
	}

	// Process call in a separate goroutine
	go h.processCallAsync(call, msg)

	// Send immediate response that call was initiated
	if msg.RefID != "" {
		return h.sendSuccessResponse(msg.RefID, "Call initiated successfully")
	}

	return nil
}

// processCallAsync handles the actual call processing in a separate goroutine
func (h *Handlers) processCallAsync(call *domain.Call, msg LineMakeCallMessage) {
	h.ctx.Logger.Info().
		Str("call_id", call.ID).
		Str("agent_id", call.AgentID).
		Str("destination", msg.DestinationNumber).
		Msg("Processing call asynchronously")

	// Prepare variables for Asterisk
	variables := map[string]string{
		"UUI":     msg.UUI,
		"agentId": call.AgentID,
		"refId":   msg.RefID,
		"callId":  call.ID,
	}

	// Make Asterisk originate call
	endpoint := fmt.Sprintf("PJSIP/%s@bank-out", msg.DestinationNumber)
	channelID, err := h.ctx.AsteriskClient.Originate(endpoint, "EmulatorService", variables)
	if err != nil {
		h.ctx.Logger.Error().Err(err).
			Str("call_id", call.ID).
			Msg("Failed to originate call in Asterisk")

		// Update call state to failed
		h.ctx.StateManager.EndCall(call.ID, domain.CallStateFailed)

		// Send call failed event
		if err := h.SendCallFailedEvent(call, fmt.Sprintf("Failed to originate call: %v", err)); err != nil {
			h.ctx.Logger.Error().Err(err).Msg("Failed to send call failed event")
		}
		return
	}

	// Update call with channel ID and set state to dialing
	h.ctx.StateManager.CallManager.SetCallChannel(call.ID, channelID)
	h.ctx.StateManager.CallManager.UpdateCallState(call.ID, domain.CallStateDialing)

	h.ctx.Logger.Info().
		Str("agent_id", call.AgentID).
		Str("call_id", call.ID).
		Str("channel_id", channelID).
		Str("destination", msg.DestinationNumber).
		Msg("Call originated successfully in goroutine")

	// Send call started event
	if err := h.SendCallStartedEvent(call); err != nil {
		h.ctx.Logger.Error().Err(err).Msg("Failed to send call started event")
	}
}

func (h *Handlers) handleLineAnswerCall(message ws.Message) error {
	h.ctx.Logger.Info().Msg("Handling lineAnswerCall")

	var msg LineAnswerCallMessage
	if err := h.parseMessage(message.Body, &msg); err != nil {
		return err
	}

	// For emulator: if call is already IN_PROGRESS, do nothing
	// Otherwise, reject the request
	agentID := h.getAgentID(msg.AgentID)
	if agentID == "" {
		return h.sendErrorResponse(msg.RefID, "No agent registered", 400)
	}

	call, hasCall := h.ctx.StateManager.GetCallByAgent(agentID)
	if !hasCall {
		return h.sendErrorResponse(msg.RefID, "No active call found", 400)
	}

	if call.State == domain.CallStateInProgress {
		h.ctx.Logger.Info().
			Str("call_id", call.ID).
			Msg("Call already in progress, ignoring answer request")

		if msg.RefID != "" {
			return h.sendSuccessResponse(msg.RefID, "Call already answered")
		}
		return nil
	}

	h.ctx.Logger.Warn().
		Str("call_id", call.ID).
		Str("call_state", string(call.State)).
		Msg("Cannot answer call in current state")

	return h.sendErrorResponse(msg.RefID, "Cannot answer call in current state", 400)
}

func (h *Handlers) handleLineSetupTransfer(message ws.Message) error {
	h.ctx.Logger.Info().Msg("Handling lineSetupTransfer")

	var msg LineSetupTransferMessage
	if err := h.parseMessage(message.Body, &msg); err != nil {
		return err
	}

	// MVP: Just log the transfer request, don't implement SIP transfer
	h.ctx.Logger.Info().
		Str("call_id", msg.CallID).
		Str("agent_id", msg.AgentID).
		Str("transfer_target", msg.TransferTarget).
		Str("transfer_type", msg.TransferType).
		Msg("Transfer request received (not implemented in MVP)")

	if msg.RefID != "" {
		return h.sendSuccessResponse(msg.RefID, "Transfer request logged (not implemented)")
	}

	return nil
}

func (h *Handlers) handleLineCompleteSPYcalls(message ws.Message) error {
	h.ctx.Logger.Info().Msg("Handling lineCompleteSPYcalls")

	var msg LineCompleteSPYCallsMessage
	if err := h.parseMessage(message.Body, &msg); err != nil {
		return err
	}

	// MVP: Just log the SPY request, don't implement
	h.ctx.Logger.Info().
		Str("agent_id", msg.AgentID).
		Msg("SPY calls request received (not implemented in MVP)")

	if msg.RefID != "" {
		return h.sendSuccessResponse(msg.RefID, "SPY request logged (not implemented)")
	}

	return nil
}

// Helper methods

func (h *Handlers) parseMessage(body interface{}, target interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal message body: %w", err)
	}

	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return nil
}

func (h *Handlers) getAgentID(msgAgentID string) string {
	if msgAgentID != "" {
		return msgAgentID
	}
	return h.ctx.CurrentAgentID
}

func (h *Handlers) sendSuccessResponse(refID, message string) error {
	response := ws.Message{
		Name: "success",
		Body: SuccessResponse{
			RefID:   refID,
			Message: message,
		},
	}
	return h.ctx.WSClient.SendMessage(response)
}

func (h *Handlers) sendErrorResponse(refID, errorMsg string, code int) error {
	response := ws.Message{
		Name: "error",
		Body: ErrorResponse{
			RefID: refID,
			Error: errorMsg,
			Code:  code,
		},
	}
	return h.ctx.WSClient.SendMessage(response)
}

// Event sending methods for Asterisk integration

func (h *Handlers) SendCallStartedEvent(call *domain.Call) error {
	event := ws.Message{
		Name: "callStarted",
		Body: CallStartedEvent{
			CallID:            call.ID,
			AgentID:           call.AgentID,
			RefID:             call.RefID,
			DestinationNumber: call.DestinationNumber,
			Timestamp:         time.Now().Format(time.RFC3339),
		},
	}
	return h.ctx.WSClient.SendMessage(event)
}

func (h *Handlers) SendCallConnectedEvent(call *domain.Call) error {
	event := ws.Message{
		Name: "callConnected",
		Body: CallConnectedEvent{
			CallID:            call.ID,
			AgentID:           call.AgentID,
			RefID:             call.RefID,
			DestinationNumber: call.DestinationNumber,
			Timestamp:         time.Now().Format(time.RFC3339),
		},
	}
	return h.ctx.WSClient.SendMessage(event)
}

func (h *Handlers) SendCallEndedEvent(call *domain.Call, reason string) error {
	duration := 0
	if call.StartedAt != nil && call.EndedAt != nil {
		duration = int(call.EndedAt.Sub(*call.StartedAt).Seconds())
	}

	event := ws.Message{
		Name: "callEnded",
		Body: CallEndedEvent{
			CallID:    call.ID,
			AgentID:   call.AgentID,
			RefID:     call.RefID,
			Reason:    reason,
			Duration:  duration,
			Timestamp: time.Now().Format(time.RFC3339),
		},
	}
	return h.ctx.WSClient.SendMessage(event)
}

func (h *Handlers) SendCallFailedEvent(call *domain.Call, reason string) error {
	event := ws.Message{
		Name: "callFailed",
		Body: CallFailedEvent{
			CallID:    call.ID,
			AgentID:   call.AgentID,
			RefID:     call.RefID,
			Reason:    reason,
			Timestamp: time.Now().Format(time.RFC3339),
		},
	}
	return h.ctx.WSClient.SendMessage(event)
}
