package domain

import "errors"

var (
	ErrAgentNotFound = errors.New("agent not found")
	ErrCallNotFound  = errors.New("call not found")
	ErrInvalidState  = errors.New("invalid state transition")
	ErrAgentNotReady = errors.New("agent is not ready")
	ErrCallInProgress = errors.New("call already in progress")
)