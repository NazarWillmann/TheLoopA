package scenario

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"TheLoopA/internal/api"
	"TheLoopA/internal/asterisk"
	"TheLoopA/internal/auth"
	"TheLoopA/internal/config"
	"TheLoopA/internal/cti"
	"TheLoopA/internal/domain"
	"TheLoopA/internal/ws"
	"github.com/rs/zerolog"
)

type Executor struct {
	config         *config.Config
	logger         zerolog.Logger
	keycloakClient *auth.KeycloakClient
	wsClient       *ws.Client
	ariClient      *asterisk.ARIClient
	ariEventClient *asterisk.ARIEventClient
	stateManager   *domain.StateManager
	ctiHandlers    *cti.Handlers
	httpServer     *http.Server
	apiHandlers    *api.APIHandlers
	ctx            context.Context
	cancel         context.CancelFunc
}

type ExecutorConfig struct {
	Config *config.Config
	Logger zerolog.Logger
}

func NewExecutor(cfg ExecutorConfig) *Executor {
	ctx, cancel := context.WithCancel(context.Background())

	return &Executor{
		config:       cfg.Config,
		logger:       cfg.Logger,
		stateManager: domain.NewStateManager(),
		ctx:          ctx,
		cancel:       cancel,
	}
}

func (e *Executor) Start() error {
	e.logger.Info().Msg("Starting Call Emulator Service")

	// Initialize Keycloak client
	if err := e.initKeycloakClient(); err != nil {
		return fmt.Errorf("failed to initialize Keycloak client: %w", err)
	}

	// Get access token
	accessToken, err := e.keycloakClient.GetAccessToken()
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	e.logger.Info().Msg("Successfully authenticated with Keycloak")

	// Initialize WebSocket client
	if err := e.initWebSocketClient(accessToken); err != nil {
		return fmt.Errorf("failed to initialize WebSocket client: %w", err)
	}

	// Initialize Asterisk ARI client
	if err := e.initAsteriskClients(); err != nil {
		return fmt.Errorf("failed to initialize Asterisk clients: %w", err)
	}

	// Initialize CTI handlers
	if err := e.initCTIHandlers(); err != nil {
		return fmt.Errorf("failed to initialize CTI handlers: %w", err)
	}

	// Initialize HTTP API server
	if err := e.initHTTPServer(); err != nil {
		return fmt.Errorf("failed to initialize HTTP server: %w", err)
	}

	// Connect to CTI WebSocket
	if err := e.wsClient.Connect(); err != nil {
		return fmt.Errorf("failed to connect to CTI WebSocket: %w", err)
	}

	// Connect to Asterisk ARI WebSocket
	if err := e.ariEventClient.Connect(); err != nil {
		return fmt.Errorf("failed to connect to ARI WebSocket: %w", err)
	}

	e.logger.Info().Msg("Call Emulator Service started successfully")

	// Start token refresh routine
	go e.tokenRefreshRoutine()

	// Start health check routine
	go e.healthCheckRoutine()

	return nil
}

func (e *Executor) Stop() error {
	e.logger.Info().Msg("Stopping Call Emulator Service")

	e.cancel()

	// Disconnect from WebSocket clients
	if e.wsClient != nil {
		if err := e.wsClient.Disconnect(); err != nil {
			e.logger.Error().Err(err).Msg("Failed to disconnect from CTI WebSocket")
		}
	}

	if e.ariEventClient != nil {
		if err := e.ariEventClient.Disconnect(); err != nil {
			e.logger.Error().Err(err).Msg("Failed to disconnect from ARI WebSocket")
		}
	}

	// Shutdown HTTP server
	if e.httpServer != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := e.httpServer.Shutdown(shutdownCtx); err != nil {
			e.logger.Error().Err(err).Msg("Failed to shutdown HTTP server")
		} else {
			e.logger.Info().Msg("HTTP server stopped")
		}
	}

	e.logger.Info().Msg("Call Emulator Service stopped")
	return nil
}

