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
// Card-specific types
// ---------------------------------------------------------------------------

// HeroCard is a Bot Framework Hero Card
type HeroCard struct {
	Title    string       `json:"title"`
	Subtitle string       `json:"subtitle,omitempty"`
	Text     string       `json:"text,omitempty"`
	Images   []CardImage  `json:"images,omitempty"`
	Buttons  []CardAction `json:"buttons,omitempty"`
}

// CardImage is an image used in a card
type CardImage struct {
	URL string `json:"url"`
	Alt string `json:"alt,omitempty"`
}

// CardAction is a button/action in a card
type CardAction struct {
	Type  string `json:"type"`
	Title string `json:"title"`
	Value string `json:"value"`
}

// ThumbnailCard is a Bot Framework Thumbnail Card
type ThumbnailCard struct {
	Title    string       `json:"title"`
	Subtitle string       `json:"subtitle,omitempty"`
	Text     string       `json:"text,omitempty"`
	Images   []CardImage  `json:"images,omitempty"`
	Buttons  []CardAction `json:"buttons,omitempty"`
}

// ---------------------------------------------------------------------------
// CardFactory helper functions
// ---------------------------------------------------------------------------

// HeroCardAttachment wraps a HeroCard in an Attachment with the correct content type
func HeroCardAttachment(card HeroCard) Attachment {
	return Attachment{
		ContentType: "application/vnd.microsoft.card.hero",
		Content:     card,
	}
}

// ThumbnailCardAttachment wraps a ThumbnailCard in an Attachment with the correct content type
func ThumbnailCardAttachment(card ThumbnailCard) Attachment {
	return Attachment{
		ContentType: "application/vnd.microsoft.card.thumbnail",
		Content:     card,
	}
}

// AdaptiveCardAttachment wraps an adaptive card body (map) in an Attachment
func AdaptiveCardAttachment(content map[string]interface{}) Attachment {
	return Attachment{
		ContentType: "application/vnd.microsoft.card.adaptive",
		Content:     content,
	}
}

// ---------------------------------------------------------------------------
// Adaptive Card content (embedded so no external file is needed at runtime)
// ---------------------------------------------------------------------------

