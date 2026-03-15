// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
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
	invokeHandler              func(ctx *TurnContext)
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

// OnInvoke registers a handler for invoke activities (e.g., skillBegin, skillEnd)
func (a *AgentApp) OnInvoke(handler func(ctx *TurnContext)) {
	a.invokeHandler = handler
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
	case "endOfConversation":
		if h, ok := a.conversationUpdateHandlers["endOfConversation"]; ok {
			h(ctx)
		}
	case "invoke":
		if a.invokeHandler != nil {
			a.invokeHandler(ctx)
		}
	}
}

// RegisterHandlers sets up the message routing for the Copilot Studio skill agent
func RegisterHandlers(app *AgentApp) {
	app.OnConversationUpdate("membersAdded", func(ctx *TurnContext) {
		if err := ctx.SendActivity("Copilot Studio skill is ready!"); err != nil {
			log.Printf("Error sending welcome: %v", err)
		}
	})

	app.OnConversationUpdate("endOfConversation", func(ctx *TurnContext) {
		log.Println("Conversation ended by Copilot Studio")
	})

	app.OnMessage(nil, func(ctx *TurnContext) {
		reply := fmt.Sprintf("Skill received: %s", ctx.Activity.Text)
		if err := ctx.SendActivity(reply); err != nil {
			log.Printf("Error sending skill reply: %v", err)
		}
	})

	app.OnInvoke(func(ctx *TurnContext) {
		activityName := ctx.Activity.Text
		switch activityName {
		case "skillBegin":
			log.Println("Skill invoked: skillBegin")
			if err := ctx.SendActivity("Skill started."); err != nil {
				log.Printf("Error sending skillBegin response: %v", err)
			}
		case "skillEnd":
			log.Println("Skill invoked: skillEnd")
			if err := ctx.SendActivity("Skill ended."); err != nil {
				log.Printf("Error sending skillEnd response: %v", err)
			}
		default:
			log.Printf("Skill invoked with unknown action: %s", activityName)
		}
	})

	app.OnError(func(ctx *TurnContext, err error) {
		log.Printf("[on_turn_error] unhandled error: %v", err)
		ctx.SendActivity("The skill encountered an error or bug.")
	})
}
