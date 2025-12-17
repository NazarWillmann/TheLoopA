package asterisk

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"TheLoopA/internal/domain"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

type ARIEvent struct {
	Type        string      `json:"type"`
	Timestamp   string      `json:"timestamp"`
	Application string      `json:"application"`
	Channel     *Channel    `json:"channel,omitempty"`
	Bridge      interface{} `json:"bridge,omitempty"`
	Endpoint    interface{} `json:"endpoint,omitempty"`
}

type EventHandler interface {
	HandleChannelCreated(event ARIEvent)
	HandleChannelStateChange(event ARIEvent)
	HandleChannelDestroyed(event ARIEvent)
}

type ARIEventClient struct {
	baseURL       string
	username      string
	password      string
	application   string
	conn          *websocket.Conn
	eventHandler  EventHandler
	logger        zerolog.Logger
	reconnectChan chan struct{}
	closeChan     chan struct{}
	mutex         sync.RWMutex
	connected     bool
	ctx           context.Context
	cancel        context.CancelFunc
}

func NewARIEventClient(baseURL, username, password, application string, eventHandler EventHandler, logger zerolog.Logger) *ARIEventClient {
	ctx, cancel := context.WithCancel(context.Background())

	return &ARIEventClient{
		baseURL:       baseURL,
		username:      username,
		password:      password,
		application:   application,
		eventHandler:  eventHandler,
		logger:        logger,
		reconnectChan: make(chan struct{}, 1),
		closeChan:     make(chan struct{}),
		ctx:           ctx,
		cancel:        cancel,
	}
}

func (c *ARIEventClient) Connect() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.connected {
		return nil
	}

	// Build WebSocket URL
	wsURL := fmt.Sprintf("ws%s/ari/events?app=%s&api_key=%s:%s",
		c.baseURL[4:], // Remove "http" prefix
		c.application,
		c.username,
		c.password)

	c.logger.Debug().Str("url", wsURL).Msg("Connecting to ARI WebSocket")

	dialer := websocket.Dialer{
		HandshakeTimeout: 30 * time.Second,
	}

	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to ARI WebSocket: %w", err)
	}

	c.conn = conn
	c.connected = true

	c.logger.Info().Str("application", c.application).Msg("Connected to ARI WebSocket")

	// Start event handling goroutines
	go c.readEvents()
	go c.handleReconnect()

	return nil
}

func (c *ARIEventClient) Disconnect() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.connected {
		return nil
	}

	c.cancel()
	close(c.closeChan)

	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		c.connected = false
		c.logger.Info().Msg("Disconnected from ARI WebSocket")
		return err
	}

	return nil
}

func (c *ARIEventClient) IsConnected() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.connected
}

func (c *ARIEventClient) readEvents() {
	defer func() {
		c.mutex.Lock()
		c.connected = false
		c.mutex.Unlock()
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			if c.conn == nil {
				return
			}

			_, data, err := c.conn.ReadMessage()
			if err != nil {
				c.logger.Error().Err(err).Msg("Failed to read ARI event")
				c.triggerReconnect()
				return
			}

			var event ARIEvent
			if err := json.Unmarshal(data, &event); err != nil {
				c.logger.Error().Err(err).Str("data", string(data)).Msg("Failed to unmarshal ARI event")
				continue
			}

			c.logger.Debug().
				Str("type", event.Type).
				Str("timestamp", event.Timestamp).
				Msg("Received ARI event")

			// Dispatch event to handler
			c.dispatchEvent(event)
		}
	}
}

func (c *ARIEventClient) dispatchEvent(event ARIEvent) {
	if c.eventHandler == nil {
		return
	}

	switch event.Type {
	case "ChannelCreated":
		go c.eventHandler.HandleChannelCreated(event)
	case "ChannelStateChange":
		go c.eventHandler.HandleChannelStateChange(event)
	case "ChannelDestroyed":
		go c.eventHandler.HandleChannelDestroyed(event)
	default:
		c.logger.Debug().Str("type", event.Type).Msg("Unhandled ARI event type")
	}
}

func (c *ARIEventClient) handleReconnect() {
	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.closeChan:
			return
		case <-c.reconnectChan:
			c.logger.Info().Msg("Attempting to reconnect to ARI WebSocket")

			for {
				select {
				case <-c.ctx.Done():
					return
				case <-c.closeChan:
					return
				default:
					if err := c.Connect(); err != nil {
						c.logger.Error().Err(err).
							Dur("backoff", backoff).
							Msg("ARI reconnection failed, retrying")

						time.Sleep(backoff)
						if backoff < maxBackoff {
							backoff *= 2
						}
					} else {
						c.logger.Info().Msg("Successfully reconnected to ARI WebSocket")
						backoff = time.Second // Reset backoff
						goto reconnected
					}
				}
			}
		reconnected:
		}
	}
}

func (c *ARIEventClient) triggerReconnect() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.connected {
		c.connected = false
		if c.conn != nil {
			err := c.conn.Close()
			if err != nil {
				return
			}
			c.conn = nil
		}

		select {
		case c.reconnectChan <- struct{}{}:
		default:
		}
	}
}