// flightAdaptiveCard is the sample adaptive card from the Python resource file
var flightAdaptiveCard = map[string]interface{}{
	"$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
	"version": "1.0",
	"type":    "AdaptiveCard",
	"speak":   "Your flight is confirmed for you and 3 other passengers from San Francisco to Amsterdam on Friday, October 10 8:30 AM",
	"body": []interface{}{
		map[string]interface{}{"type": "TextBlock", "text": "Passengers", "weight": "bolder", "isSubtle": false},
		map[string]interface{}{"type": "TextBlock", "text": "Sarah Hum", "separator": true},
		map[string]interface{}{"type": "TextBlock", "text": "Jeremy Goldberg", "spacing": "none"},
		map[string]interface{}{"type": "TextBlock", "text": "Evan Litvak", "spacing": "none"},
		map[string]interface{}{"type": "TextBlock", "text": "2 Stops", "weight": "bolder", "spacing": "medium"},
		map[string]interface{}{"type": "TextBlock", "text": "Fri, October 10 8:30 AM", "weight": "bolder", "spacing": "none"},
		map[string]interface{}{
			"type":      "ColumnSet",
			"separator": true,
			"columns": []interface{}{
				map[string]interface{}{
					"type":  "Column",
					"width": 1,
					"items": []interface{}{
						map[string]interface{}{"type": "TextBlock", "text": "San Francisco", "isSubtle": true},
						map[string]interface{}{"type": "TextBlock", "size": "extraLarge", "color": "accent", "text": "SFO", "spacing": "none"},
					},
				},
				map[string]interface{}{
					"type":  "Column",
					"width": "auto",
					"items": []interface{}{
						map[string]interface{}{"type": "TextBlock", "text": " "},
						map[string]interface{}{"type": "Image", "url": "http://adaptivecards.io/content/airplane.png", "size": "small", "spacing": "none"},
					},
				},
				map[string]interface{}{
					"type":  "Column",
					"width": 1,
					"items": []interface{}{
						map[string]interface{}{"type": "TextBlock", "horizontalAlignment": "right", "text": "Amsterdam", "isSubtle": true},
						map[string]interface{}{"type": "TextBlock", "horizontalAlignment": "right", "size": "extraLarge", "color": "accent", "text": "AMS", "spacing": "none"},
					},
				},
			},
		},
		map[string]interface{}{"type": "TextBlock", "text": "Non-Stop", "weight": "bolder", "spacing": "medium"},
		map[string]interface{}{"type": "TextBlock", "text": "Fri, October 18 9:50 PM", "weight": "bolder", "spacing": "none"},
		map[string]interface{}{
			"type":      "ColumnSet",
			"separator": true,
			"columns": []interface{}{
				map[string]interface{}{
					"type":  "Column",
					"width": 1,
					"items": []interface{}{
						map[string]interface{}{"type": "TextBlock", "text": "Amsterdam", "isSubtle": true},
						map[string]interface{}{"type": "TextBlock", "size": "extraLarge", "color": "accent", "text": "AMS", "spacing": "none"},
					},
				},
				map[string]interface{}{
					"type":  "Column",
					"width": "auto",
					"items": []interface{}{
						map[string]interface{}{"type": "TextBlock", "text": " "},
						map[string]interface{}{"type": "Image", "url": "http://adaptivecards.io/content/airplane.png", "size": "small", "spacing": "none"},
					},
				},
				map[string]interface{}{
					"type":  "Column",
					"width": 1,
					"items": []interface{}{
						map[string]interface{}{"type": "TextBlock", "horizontalAlignment": "right", "text": "San Francisco", "isSubtle": true},
						map[string]interface{}{"type": "TextBlock", "horizontalAlignment": "right", "size": "extraLarge", "color": "accent", "text": "SFO", "spacing": "none"},
					},
				},
			},
		},
		map[string]interface{}{
			"type":    "ColumnSet",
			"spacing": "medium",
			"columns": []interface{}{
				map[string]interface{}{
					"type":  "Column",
					"width": "1",
					"items": []interface{}{
						map[string]interface{}{"type": "TextBlock", "text": "Total", "size": "medium", "isSubtle": true},
					},
				},
				map[string]interface{}{
					"type":  "Column",
					"width": 1,
					"items": []interface{}{
						map[string]interface{}{"type": "TextBlock", "horizontalAlignment": "right", "text": "$4,032.54", "size": "medium", "weight": "bolder"},
					},
				},
			},
		},
	},
}

// ---------------------------------------------------------------------------
// Handler registration
// ---------------------------------------------------------------------------

const usageText = `Cards Bot — send one of these commands:
  hero       — Hero Card
  thumbnail  — Thumbnail Card
  adaptive   — Adaptive Card (flight itinerary)
  all        — All card types at once`

