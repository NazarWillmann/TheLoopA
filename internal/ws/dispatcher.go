package ws

import (
	"sync"

	"github.com/rs/zerolog"
)

type MessageHandler interface {
	Handle(message Message) error
}

type MessageHandlerFunc func(message Message) error

func (f MessageHandlerFunc) Handle(message Message) error {
	return f(message)
}

type Dispatcher struct {
	handlers map[string]MessageHandler
	mutex    sync.RWMutex
	logger   zerolog.Logger
}

func NewDispatcher(logger zerolog.Logger) *Dispatcher {
	return &Dispatcher{
		handlers: make(map[string]MessageHandler),
		logger:   logger,
	}
}

func (d *Dispatcher) RegisterHandler(messageName string, handler MessageHandler) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.handlers[messageName] = handler
	d.logger.Debug().Str("message_name", messageName).Msg("Registered message handler")
}

func (d *Dispatcher) UnregisterHandler(messageName string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	delete(d.handlers, messageName)
	d.logger.Debug().Str("message_name", messageName).Msg("Unregistered message handler")
}

func (d *Dispatcher) Dispatch(message Message) {
	d.mutex.RLock()
	handler, exists := d.handlers[message.Name]
	d.mutex.RUnlock()

	if !exists {
		d.logger.Warn().
			Str("message_name", message.Name).
			Msg("No handler registered for message")
		return
	}

	go func() {
		if err := handler.Handle(message); err != nil {
			d.logger.Error().
				Err(err).
				Str("message_name", message.Name).
				Msg("Handler failed to process message")
		}
	}()
}

func (d *Dispatcher) GetRegisteredHandlers() []string {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	handlers := make([]string, 0, len(d.handlers))
	for name := range d.handlers {
		handlers = append(handlers, name)
	}
	return handlers
}