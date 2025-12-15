package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

type Message struct {
	Name string      `json:"name"`
	Body interface{} `json:"body"`
}

type Client struct {
	url           string
	accessToken   string
	conn          *websocket.Conn
	dispatcher    *Dispatcher
	logger        zerolog.Logger
	reconnectChan chan struct{}
	closeChan     chan struct{}
	mutex         sync.RWMutex
	connected     bool
	ctx           context.Context
	cancel        context.CancelFunc
}

type ClientConfig struct {
	URL         string
	AccessToken string
	Logger      zerolog.Logger
}

func NewClient(config ClientConfig) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &Client{
		url:           config.URL,
		accessToken:   config.AccessToken,
		dispatcher:    NewDispatcher(config.Logger),
		logger:        config.Logger,
		reconnectChan: make(chan struct{}, 1),
		closeChan:     make(chan struct{}),
		ctx:           ctx,
		cancel:        cancel,
	}
}

func (c *Client) Connect() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.connected {
		return nil
	}

	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+c.accessToken)

	dialer := websocket.Dialer{
		HandshakeTimeout: 30 * time.Second,
	}

	conn, _, err := dialer.Dial(c.url, headers)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	c.conn = conn
	c.connected = true

	c.logger.Info().Str("url", c.url).Msg("Connected to CTI WebSocket")

	// Start message handling goroutines
	go c.readMessages()
	go c.handleReconnect()

	return nil
}

func (c *Client) Disconnect() error {
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
		c.logger.Info().Msg("Disconnected from CTI WebSocket")
		return err
	}

	return nil
}

func (c *Client) IsConnected() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.connected
}

func (c *Client) SendMessage(message Message) error {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if !c.connected || c.conn == nil {
		return fmt.Errorf("WebSocket not connected")
	}

	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	c.logger.Debug().
		Str("name", message.Name).
		RawJSON("body", data).
		Msg("Sending CTI message")

	if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		c.logger.Error().Err(err).Msg("Failed to send message")
		c.triggerReconnect()
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

func (c *Client) RegisterHandler(messageName string, handler MessageHandler) {
	c.dispatcher.RegisterHandler(messageName, handler)
}

func (c *Client) readMessages() {
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
				c.logger.Error().Err(err).Msg("Failed to read message")
				c.triggerReconnect()
				return
			}

			var message Message
			if err := json.Unmarshal(data, &message); err != nil {
				c.logger.Error().Err(err).Str("data", string(data)).Msg("Failed to unmarshal message")
				continue
			}

			c.logger.Debug().
				Str("name", message.Name).
				RawJSON("body", data).
				Msg("Received CTI message")

			// Dispatch message to handler
			c.dispatcher.Dispatch(message)
		}
	}
}

func (c *Client) handleReconnect() {
	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.closeChan:
			return
		case <-c.reconnectChan:
			c.logger.Info().Msg("Attempting to reconnect to CTI WebSocket")

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
							Msg("Reconnection failed, retrying")

						time.Sleep(backoff)
						if backoff < maxBackoff {
							backoff *= 2
						}
					} else {
						c.logger.Info().Msg("Successfully reconnected to CTI WebSocket")
						backoff = time.Second // Reset backoff
						goto reconnected
					}
				}
			}
		reconnected:
		}
	}
}

func (c *Client) triggerReconnect() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.connected {
		c.connected = false
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}

		select {
		case c.reconnectChan <- struct{}{}:
		default:
		}
	}
}

func (c *Client) UpdateAccessToken(token string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	c.accessToken = token
	c.logger.Info().Msg("Access token updated")
}