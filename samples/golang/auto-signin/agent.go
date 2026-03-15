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
	"sync"
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

// CardAction represents a button or action on a card
type CardAction struct {
	Type  string `json:"type"`
	Title string `json:"title"`
	Value string `json:"value"`
}

// OAuthCard represents an OAuth sign-in card attachment
type OAuthCard struct {
	Text           string       `json:"text"`
	ConnectionName string       `json:"connectionName"`
	Buttons        []CardAction `json:"buttons,omitempty"`
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
	// tokenStore holds OAuth tokens keyed by conversation ID
	tokenStore sync.Map
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
	case "event":
		// Token response events arrive as "event" activities with name "tokens/response"
		if activity.Name == "tokens/response" {
			a.handleTokenResponse(ctx)
		}
	}
}

// tokenResponse is the payload delivered in a tokens/response event
type tokenResponse struct {
	ConnectionName string `json:"connectionName"`
	Token          string `json:"token"`
}

// handleTokenResponse processes the OAuth token delivered by the channel after sign-in
func (a *AgentApp) handleTokenResponse(ctx *TurnContext) {
	convID := ctx.Activity.Conversation.ID

	// Decode the token payload from Activity.Value
	valueBytes, err := json.Marshal(ctx.Activity.Value)
	if err != nil {
		log.Printf("Error marshaling token value: %v", err)
		return
	}
	var tr tokenResponse
	if err := json.Unmarshal(valueBytes, &tr); err != nil {
		log.Printf("Error decoding token response: %v", err)
		return
	}

	// Store the token keyed by "<convID>/<connectionName>"
	storeKey := convID + "/" + tr.ConnectionName
	a.tokenStore.Store(storeKey, tr.Token)
	log.Printf("Stored token for connection %q (conversation %s)", tr.ConnectionName, convID)

	if err := ctx.SendActivity("You have been signed in successfully!"); err != nil {
		log.Printf("Error sending sign-in confirmation: %v", err)
	}
}

// getToken retrieves a stored token for the given conversation and connection name.
// Returns the token string and true if found.
func (a *AgentApp) getToken(convID, connectionName string) (string, bool) {
	storeKey := convID + "/" + connectionName
	val, ok := a.tokenStore.Load(storeKey)
	if !ok {
		return "", false
	}
	token, ok := val.(string)
	return token, ok
}

// sendOAuthCard sends an OAuthCard attachment that initiates the sign-in flow
func sendOAuthCard(ctx *TurnContext, connectionName, text string) error {
	card := OAuthCard{
		Text:           text,
		ConnectionName: connectionName,
		Buttons: []CardAction{
			{
				Type:  "signin",
				Title: "Sign In",
				Value: "",
			},
		},
	}
	attachment := Attachment{
		ContentType: "application/vnd.microsoft.card.oauth",
		Content:     card,
	}
	return ctx.SendActivityWithAttachment(attachment)
}