// RegisterHandlers sets up message routing for the cards agent
func RegisterHandlers(app *AgentApp) {
	app.OnConversationUpdate("membersAdded", func(ctx *TurnContext) {
		if err := ctx.SendActivity("Welcome to the Cards Bot! " + usageText); err != nil {
			log.Printf("Error sending welcome: %v", err)
		}
	})

	// Hero card
	app.OnMessage(regexp.MustCompile(`(?i)^hero$`), func(ctx *TurnContext) {
		card := HeroCardAttachment(HeroCard{
			Title: "Copilot Hero Card",
			Images: []CardImage{
				{URL: "https://blogs.microsoft.com/wp-content/uploads/prod/2023/09/Press-Image_FINAL_16x9-4.jpg"},
			},
			Buttons: []CardAction{
				{Type: "openUrl", Title: "Get started", Value: "https://docs.microsoft.com/en-us/azure/bot-service/"},
			},
		})
		if err := ctx.SendActivityWithAttachment(card); err != nil {
			log.Printf("Error sending hero card: %v", err)
		}
	})

	// Thumbnail card
	app.OnMessage(regexp.MustCompile(`(?i)^thumbnail$`), func(ctx *TurnContext) {
		card := ThumbnailCardAttachment(ThumbnailCard{
			Title:    "Copilot Thumbnail Card",
			Subtitle: "Your bots - wherever your users are talking",
			Text:     "Build and connect intelligent bots to interact with your users naturally wherever they are, from text/sms to Skype, Slack, Office 365 mail and other popular services.",
			Images: []CardImage{
				{URL: "https://blogs.microsoft.com/wp-content/uploads/prod/2023/09/Press-Image_FINAL_16x9-4.jpg"},
			},
			Buttons: []CardAction{
				{Type: "openUrl", Title: "Get started", Value: "https://docs.microsoft.com/en-us/azure/bot-service/"},
			},
		})
		if err := ctx.SendActivityWithAttachment(card); err != nil {
			log.Printf("Error sending thumbnail card: %v", err)
		}
	})

	// Adaptive card
	app.OnMessage(regexp.MustCompile(`(?i)^adaptive$`), func(ctx *TurnContext) {
		card := AdaptiveCardAttachment(flightAdaptiveCard)
		if err := ctx.SendActivityWithAttachment(card); err != nil {
			log.Printf("Error sending adaptive card: %v", err)
		}
	})

	// All cards
	app.OnMessage(regexp.MustCompile(`(?i)^all$`), func(ctx *TurnContext) {
		cards := []Attachment{
			HeroCardAttachment(HeroCard{
				Title: "Copilot Hero Card",
				Images: []CardImage{
					{URL: "https://blogs.microsoft.com/wp-content/uploads/prod/2023/09/Press-Image_FINAL_16x9-4.jpg"},
				},
				Buttons: []CardAction{
					{Type: "openUrl", Title: "Get started", Value: "https://docs.microsoft.com/en-us/azure/bot-service/"},
				},
			}),
			ThumbnailCardAttachment(ThumbnailCard{
				Title:    "Copilot Thumbnail Card",
				Subtitle: "Your bots - wherever your users are talking",
				Text:     "Build and connect intelligent bots to interact with your users naturally wherever they are, from text/sms to Skype, Slack, Office 365 mail and other popular services.",
				Images: []CardImage{
					{URL: "https://blogs.microsoft.com/wp-content/uploads/prod/2023/09/Press-Image_FINAL_16x9-4.jpg"},
				},
				Buttons: []CardAction{
					{Type: "openUrl", Title: "Get started", Value: "https://docs.microsoft.com/en-us/azure/bot-service/"},
				},
			}),
			AdaptiveCardAttachment(flightAdaptiveCard),
		}
		reply := Activity{
			Type:         "message",
			From:         ChannelAccount{ID: app.botID, Name: app.botName},
			Conversation: ctx.Activity.Conversation,
			ReplyToID:    ctx.Activity.ID,
			Attachments:  cards,
		}
		ctx.writer.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(ctx.writer).Encode(reply); err != nil {
			log.Printf("Error sending all cards: %v", err)
		}
	})

	// Catch-all: usage instructions
	app.OnMessage(nil, func(ctx *TurnContext) {
		text := strings.TrimSpace(ctx.Activity.Text)
		msg := fmt.Sprintf("Command %q not recognized.\n\n%s", text, usageText)
		if err := ctx.SendActivity(msg); err != nil {
			log.Printf("Error sending usage: %v", err)
		}
	})

	app.OnError(func(ctx *TurnContext, err error) {
		log.Printf("[on_turn_error] unhandled error: %v", err)
		ctx.SendActivity("The bot encountered an error or bug.")
	})
}
