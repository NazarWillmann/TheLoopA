package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

// CallExecutor interface defines the methods needed by API handlers
type CallExecutor interface {
	RunBasicScenario(destinationNumber string) error
	GetSystemStatus() map[string]interface{}
}

type APIHandlers struct {
	executor CallExecutor
	logger   zerolog.Logger
}

type MakeCallRequest struct {
	Destination string `json:"destination"`
	CallerID    string `json:"caller_id,omitempty"`
	UUI         string `json:"uui,omitempty"`
}

type MakeCallResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	CallID  string `json:"call_id,omitempty"`
	RefID   string `json:"ref_id,omitempty"`
}

type AnswerCallRequest struct {
	CallID  string `json:"call_id"`
	AgentID string `json:"agent_id,omitempty"`
}

type HangupCallRequest struct {
	CallID  string `json:"call_id"`
	AgentID string `json:"agent_id,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

type TransferCallRequest struct {
	CallID         string `json:"call_id"`
	AgentID        string `json:"agent_id,omitempty"`
	TransferTarget string `json:"transfer_target"`
	TransferType   string `json:"transfer_type,omitempty"`
}

type CallActionResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	CallID  string `json:"call_id,omitempty"`
}

type CallStatusResponse struct {
	Success bool                   `json:"success"`
	Call    map[string]interface{} `json:"call,omitempty"`
}

type StatusResponse struct {
	Success bool                    `json:"success"`
	Status  map[string]interface{}  `json:"status"`
	Version string                  `json:"version"`
}

type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

func NewAPIHandlers(executor CallExecutor, logger zerolog.Logger) *APIHandlers {
	return &APIHandlers{
		executor: executor,
		logger:   logger,
	}
}

func (h *APIHandlers) SetupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("/api/v1/call/make", h.handleMakeCall)
	mux.HandleFunc("/api/v1/call/answer", h.handleAnswerCall)
	mux.HandleFunc("/api/v1/call/hangup", h.handleHangupCall)
	mux.HandleFunc("/api/v1/call/transfer", h.handleTransferCall)
	mux.HandleFunc("/api/v1/call/status", h.handleCallStatus)
	mux.HandleFunc("/api/v1/status", h.handleStatus)
	mux.HandleFunc("/api/v1/health", h.handleHealth)

	// Add CORS middleware
	return h.corsMiddleware(mux)
}

func (h *APIHandlers) handleMakeCall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req MakeCallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error().Err(err).Msg("Failed to decode make call request")
		h.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Destination == "" {
		h.sendError(w, "Destination is required", http.StatusBadRequest)
		return
	}

	h.logger.Info().
		Str("destination", req.Destination).
		Str("caller_id", req.CallerID).
		Str("uui", req.UUI).
		Msg("API call request received")

	// Run the basic scenario with the provided destination
	if err := h.executor.RunBasicScenario(req.Destination); err != nil {
		h.logger.Error().Err(err).Msg("Failed to execute call scenario")
		h.sendError(w, fmt.Sprintf("Failed to make call: %v", err), http.StatusInternalServerError)
		return
	}

	response := MakeCallResponse{
		Success: true,
		Message: "Call initiated successfully",
		RefID:   fmt.Sprintf("api-call-%d", time.Now().Unix()),
	}

	h.sendJSON(w, response, http.StatusOK)
}

func (h *APIHandlers) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get system status from executor
	status := h.executor.GetSystemStatus()
	response := StatusResponse{
		Success: true,
		Status:  status,
		Version: "1.0.0",
	}

	h.sendJSON(w, response, http.StatusOK)
}

func (h *APIHandlers) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"status":  "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
	}

	h.sendJSON(w, response, http.StatusOK)
}

func (h *APIHandlers) handleAnswerCall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AnswerCallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error().Err(err).Msg("Failed to decode answer call request")
		h.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.CallID == "" {
		h.sendError(w, "Call ID is required", http.StatusBadRequest)
		return
	}

	h.logger.Info().
		Str("call_id", req.CallID).
		Str("agent_id", req.AgentID).
		Msg("API answer call request received")

	// For now, just return success - actual implementation would interact with call state
	response := CallActionResponse{
		Success: true,
		Message: "Call answer request processed",
		CallID:  req.CallID,
	}

	h.sendJSON(w, response, http.StatusOK)
}

func (h *APIHandlers) handleHangupCall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req HangupCallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error().Err(err).Msg("Failed to decode hangup call request")
		h.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.CallID == "" {
		h.sendError(w, "Call ID is required", http.StatusBadRequest)
		return
	}

	h.logger.Info().
		Str("call_id", req.CallID).
		Str("agent_id", req.AgentID).
		Str("reason", req.Reason).
		Msg("API hangup call request received")

	// For now, just return success - actual implementation would interact with call state
	response := CallActionResponse{
		Success: true,
		Message: "Call hangup request processed",
		CallID:  req.CallID,
	}

	h.sendJSON(w, response, http.StatusOK)
}

func (h *APIHandlers) handleTransferCall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req TransferCallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error().Err(err).Msg("Failed to decode transfer call request")
		h.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.CallID == "" || req.TransferTarget == "" {
		h.sendError(w, "Call ID and transfer target are required", http.StatusBadRequest)
		return
	}

	h.logger.Info().
		Str("call_id", req.CallID).
		Str("agent_id", req.AgentID).
		Str("transfer_target", req.TransferTarget).
		Str("transfer_type", req.TransferType).
		Msg("API transfer call request received")

	// For now, just return success - actual implementation would interact with call state
	response := CallActionResponse{
		Success: true,
		Message: "Call transfer request processed",
		CallID:  req.CallID,
	}

	h.sendJSON(w, response, http.StatusOK)
}

func (h *APIHandlers) handleCallStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	callID := r.URL.Query().Get("call_id")
	if callID == "" {
		h.sendError(w, "Call ID parameter is required", http.StatusBadRequest)
		return
	}

	h.logger.Info().
		Str("call_id", callID).
		Msg("API call status request received")

	// For now, return a mock call status - actual implementation would query call state
	response := CallStatusResponse{
		Success: true,
		Call: map[string]interface{}{
			"call_id": callID,
			"state":   "UNKNOWN",
			"message": "Call status retrieval not fully implemented",
		},
	}

	h.sendJSON(w, response, http.StatusOK)
}

func (h *APIHandlers) sendJSON(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error().Err(err).Msg("Failed to encode JSON response")
	}
}

func (h *APIHandlers) sendError(w http.ResponseWriter, message string, statusCode int) {
	response := ErrorResponse{
		Success: false,
		Error:   message,
	}
	h.sendJSON(w, response, statusCode)
}

func (h *APIHandlers) corsMiddleware(next *http.ServeMux) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})

	return mux
}
