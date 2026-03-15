// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
)

// ChannelAccount identifies a participant
type ChannelAccount struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ConversationAccount identifies the conversation
type ConversationAccount struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Activity is a Bot Framework activity (message, event, etc.)
type Activity struct {
	Type         string              `json:"type"`
	ID           string              `json:"id"`
	Text         string              `json:"text"`
	From         ChannelAccount      `json:"from"`
	Recipient    ChannelAccount      `json:"recipient"`
	Conversation ConversationAccount `json:"conversation"`
	ReplyToID    string              `json:"replyToId,omitempty"`
	ServiceURL   string              `json:"serviceUrl"`
	ChannelID    string              `json:"channelId"`
	MembersAdded []ChannelAccount    `json:"membersAdded,omitempty"`
	Attachments  []Attachment        `json:"attachments,omitempty"`
	Value        interface{}         `json:"value,omitempty"`
}

// Attachment represents a rich card or file attachment
type Attachment struct {
	ContentType string      `json:"contentType"`
	Content     interface{} `json:"content"`
}

// TurnContext holds the current turn's activity and response writer
type TurnContext struct {
	Activity   *Activity
	writer     http.ResponseWriter
	serviceURL string
	botID      string
	botName    string
}

// SendActivity sends a text reply to the user
func (tc *TurnContext) SendActivity(text string) error {
	reply := Activity{
		Type:         "message",
		Text:         text,
		From:         ChannelAccount{ID: tc.botID, Name: tc.botName},
		Recipient:    tc.Activity.From,
		Conversation: tc.Activity.Conversation,
		ReplyToID:    tc.Activity.ID,
	}
	tc.writer.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(tc.writer).Encode(reply)
}

// SendActivityWithAttachment sends a reply with an attachment
func (tc *TurnContext) SendActivityWithAttachment(attachment Attachment) error {
	reply := Activity{
		Type:         "message",
		From:         ChannelAccount{ID: tc.botID, Name: tc.botName},
		Recipient:    tc.Activity.From,
		Conversation: tc.Activity.Conversation,
		ReplyToID:    tc.Activity.ID,
		Attachments:  []Attachment{attachment},
	}
	tc.writer.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(tc.writer).Encode(reply)
}

// MessageHandler is a function that handles an incoming message activity
type MessageHandler func(ctx *TurnContext)

// ConversationUpdateHandler is a function that handles a conversation update activity
type ConversationUpdateHandler func(ctx *TurnContext)

// ErrorHandler is a function that handles errors occurring during a turn
type ErrorHandler func(ctx *TurnContext, err error)

// messageEntry pairs a pattern (or nil for catch-all) with its handler
type messageEntry struct {
	pattern *regexp.Regexp // nil = catch-all
	handler MessageHandler
}

// AgentApp is the central routing hub for a bot agent
type AgentApp struct {
	messageHandlers            []messageEntry
	conversationUpdateHandlers map[string]ConversationUpdateHandler
	errorHandler               ErrorHandler
	botID                      string
	botName                    string
}

// NewAgentApp creates a new AgentApp, reading BOT_ID and BOT_NAME from the environment
func NewAgentApp() *AgentApp {
	return &AgentApp{
		conversationUpdateHandlers: make(map[string]ConversationUpdateHandler),
		botID:                      os.Getenv("BOT_ID"),
		botName:                    os.Getenv("BOT_NAME"),
	}
}

// OnMessage registers a handler for incoming message activities.
// If pattern is nil, the handler acts as a catch-all.
func (a *AgentApp) OnMessage(pattern *regexp.Regexp, handler MessageHandler) {
	a.messageHandlers = append(a.messageHandlers, messageEntry{pattern, handler})
}

// OnConversationUpdate registers a handler for a specific conversation update event
func (a *AgentApp) OnConversationUpdate(event string, handler ConversationUpdateHandler) {
	a.conversationUpdateHandlers[event] = handler
}

// OnError registers a handler for errors that occur during a turn
func (a *AgentApp) OnError(handler ErrorHandler) {
	a.errorHandler = handler
}

// Router returns an http.Handler with the /api/messages route registered
func (a *AgentApp) Router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/messages", a.handleMessages)
	return mux
}

