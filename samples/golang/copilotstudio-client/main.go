// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	settings := ConnectionSettings{
		EnvironmentID: os.Getenv("COPILOT_ENVIRONMENT_ID"),
		AgentID:       os.Getenv("COPILOT_AGENT_ID"),
		TenantID:      os.Getenv("COPILOT_TENANT_ID"),
		Token:         os.Getenv("DIRECTLINE_TOKEN"),
	}

	client := NewCopilotClient(settings)

	log.Println("Starting conversation with Copilot Studio agent...")
	if err := client.StartConversation(); err != nil {
		log.Fatalf("Failed to start conversation: %v", err)
	}
	log.Println("Conversation started. Type 'quit' or 'exit' to stop.")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\n>>> ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		lower := strings.ToLower(input)
		if lower == "quit" || lower == "exit" {
			fmt.Println("Exiting...")
			break
		}
		reply, err := client.AskQuestion(input)
		if err != nil {
			log.Printf("Error sending question: %v", err)
			continue
		}
		if reply != "" {
			fmt.Println(reply)
		} else {
			fmt.Println("(no response)")
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("Scanner error: %v", err)
	}
}