func (e *Executor) Wait() {
	<-e.ctx.Done()
}

// GetSystemStatus returns the system status from the state manager
func (e *Executor) GetSystemStatus() map[string]interface{} {
	return e.stateManager.GetSystemStatus()
}

func (e *Executor) initKeycloakClient() error {
	e.keycloakClient = auth.NewKeycloakClient(
		e.config.CTI.Auth.KeycloakURL,
		e.config.CTI.Auth.ClientID,
		e.config.CTI.Auth.ClientSecret,
		e.config.CTI.Auth.Username,
		e.config.CTI.Auth.Password,
	)

	e.logger.Debug().
		Str("keycloak_url", e.config.CTI.Auth.KeycloakURL).
		Str("client_id", e.config.CTI.Auth.ClientID).
		Msg("Keycloak client initialized")

	return nil
}

func (e *Executor) initWebSocketClient(accessToken string) error {
	wsConfig := ws.ClientConfig{
		URL:         e.config.CTI.WSURL,
		AccessToken: accessToken,
		Logger:      e.logger.With().Str("component", "ws_client").Logger(),
	}

	e.wsClient = ws.NewClient(wsConfig)

	e.logger.Debug().
		Str("ws_url", e.config.CTI.WSURL).
		Msg("WebSocket client initialized")

	return nil
}

func (e *Executor) initAsteriskClients() error {
	// Initialize ARI REST client
	e.ariClient = asterisk.NewARIClient(
		e.config.Asterisk.ARIURL,
		e.config.Asterisk.Username,
		e.config.Asterisk.Password,
		e.logger.With().Str("component", "ari_client").Logger(),
	)

	e.logger.Debug().
		Str("ari_url", e.config.Asterisk.ARIURL).
		Str("username", e.config.Asterisk.Username).
		Msg("ARI REST client initialized")

	return nil
}

func (e *Executor) initCTIHandlers() error {
	// Create event handler for ARI events
	eventHandler := asterisk.NewDefaultEventHandler(
		e.stateManager,
		nil, // Will be set after CTI handlers are created
		e.logger.With().Str("component", "ari_event_handler").Logger(),
	)

	// Initialize ARI event client
	e.ariEventClient = asterisk.NewARIEventClient(
		e.config.Asterisk.ARIURL,
		e.config.Asterisk.Username,
		e.config.Asterisk.Password,
		"call-emulator",
		eventHandler,
		e.logger.With().Str("component", "ari_event_client").Logger(),
	)

	// Initialize CTI handlers
	handlerContext := &cti.HandlerContext{
		StateManager:   e.stateManager,
		WSClient:       e.wsClient,
		AsteriskClient: e.ariClient,
		Logger:         e.logger.With().Str("component", "cti_handlers").Logger(),
	}

	e.ctiHandlers = cti.NewHandlers(handlerContext)

	// Set CTI handlers as event sender for ARI event handler
	eventHandler.SetCTIEventSender(e.ctiHandlers)

	// Register CTI message handlers
	e.ctiHandlers.RegisterHandlers()

	e.logger.Debug().Msg("CTI handlers initialized and registered")

	return nil
}

func (e *Executor) initHTTPServer() error {
	if !e.config.HTTP.Enabled {
		e.logger.Info().Msg("HTTP API server is disabled")
		return nil
	}

	// Initialize API handlers
	e.apiHandlers = api.NewAPIHandlers(e, e.logger.With().Str("component", "api_handlers").Logger())

	// Setup routes
	mux := e.apiHandlers.SetupRoutes()

	// Create HTTP server
	addr := ":" + strconv.Itoa(e.config.HTTP.Port)
	e.httpServer = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Start HTTP server in a goroutine
	go func() {
		e.logger.Info().
			Int("port", e.config.HTTP.Port).
			Msg("Starting HTTP API server")

		if err := e.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			e.logger.Error().Err(err).Msg("HTTP server failed")
		}
	}()

	e.logger.Debug().
		Int("port", e.config.HTTP.Port).
		Msg("HTTP API server initialized")

	return nil
}

