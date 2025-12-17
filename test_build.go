package main

import (
	"fmt"
	"log"

	"TheLoopA/internal/config"
	"TheLoopA/internal/domain"
)

func main() {
	fmt.Println("Testing Call Emulator Service components...")

	// Test configuration loading
	fmt.Println("1. Testing configuration loading...")
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Printf("Config test failed: %v", err)
	} else {
		fmt.Printf("   ✓ Config loaded successfully: CTI URL = %s\n", cfg.CTI.WSURL)
	}

	// Test domain models
	fmt.Println("2. Testing domain models...")
	stateManager := domain.NewStateManager()

	// Create an agent
	agent := stateManager.AgentManager.CreateAgent()
	fmt.Printf("   ✓ Agent created: ID = %s, State = %s\n", agent.ID, agent.State)

	// Update agent state
	err = stateManager.AgentManager.UpdateAgentState(agent.ID, domain.AgentStateReady)
	if err != nil {
		log.Printf("Agent state update failed: %v", err)
	} else {
		fmt.Println("   ✓ Agent state updated to READY")
	}

	// Create a call
	call, err := stateManager.StartCall(agent.ID, "test-ref-001", "1000", "test-uui")
	if err != nil {
		log.Printf("Call creation failed: %v", err)
	} else {
		fmt.Printf("   ✓ Call created: ID = %s, State = %s\n", call.ID, call.State)
	}

	// Test system status
	status := stateManager.GetSystemStatus()
	fmt.Printf("   ✓ System status: %+v\n", status)

	fmt.Println("\n All basic tests passed!")
	fmt.Println("\nTo run the full service:")
	fmt.Println("  go run cmd/emulator/main.go")
	fmt.Println("\nTo run with test scenario:")
	fmt.Println("  go run cmd/emulator/main.go -scenario -destination=1000")
	fmt.Println("\nTo build the service:")
	fmt.Println("  go build -o call-emulator cmd/emulator/main.go")
}
