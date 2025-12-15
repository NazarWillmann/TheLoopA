package cti

// Base message structure for CTI commands
type BaseMessage struct {
	RefID string `json:"refId,omitempty"`
}

// Agent registration message
type AgentRegisterMessage struct {
	BaseMessage
}

// Agent login message
type AgentLoginMessage struct {
	BaseMessage
	AgentID  string `json:"agentId,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// Agent change state message
type AgentChangeStateMessage struct {
	BaseMessage
	AgentID string `json:"agentId,omitempty"`
	State   int    `json:"state"`
}

// Agent logout message
type AgentLogoutMessage struct {
	BaseMessage
	AgentID string `json:"agentId,omitempty"`
}

// Line make call message
type LineMakeCallMessage struct {
	BaseMessage
	DestinationNumber string `json:"destinationNumber"`
	UUI               string `json:"uui,omitempty"`
	AgentID           string `json:"agentId,omitempty"`
}

// Line answer call message
type LineAnswerCallMessage struct {
	BaseMessage
	CallID  string `json:"callId,omitempty"`
	AgentID string `json:"agentId,omitempty"`
}

// Line setup transfer message
type LineSetupTransferMessage struct {
	BaseMessage
	CallID            string `json:"callId,omitempty"`
	AgentID           string `json:"agentId,omitempty"`
	TransferTarget    string `json:"transferTarget,omitempty"`
	TransferType      string `json:"transferType,omitempty"`
}

// Line complete SPY calls message
type LineCompleteSPYCallsMessage struct {
	BaseMessage
	AgentID string `json:"agentId,omitempty"`
}

// Response messages for outbound events
type AgentStateChangedEvent struct {
	AgentID   string `json:"agentId"`
	State     string `json:"state"`
	Timestamp string `json:"timestamp"`
}

type CallStartedEvent struct {
	CallID            string `json:"callId"`
	AgentID           string `json:"agentId"`
	RefID             string `json:"refId"`
	DestinationNumber string `json:"destinationNumber"`
	Timestamp         string `json:"timestamp"`
}

type CallConnectedEvent struct {
	CallID            string `json:"callId"`
	AgentID           string `json:"agentId"`
	RefID             string `json:"refId"`
	DestinationNumber string `json:"destinationNumber"`
	Timestamp         string `json:"timestamp"`
}

type CallEndedEvent struct {
	CallID    string `json:"callId"`
	AgentID   string `json:"agentId"`
	RefID     string `json:"refId"`
	Reason    string `json:"reason"`
	Duration  int    `json:"duration,omitempty"`
	Timestamp string `json:"timestamp"`
}

type CallFailedEvent struct {
	CallID    string `json:"callId"`
	AgentID   string `json:"agentId"`
	RefID     string `json:"refId"`
	Reason    string `json:"reason"`
	Timestamp string `json:"timestamp"`
}

// Success/Error response messages
type SuccessResponse struct {
	RefID   string `json:"refId,omitempty"`
	Message string `json:"message,omitempty"`
}

type ErrorResponse struct {
	RefID   string `json:"refId,omitempty"`
	Error   string `json:"error"`
	Code    int    `json:"code,omitempty"`
}

// Agent state constants mapping
const (
	AgentStateCodeReady = 5000
	AgentStateCodeBusy  = 5001
)

// Call state reasons
const (
	CallEndReasonNormal     = "NORMAL"
	CallEndReasonBusy       = "BUSY"
	CallEndReasonNoAnswer   = "NO_ANSWER"
	CallEndReasonFailed     = "FAILED"
	CallEndReasonCancelled  = "CANCELLED"
)