func (e *Executor) tokenRefreshRoutine() {
	ticker := time.NewTicker(30 * time.Minute) // Refresh every 30 minutes
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			e.logger.Debug().Msg("Refreshing access token")

			accessToken, err := e.keycloakClient.GetAccessToken()
			if err != nil {
				e.logger.Error().Err(err).Msg("Failed to refresh access token")
				continue
			}

			e.wsClient.UpdateAccessToken(accessToken)
			e.logger.Debug().Msg("Access token refreshed successfully")
		}
	}
}

func (e *Executor) healthCheckRoutine() {
	ticker := time.NewTicker(1 * time.Minute) // Check every minute
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			e.performHealthCheck()
		}
	}
}

func (e *Executor) performHealthCheck() {
	// Check WebSocket connections
	wsConnected := e.wsClient.IsConnected()
	ariConnected := e.ariEventClient.IsConnected()

	// Get system status
	systemStatus := e.stateManager.GetSystemStatus()

	e.logger.Debug().
		Bool("cti_ws_connected", wsConnected).
		Bool("ari_ws_connected", ariConnected).
		Interface("system_status", systemStatus).
		Msg("Health check completed")

	// Log warnings for disconnected services
	if !wsConnected {
		e.logger.Warn().Msg("CTI WebSocket is disconnected")
	}

	if !ariConnected {
		e.logger.Warn().Msg("ARI WebSocket is disconnected")
	}
}

// RunBasicScenario executes a basic test scenario
func (e *Executor) RunBasicScenario(destinationNumber string) error {
	e.logger.Info().
		Str("destination", destinationNumber).
		Msg("Running basic call scenario")

	// Wait for connections to be established
	time.Sleep(2 * time.Second)

	// Step 1: Agent Register
	registerMsg := ws.Message{
		Name: "agentRegister",
		Body: cti.AgentRegisterMessage{
			BaseMessage: cti.BaseMessage{RefID: "test-register-001"},
		},
	}

	if err := e.wsClient.SendMessage(registerMsg); err != nil {
		return fmt.Errorf("failed to send agent register: %w", err)
	}

	e.logger.Info().Msg("Sent agentRegister message")
	time.Sleep(1 * time.Second)

	// Step 2: Agent Login
	loginMsg := ws.Message{
		Name: "agentLogin",
		Body: cti.AgentLoginMessage{
			BaseMessage: cti.BaseMessage{RefID: "test-login-001"},
		},
	}

	if err := e.wsClient.SendMessage(loginMsg); err != nil {
		return fmt.Errorf("failed to send agent login: %w", err)
	}

	e.logger.Info().Msg("Sent agentLogin message")
	time.Sleep(1 * time.Second)

	// Step 3: Agent Change State to READY
	changeStateMsg := ws.Message{
		Name: "agentChangeState",
		Body: cti.AgentChangeStateMessage{
			BaseMessage: cti.BaseMessage{RefID: "test-ready-001"},
			State:       cti.AgentStateCodeReady,
		},
	}

	if err := e.wsClient.SendMessage(changeStateMsg); err != nil {
		return fmt.Errorf("failed to send agent change state: %w", err)
	}

	e.logger.Info().Msg("Sent agentChangeState (READY) message")
	time.Sleep(1 * time.Second)

	// Step 4: Make Call
	makeCallMsg := ws.Message{
		Name: "lineMakeCall",
		Body: cti.LineMakeCallMessage{
			BaseMessage:       cti.BaseMessage{RefID: "test-call-001"},
			DestinationNumber: destinationNumber,
			UUI:               "test-uui-data",
		},
	}

	if err := e.wsClient.SendMessage(makeCallMsg); err != nil {
		return fmt.Errorf("failed to send make call: %w", err)
	}

	e.logger.Info().
		Str("destination", destinationNumber).
		Msg("Sent lineMakeCall message")

	e.logger.Info().Msg("Basic scenario completed successfully")
	return nil
}