// DefaultEventHandler Default event handler implementation
type DefaultEventHandler struct {
	stateManager *domain.StateManager
	ctiHandlers  CTIEventSender
	logger       zerolog.Logger
}

type CTIEventSender interface {
	SendCallConnectedEvent(call *domain.Call) error
	SendCallEndedEvent(call *domain.Call, reason string) error
	SendCallFailedEvent(call *domain.Call, reason string) error
}

func NewDefaultEventHandler(stateManager *domain.StateManager, ctiHandlers CTIEventSender, logger zerolog.Logger) *DefaultEventHandler {
	return &DefaultEventHandler{
		stateManager: stateManager,
		ctiHandlers:  ctiHandlers,
		logger:       logger,
	}
}

func (h *DefaultEventHandler) SetCTIEventSender(sender CTIEventSender) {
	h.ctiHandlers = sender
}

func (h *DefaultEventHandler) HandleChannelCreated(event ARIEvent) {
	if event.Channel == nil {
		return
	}

	h.logger.Info().
		Str("channel_id", event.Channel.ID).
		Str("channel_name", event.Channel.Name).
		Str("state", event.Channel.State).
		Msg("Channel created")

	// Find call by channel ID and update state to DIALING
	call, exists := h.stateManager.CallManager.GetCallByChannel(event.Channel.ID)
	if exists {
		h.stateManager.CallManager.UpdateCallState(call.ID, domain.CallStateDialing)
		h.logger.Info().
			Str("call_id", call.ID).
			Str("channel_id", event.Channel.ID).
			Msg("Updated call state to DIALING")
	}
}

func (h *DefaultEventHandler) HandleChannelStateChange(event ARIEvent) {
	if event.Channel == nil {
		return
	}

	h.logger.Info().
		Str("channel_id", event.Channel.ID).
		Str("channel_name", event.Channel.Name).
		Str("state", event.Channel.State).
		Msg("Channel state changed")

	// Find call by channel ID
	call, exists := h.stateManager.CallManager.GetCallByChannel(event.Channel.ID)
	if !exists {
		h.logger.Debug().
			Str("channel_id", event.Channel.ID).
			Msg("No call found for channel state change")
		return
	}

	// Map Asterisk channel states to call states
	var newCallState domain.CallState
	var sendEvent bool

	switch event.Channel.State {
	case "Ring":
		newCallState = domain.CallStateRinging
	case "Up":
		newCallState = domain.CallStateInProgress
		sendEvent = true
	case "Down":
		// Channel is down, call ended
		newCallState = domain.CallStateEnded
		sendEvent = true
	default:
		h.logger.Debug().
			Str("channel_state", event.Channel.State).
			Msg("Unhandled channel state")
		return
	}

	// Update call state
	if err := h.stateManager.CallManager.UpdateCallState(call.ID, newCallState); err != nil {
		h.logger.Error().Err(err).Msg("Failed to update call state")
		return
	}

	h.logger.Info().
		Str("call_id", call.ID).
		Str("channel_id", event.Channel.ID).
		Str("new_state", string(newCallState)).
		Msg("Updated call state")

	// Send CTI events for significant state changes
	if sendEvent && h.ctiHandlers != nil {
		switch newCallState {
		case domain.CallStateInProgress:
			if err := h.ctiHandlers.SendCallConnectedEvent(call); err != nil {
				h.logger.Error().Err(err).Msg("Failed to send call connected event")
			}
		case domain.CallStateEnded:
			if err := h.ctiHandlers.SendCallEndedEvent(call, "NORMAL"); err != nil {
				h.logger.Error().Err(err).Msg("Failed to send call ended event")
			}
		}
	}
}

func (h *DefaultEventHandler) HandleChannelDestroyed(event ARIEvent) {
	if event.Channel == nil {
		return
	}

	h.logger.Info().
		Str("channel_id", event.Channel.ID).
		Str("channel_name", event.Channel.Name).
		Msg("Channel destroyed")

	// Find call by channel ID
	call, exists := h.stateManager.CallManager.GetCallByChannel(event.Channel.ID)
	if !exists {
		h.logger.Debug().
			Str("channel_id", event.Channel.ID).
			Msg("No call found for channel destruction")
		return
	}

	// Determine if call ended normally or failed
	var reason string
	var newState domain.CallState

	if call.State == domain.CallStateInProgress {
		reason = "NORMAL"
		newState = domain.CallStateEnded
	} else {
		reason = "FAILED"
		newState = domain.CallStateFailed
	}

	// Update call state
	if err := h.stateManager.EndCall(call.ID, newState); err != nil {
		h.logger.Error().Err(err).Msg("Failed to end call")
		return
	}

	h.logger.Info().
		Str("call_id", call.ID).
		Str("channel_id", event.Channel.ID).
		Str("reason", reason).
		Msg("Call ended due to channel destruction")

	// Send CTI event
	if h.ctiHandlers != nil {
		if newState == domain.CallStateEnded {
			if err := h.ctiHandlers.SendCallEndedEvent(call, reason); err != nil {
				h.logger.Error().Err(err).Msg("Failed to send call ended event")
			}
		} else {
			if err := h.ctiHandlers.SendCallFailedEvent(call, reason); err != nil {
				h.logger.Error().Err(err).Msg("Failed to send call failed event")
			}
		}
	}
}
