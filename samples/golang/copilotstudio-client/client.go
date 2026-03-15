// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// ConnectionSettings holds Copilot Studio connection config
type ConnectionSettings struct {
	EnvironmentID string
	AgentID       string
	TenantID      string
	Token         string
}

// CopilotClient is a REST client for Copilot Studio Direct Line API
type CopilotClient struct {
	settings       ConnectionSettings
	httpClient     *http.Client
	conversationID string
	watermark      string
}

// NewCopilotClient creates a new CopilotClient with the given settings
func NewCopilotClient(settings ConnectionSettings) *CopilotClient {
	return &CopilotClient{
		settings:   settings,
		httpClient: &http.Client{},
	}
}

// StartConversation initializes a new conversation with Copilot Studio
func (c *CopilotClient) StartConversation() error {
	// POST to Direct Line token endpoint to get conversation ID
	// This is a simplified implementation - real impl needs proper auth token
	url := "https://directline.botframework.com/v3/directline/conversations"
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.settings.Token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("start conversation failed: %w", err)
	}
	defer resp.Body.Close()
	var result struct {
		ConversationID string `json:"conversationId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}
	c.conversationID = result.ConversationID
	return nil
}

// AskQuestion sends a question and returns the bot's reply
func (c *CopilotClient) AskQuestion(question string) (string, error) {
	// Send message
	sendURL := fmt.Sprintf("https://directline.botframework.com/v3/directline/conversations/%s/activities", c.conversationID)
	payload := map[string]interface{}{
		"type": "message",
		"text": question,
		"from": map[string]string{"id": "user"},
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", sendURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.settings.Token)
	req.Header.Set("Content-Type", "application/json")
	if _, err := c.httpClient.Do(req); err != nil {
		return "", fmt.Errorf("send message failed: %w", err)
	}
	// Poll for reply
	pollURL := fmt.Sprintf("https://directline.botframework.com/v3/directline/conversations/%s/activities?watermark=%s", c.conversationID, c.watermark)
	req2, _ := http.NewRequest("GET", pollURL, nil)
	req2.Header.Set("Authorization", "Bearer "+c.settings.Token)
	resp2, err := c.httpClient.Do(req2)
	if err != nil {
		return "", err
	}
	defer resp2.Body.Close()
	var result struct {
		Activities []struct {
			Type string `json:"type"`
			Text string `json:"text"`
			From struct {
				ID string `json:"id"`
			} `json:"from"`
		} `json:"activities"`
		Watermark string `json:"watermark"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&result); err != nil {
		return "", err
	}
	c.watermark = result.Watermark
	for _, act := range result.Activities {
		if act.Type == "message" && act.From.ID != "user" {
			return act.Text, nil
		}
	}
	return "", nil
}
