package domain

import (
	"sync"
)

// StateManager provides centralized state management for agents and calls
type StateManager struct {
	AgentManager *AgentManager
	CallManager  *CallManager
	mutex        sync.RWMutex
}

// NewStateManager creates a new state manager instance
func NewStateManager() *StateManager {
	return &StateManager{
		AgentManager: NewAgentManager(),
		CallManager:  NewCallManager(),
	}
}

// GetAgentByCall returns the agent associated with a call
func (sm *StateManager) GetAgentByCall(callID string) (*Agent, bool) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	call, exists := sm.CallManager.GetCall(callID)
	if !exists {
		return nil, false
	}

	return sm.AgentManager.GetAgent(call.AgentID)
}

// GetCallByAgent returns the active call for an agent
func (sm *StateManager) GetCallByAgent(agentID string) (*Call, bool) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	return sm.CallManager.GetCallByAgent(agentID)
}

// IsAgentReady checks if an agent is in READY state
func (sm *StateManager) IsAgentReady(agentID string) bool {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	agent, exists := sm.AgentManager.GetAgent(agentID)
	if !exists {
		return false
	}

	return agent.State == AgentStateReady
}

// IsAgentBusy checks if an agent is in BUSY state or has an active call
func (sm *StateManager) IsAgentBusy(agentID string) bool {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	agent, exists := sm.AgentManager.GetAgent(agentID)
	if !exists {
		return false
	}

	if agent.State == AgentStateBusy {
		return true
	}

	// Check if agent has an active call
	call, hasCall := sm.CallManager.GetCallByAgent(agentID)
	if hasCall && (call.State == CallStateDialing || call.State == CallStateRinging || call.State == CallStateInProgress) {
		return true
	}

	return false
}

// StartCall creates a call and updates agent state to BUSY
func (sm *StateManager) StartCall(agentID, refID, destinationNumber, uui string) (*Call, error) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// Check if agent exists and is ready
	agent, exists := sm.AgentManager.GetAgent(agentID)
	if !exists {
		return nil, ErrAgentNotFound
	}

	if agent.State != AgentStateReady {
		return nil, ErrAgentNotReady
	}

	// Check if agent already has an active call
	if existingCall, hasCall := sm.CallManager.GetCallByAgent(agentID); hasCall {
		if existingCall.State == CallStateDialing || existingCall.State == CallStateRinging || existingCall.State == CallStateInProgress {
			return nil, ErrCallInProgress
		}
	}

	// Create the call
	call := sm.CallManager.CreateCall(agentID, refID, destinationNumber, uui)

	// Update agent state to BUSY
	agent.State = AgentStateBusy
	agent.CallID = call.ID
	agent.LastRefID = refID

	return call, nil
}

// EndCall ends a call and updates agent state back to READY
func (sm *StateManager) EndCall(callID string, state CallState) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	call, exists := sm.CallManager.GetCall(callID)
	if !exists {
		return ErrCallNotFound
	}

	// Update call state
	if err := sm.CallManager.UpdateCallState(callID, state); err != nil {
		return err
	}

	// Update agent state back to READY if the call is ended
	if state == CallStateEnded || state == CallStateFailed {
		agent, exists := sm.AgentManager.GetAgent(call.AgentID)
		if exists && agent.State == AgentStateBusy {
			agent.State = AgentStateReady
			agent.CallID = ""
		}
	}

	return nil
}

// CleanupAgent removes agent and associated calls
func (sm *StateManager) CleanupAgent(agentID string) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// Cleanup calls for the agent
	sm.CallManager.CleanupCallsForAgent(agentID)

	// Cleanup agent
	sm.AgentManager.CleanupAgent(agentID)
}

// GetSystemStatus returns overall system status
func (sm *StateManager) GetSystemStatus() map[string]interface{} {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	agents := sm.AgentManager.GetAllAgents()
	calls := sm.CallManager.GetAllCalls()

	agentStates := make(map[string]int)
	callStates := make(map[string]int)

	for _, agent := range agents {
		agentStates[string(agent.State)]++
	}

	for _, call := range calls {
		callStates[string(call.State)]++
	}

	return map[string]interface{}{
		"agents": map[string]interface{}{
			"total":  len(agents),
			"states": agentStates,
		},
		"calls": map[string]interface{}{
			"total":  len(calls),
			"states": callStates,
		},
	}
}