// RegisterHandlers wires up all message and event handlers for the auto-signin agent
func RegisterHandlers(app *AgentApp) {
	graphConn := os.Getenv("CONNECTIONS__GRAPH__SETTINGS__CONNECTIONNAME")
	if graphConn == "" {
		graphConn = "GRAPH"
	}
	githubConn := os.Getenv("CONNECTIONS__GITHUB__SETTINGS__CONNECTIONNAME")
	if githubConn == "" {
		githubConn = "GITHUB"
	}

	// Welcome message on new conversation
	app.OnConversationUpdate("membersAdded", func(ctx *TurnContext) {
		if err := ctx.SendActivity(
			"Welcome to the Auto Sign-In sample! Commands:\n" +
				"- /status — check sign-in status\n" +
				"- /me or /profile — show your Microsoft Graph profile\n" +
				"- /prs or /pull-requests — show GitHub pull requests\n" +
				"- /logout — sign out of all connections",
		); err != nil {
			log.Printf("Error sending welcome: %v", err)
		}
	})

	// Status: report which connections have tokens
	app.OnMessage(regexp.MustCompile(`(?i)^/(status|auth status|check status)$`), func(ctx *TurnContext) {
		convID := ctx.Activity.Conversation.ID
		_, hasGraph := app.getToken(convID, graphConn)
		_, hasGitHub := app.getToken(convID, githubConn)

		graphStatus := "Not connected"
		if hasGraph {
			graphStatus = "Connected"
		}
		githubStatus := "Not connected"
		if hasGitHub {
			githubStatus = "Connected"
		}

		if err := ctx.SendActivity(fmt.Sprintf(
			"Graph status: %s\nGitHub status: %s", graphStatus, githubStatus,
		)); err != nil {
			log.Printf("Error sending status: %v", err)
		}
	})

	// Logout: clear stored tokens for this conversation
	app.OnMessage(regexp.MustCompile(`(?i)^/logout$`), func(ctx *TurnContext) {
		convID := ctx.Activity.Conversation.ID
		app.tokenStore.Delete(convID + "/" + graphConn)
		app.tokenStore.Delete(convID + "/" + githubConn)
		if err := ctx.SendActivity("You have been logged out."); err != nil {
			log.Printf("Error sending logout confirmation: %v", err)
		}
	})

	// /me or /profile: retrieve Microsoft Graph user profile
	app.OnMessage(regexp.MustCompile(`(?i)^/(me|profile)$`), func(ctx *TurnContext) {
		convID := ctx.Activity.Conversation.ID
		token, ok := app.getToken(convID, graphConn)
		if !ok {
			// No token yet — send OAuth sign-in card to start the flow
			if err := sendOAuthCard(ctx, graphConn, "Please sign in to Microsoft to view your profile."); err != nil {
				log.Printf("Error sending Graph OAuth card: %v", err)
			}
			return
		}

		user, err := GetGraphUser(token)
		if err != nil {
			log.Printf("Error calling Microsoft Graph: %v", err)
			if sendErr := ctx.SendActivity("Failed to retrieve your Microsoft Graph profile. Please try signing in again."); sendErr != nil {
				log.Printf("Error sending error message: %v", sendErr)
			}
			return
		}

		msg := fmt.Sprintf("Microsoft Graph profile:\nName: %s\nEmail: %s\nID: %s",
			user.DisplayName, user.Mail, user.ID)
		if err := ctx.SendActivity(msg); err != nil {
			log.Printf("Error sending Graph profile: %v", err)
		}
	})

	// /prs or /pull-requests: retrieve GitHub pull requests
	app.OnMessage(regexp.MustCompile(`(?i)^/(prs|pull.?requests?)$`), func(ctx *TurnContext) {
		convID := ctx.Activity.Conversation.ID
		token, ok := app.getToken(convID, githubConn)
		if !ok {
			// No token yet — send OAuth sign-in card to start the flow
			if err := sendOAuthCard(ctx, githubConn, "Please sign in to GitHub to view pull requests."); err != nil {
				log.Printf("Error sending GitHub OAuth card: %v", err)
			}
			return
		}

		ghUser, err := GetGitHubUser(token)
		if err != nil {
			log.Printf("Error calling GitHub API (user): %v", err)
			if sendErr := ctx.SendActivity("Failed to retrieve your GitHub profile."); sendErr != nil {
				log.Printf("Error sending error message: %v", sendErr)
			}
			return
		}

		profileMsg := fmt.Sprintf("GitHub profile:\nLogin: %s\nName: %s\nEmail: %s",
			ghUser.Login, ghUser.Name, ghUser.Email)
		if err := ctx.SendActivity(profileMsg); err != nil {
			log.Printf("Error sending GitHub profile: %v", err)
		}

		// Fetch pull requests from a public repository (using octocat/Hello-World as example)
		prs, err := GetGitHubPullRequests("octocat", "Hello-World", token)
		if err != nil {
			log.Printf("Error fetching GitHub pull requests: %v", err)
			if sendErr := ctx.SendActivity("Failed to retrieve pull requests."); sendErr != nil {
				log.Printf("Error sending error message: %v", sendErr)
			}
			return
		}

		if len(prs) == 0 {
			if err := ctx.SendActivity("No open pull requests found."); err != nil {
				log.Printf("Error sending no-PRs message: %v", err)
			}
			return
		}

		for _, pr := range prs {
			msg := fmt.Sprintf("PR #%d: %s\n%s", pr.Number, pr.Title, pr.HTMLURL)
			if err := ctx.SendActivity(msg); err != nil {
				log.Printf("Error sending PR message: %v", err)
			}
		}
	})

	// Catch-all: echo the message back
	app.OnMessage(nil, func(ctx *TurnContext) {
		if err := ctx.SendActivity(fmt.Sprintf("You said: %s", ctx.Activity.Text)); err != nil {
			log.Printf("Error sending echo: %v", err)
		}
	})

	app.OnError(func(ctx *TurnContext, err error) {
		log.Printf("[on_turn_error] unhandled error: %v", err)
		ctx.SendActivity("The bot encountered an error or bug.")
	})
}
