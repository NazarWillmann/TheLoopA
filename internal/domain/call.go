package domain

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type CallState string

const (
	CallStateCreated    CallState = "CREATED"
	CallStateDialing    CallState = "DIALING"
	CallStateRinging    CallState = "RINGING"
	CallStateInProgress CallState = "IN_PROGRESS"
	CallStateEnded      CallState = "ENDED"
	CallStateFailed     CallState = "FAILED"
)

type Call struct {
	ID                string    `json:"id"`
	AgentID           string    `json:"agent_id"`
	RefID             string    `json:"ref_id"`
	DestinationNumber string    `json:"destination_number"`
	UUI               string    `json:"uui,omitempty"`
	State             CallState `json:"state"`
	AsteriskChannelID string    `json:"asterisk_channel_id,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	StartedAt         *time.Time `json:"started_at,omitempty"`
	EndedAt           *time.Time `json:"ended_at,omitempty"`
}

type CallManager struct {
	calls         map[string]*Call
	callsByRefID  map[string]*Call
	callsByAgent  map[string]*Call
	callsByChannel map[string]*Call
	mutex         sync.RWMutex
}

func NewCallManager() *CallManager {
	return &CallManager{
		calls:         make(map[string]*Call),
		callsByRefID:  make(map[string]*Call),
		callsByAgent:  make(map[string]*Call),
		callsByChannel: make(map[string]*Call),
	}
}

func (cm *CallManager) CreateCall(agentID, refID, destinationNumber, uui string) *Call {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	call := &Call{
		ID:                uuid.New().String(),
		AgentID:           agentID,
		RefID:             refID,
		DestinationNumber: destinationNumber,
		UUI:               uui,
		State:             CallStateCreated,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	cm.calls[call.ID] = call
	cm.callsByRefID[refID] = call
	cm.callsByAgent[agentID] = call

	return call
}

func (cm *CallManager) GetCall(id string) (*Call, bool) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	call, exists := cm.calls[id]
	return call, exists
}

func (cm *CallManager) GetCallByRefID(refID string) (*Call, bool) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	call, exists := cm.callsByRefID[refID]
	return call, exists
}

func (cm *CallManager) GetCallByAgent(agentID string) (*Call, bool) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	call, exists := cm.callsByAgent[agentID]
	return call, exists
}

func (cm *CallManager) GetCallByChannel(channelID string) (*Call, bool) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	call, exists := cm.callsByChannel[channelID]
	return call, exists
}

func (cm *CallManager) UpdateCallState(id string, state CallState) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	call, exists := cm.calls[id]
	if !exists {
		return ErrCallNotFound
	}

	call.State = state
	call.UpdatedAt = time.Now()

	// Set timestamps based on state
	now := time.Now()
	switch state {
	case CallStateInProgress:
		if call.StartedAt == nil {
			call.StartedAt = &now
		}
	case CallStateEnded, CallStateFailed:
		if call.EndedAt == nil {
			call.EndedAt = &now
		}
	}

	return nil
}

func (cm *CallManager) SetCallChannel(id string, channelID string) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	call, exists := cm.calls[id]
	if !exists {
		return ErrCallNotFound
	}

	// Remove old channel mapping if exists
	if call.AsteriskChannelID != "" {
		delete(cm.callsByChannel, call.AsteriskChannelID)
	}

	call.AsteriskChannelID = channelID
	call.UpdatedAt = time.Now()

	// Add new channel mapping
	if channelID != "" {
		cm.callsByChannel[channelID] = call
	}

	return nil
}

func (cm *CallManager) RemoveCall(id string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	call, exists := cm.calls[id]
	if !exists {
		return
	}

	// Remove from all mappings
	delete(cm.calls, id)
	delete(cm.callsByRefID, call.RefID)
	delete(cm.callsByAgent, call.AgentID)
	if call.AsteriskChannelID != "" {
		delete(cm.callsByChannel, call.AsteriskChannelID)
	}
}

func (cm *CallManager) GetAllCalls() map[string]*Call {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	result := make(map[string]*Call)
	for k, v := range cm.calls {
		result[k] = v
	}
	return result
}

func (cm *CallManager) CleanupCallsForAgent(agentID string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Find and cleanup calls for the agent
	for _, call := range cm.calls {
		if call.AgentID == agentID {
			call.State = CallStateEnded
			now := time.Now()
			call.EndedAt = &now
			call.UpdatedAt = now
		}
	}
}
