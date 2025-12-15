package domain

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type AgentState string

const (
	AgentStateRegistered AgentState = "REGISTERED"
	AgentStateLoggedIn   AgentState = "LOGGED_IN"
	AgentStateReady      AgentState = "READY"
	AgentStateBusy       AgentState = "BUSY"
	AgentStateLoggedOut  AgentState = "LOGGED_OUT"
)

type Agent struct {
	ID          string     `json:"id"`
	State       AgentState `json:"state"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CallID      string     `json:"call_id,omitempty"`
	LastRefID   string     `json:"last_ref_id,omitempty"`
}

type AgentManager struct {
	agents map[string]*Agent
	mutex  sync.RWMutex
}

func NewAgentManager() *AgentManager {
	return &AgentManager{
		agents: make(map[string]*Agent),
	}
}

func (am *AgentManager) CreateAgent() *Agent {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	agent := &Agent{
		ID:        uuid.New().String(),
		State:     AgentStateRegistered,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	am.agents[agent.ID] = agent
	return agent
}

func (am *AgentManager) GetAgent(id string) (*Agent, bool) {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	agent, exists := am.agents[id]
	return agent, exists
}

func (am *AgentManager) UpdateAgentState(id string, state AgentState) error {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	agent, exists := am.agents[id]
	if !exists {
		return ErrAgentNotFound
	}

	agent.State = state
	agent.UpdatedAt = time.Now()
	return nil
}

func (am *AgentManager) SetAgentCall(id string, callID string) error {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	agent, exists := am.agents[id]
	if !exists {
		return ErrAgentNotFound
	}

	agent.CallID = callID
	agent.UpdatedAt = time.Now()
	return nil
}

func (am *AgentManager) SetAgentRefID(id string, refID string) error {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	agent, exists := am.agents[id]
	if !exists {
		return ErrAgentNotFound
	}

	agent.LastRefID = refID
	agent.UpdatedAt = time.Now()
	return nil
}

func (am *AgentManager) RemoveAgent(id string) {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	delete(am.agents, id)
}

func (am *AgentManager) GetAllAgents() map[string]*Agent {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	result := make(map[string]*Agent)
	for k, v := range am.agents {
		result[k] = v
	}
	return result
}

func (am *AgentManager) CleanupAgent(id string) {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	if agent, exists := am.agents[id]; exists {
		agent.State = AgentStateLoggedOut
		agent.CallID = ""
		agent.UpdatedAt = time.Now()
	}
}