func (a *AgentApp) handleMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var activity Activity
	if err := json.NewDecoder(r.Body).Decode(&activity); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	ctx := &TurnContext{
		Activity:   &activity,
		writer:     w,
		serviceURL: activity.ServiceURL,
		botID:      a.botID,
		botName:    a.botName,
	}
	defer func() {
		if rec := recover(); rec != nil {
			if a.errorHandler != nil {
				a.errorHandler(ctx, fmt.Errorf("%v", rec))
			}
		}
	}()
	switch activity.Type {
	case "conversationUpdate":
		if len(activity.MembersAdded) > 0 {
			if h, ok := a.conversationUpdateHandlers["membersAdded"]; ok {
				h(ctx)
			}
		}
	case "message":
		for _, entry := range a.messageHandlers {
			if entry.pattern == nil || entry.pattern.MatchString(activity.Text) {
				entry.handler(ctx)
				return
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Azure OpenAI streaming support
// ---------------------------------------------------------------------------

// azureOpenAIRequest is the request body for the Azure OpenAI chat completions API.
type azureOpenAIRequest struct {
	Model    string                   `json:"model"`
	Messages []map[string]string      `json:"messages"`
	Stream   bool                     `json:"stream"`
}

// azureOpenAIStreamChunk is a single chunk from the SSE stream.
// Only the fields we need are unmarshalled.
type azureOpenAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

// streamAzureOpenAI calls the Azure OpenAI chat completions API with streaming
// enabled and collects all content chunks, returning the assembled full response.
//
// NOTE on streaming design: The Bot Framework protocol does not support
// server-sent events (SSE) or chunked streaming to the client over the standard
// /api/messages HTTP response. True token-level streaming requires the Azure Bot
// Service Direct Line streaming protocol (WebSockets). As a pragmatic alternative
// this implementation collects all chunks from the Azure OpenAI SSE stream and
// sends the assembled response as a single Bot Framework activity. The SSE
// parsing is done manually on the response body without any third-party SDK.
func streamAzureOpenAI(messages []map[string]string) (string, error) {
	endpoint := os.Getenv("AZURE_OPENAI_ENDPOINT")
	deployment := os.Getenv("AZURE_OPENAI_DEPLOYMENT")
	apiKey := os.Getenv("AZURE_OPENAI_API_KEY")

	url := fmt.Sprintf(
		"%s/openai/deployments/%s/chat/completions?api-version=2024-02-01",
		strings.TrimRight(endpoint, "/"),
		deployment,
	)

	reqBody := azureOpenAIRequest{
		Model:    deployment,
		Messages: messages,
		Stream:   true,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Azure OpenAI returned %d: %s", resp.StatusCode, string(body))
	}

	// Parse the SSE stream line by line.
	// Each event looks like:
	//   data: {"choices":[{"delta":{"content":"Hello"},...}],...}
	// The stream ends with:
	//   data: [DONE]
	var sb strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			break
		}
		var chunk azureOpenAIStreamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			log.Printf("Warning: failed to parse SSE chunk: %v", err)
			continue
		}
		if len(chunk.Choices) > 0 {
			sb.WriteString(chunk.Choices[0].Delta.Content)
		}
	}
	if err := scanner.Err(); err != nil {
		return sb.String(), fmt.Errorf("reading SSE stream: %w", err)
	}

	return sb.String(), nil
}

// ---------------------------------------------------------------------------
// Handler registration
// ---------------------------------------------------------------------------

// RegisterHandlers sets up the message routing for the azureai-streaming agent.
func RegisterHandlers(app *AgentApp) {
	app.OnConversationUpdate("membersAdded", func(ctx *TurnContext) {
		if err := ctx.SendActivity("Welcome to the streaming sample. Type **poem** to see Azure OpenAI generate a poem!"); err != nil {
			log.Printf("Error sending welcome: %v", err)
		}
	})

	// Handle "poem" messages: call Azure OpenAI with streaming, collect chunks,
	// then send the assembled reply as a single Bot Framework activity.
	app.OnMessage(regexp.MustCompile(`(?i)poem`), func(ctx *TurnContext) {
		// Send an informative "Thinking..." update while the model generates.
		if err := ctx.SendActivity("Thinking..."); err != nil {
			log.Printf("Error sending thinking update: %v", err)
		}

		messages := []map[string]string{
			{
				"role": "system",
				"content": "You are a creative assistant who has deeply studied Greek and Roman Gods. " +
					"You also know all of the Percy Jackson Series. " +
					"You write poems about the Greek Gods as they are depicted in the Percy Jackson books. " +
					"You format the poems in a way that is easy to read and understand. " +
					"You break your poems into stanzas. " +
					"You format your poems in Markdown using double lines to separate stanzas.",
			},
			{
				"role":    "user",
				"content": "Write a poem in no less than 500 words about the Greek God Apollo as depicted in the Percy Jackson books",
			},
		}

		fullResponse, err := streamAzureOpenAI(messages)
		if err != nil {
			log.Printf("Error calling Azure OpenAI: %v", err)
			if sendErr := ctx.SendActivity("An error occurred while generating the poem. Please try again later."); sendErr != nil {
				log.Printf("Error sending error message: %v", sendErr)
			}
			return
		}

		if err := ctx.SendActivity(fullResponse); err != nil {
			log.Printf("Error sending poem response: %v", err)
		}
	})

	// Catch-all handler for any other messages.
	app.OnMessage(nil, func(ctx *TurnContext) {
		if err := ctx.SendActivity("Type **poem** to see Azure OpenAI generate a poem about Apollo!"); err != nil {
			log.Printf("Error sending fallback: %v", err)
		}
	})

	app.OnError(func(ctx *TurnContext, err error) {
		log.Printf("[on_turn_error] unhandled error: %v", err)
		if sendErr := ctx.SendActivity("The bot encountered an error or bug."); sendErr != nil {
			log.Printf("Error sending error activity: %v", sendErr)
		}
	})
}
