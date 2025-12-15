package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"TheLoopA/internal/config"
	"TheLoopA/internal/scenario"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	configPath      = flag.String("config", "config.yaml", "Path to configuration file")
	logLevel        = flag.String("log-level", "", "Log level (debug, info, warn, error)")
	runScenario     = flag.Bool("scenario", false, "Run basic test scenario")
	destinationNum  = flag.String("destination", "1000", "Destination number for test scenario")
	version         = "1.0.0"
	buildTime       = "unknown"
	gitCommit       = "unknown"
)

func main() {
	flag.Parse()

	// Print version information
	fmt.Printf("Call Emulator Service v%s\n", version)
	fmt.Printf("Build Time: %s\n", buildTime)
	fmt.Printf("Git Commit: %s\n", gitCommit)
	fmt.Println()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal().Err(err).Str("config_path", *configPath).Msg("Failed to load configuration")
	}

	// Setup logging
	logger := setupLogging(cfg, *logLevel)
	logger.Info().
		Str("version", version).
		Str("config_path", *configPath).
		Msg("Starting Call Emulator Service")

	// Create and start executor
	executor := scenario.NewExecutor(scenario.ExecutorConfig{
		Config: cfg,
		Logger: logger,
	})

	// Start the service
	if err := executor.Start(); err != nil {
		logger.Fatal().Err(err).Msg("Failed to start service")
	}

	// Run test scenario if requested
	if *runScenario {
		logger.Info().
			Str("destination", *destinationNum).
			Msg("Running test scenario")

		// Wait a bit for services to initialize
		time.Sleep(5 * time.Second)

		if err := executor.RunBasicScenario(*destinationNum); err != nil {
			logger.Error().Err(err).Msg("Test scenario failed")
		} else {
			logger.Info().Msg("Test scenario completed successfully")
		}
	}

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	logger.Info().Msg("Service is running. Press Ctrl+C to stop.")

	// Wait for shutdown signal
	sig := <-sigChan
	logger.Info().Str("signal", sig.String()).Msg("Received shutdown signal")

	// Graceful shutdown
	logger.Info().Msg("Shutting down service...")
	if err := executor.Stop(); err != nil {
		logger.Error().Err(err).Msg("Error during shutdown")
	}

	logger.Info().Msg("Service stopped successfully")
}

func setupLogging(cfg *config.Config, logLevelFlag string) zerolog.Logger {
	// Determine log level
	var logLevel zerolog.Level
	levelStr := cfg.Service.LogLevel
	if logLevelFlag != "" {
		levelStr = logLevelFlag
	}

	switch levelStr {
	case "debug":
		logLevel = zerolog.DebugLevel
	case "info":
		logLevel = zerolog.InfoLevel
	case "warn":
		logLevel = zerolog.WarnLevel
	case "error":
		logLevel = zerolog.ErrorLevel
	default:
		logLevel = zerolog.InfoLevel
	}

	// Configure zerolog
	zerolog.SetGlobalLevel(logLevel)
	zerolog.TimeFieldFormat = time.RFC3339

	// Create console writer with colors
	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "15:04:05",
		FormatLevel: func(i interface{}) string {
			return fmt.Sprintf("| %-6s|", i)
		},
		FormatMessage: func(i interface{}) string {
			return fmt.Sprintf("%s", i)
		},
		FormatFieldName: func(i interface{}) string {
			return fmt.Sprintf("%s:", i)
		},
		FormatFieldValue: func(i interface{}) string {
			return fmt.Sprintf("%s", i)
		},
	}

	logger := zerolog.New(consoleWriter).With().
		Timestamp().
		Str("service", "call-emulator").
		Logger()

	logger.Info().
		Str("level", logLevel.String()).
		Msg("Logging initialized")

	return logger
}
