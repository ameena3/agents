// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

// Package main implements the On-Behalf-Of (OBO) authorization sample.
//
// The OBO flow allows an agent to exchange an incoming user token for a new
// token scoped to a downstream service (e.g., Copilot Studio). The steps are:
//
//  1. The user signs in via the Bot Framework OAuth flow; the channel delivers a
//     tokens/response event carrying the user's token for the configured connection.
//  2. The agent calls Azure AD's token endpoint using the OBO grant
//     (urn:ietf:params:oauth:grant-type:jwt-bearer) to exchange the incoming
//     token for a token scoped to the target service.
//  3. The downstream service (e.g., Copilot Studio) is called with the new token.
//
// NOTE: The MSAL Go library (AzureAD/microsoft-authentication-library-for-go)
// does not yet expose a first-class OBO API. The ExchangeTokenOBO function below
// implements the raw HTTP token exchange directly against the AAD v2 endpoint.
// Replace this stub with the MSAL ConfidentialClientApplication.AcquireTokenOnBehalfOf
// method once it becomes available in the library.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
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
	Name         string              `json:"name,omitempty"`
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
	// tokenStore holds incoming OAuth tokens keyed by conversation ID.
	// These tokens are later exchanged via the OBO flow.
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

// tokenResponsePayload is the payload delivered in a tokens/response event
type tokenResponsePayload struct {
	ConnectionName string `json:"connectionName"`
	Token          string `json:"token"`
}

// handleTokenResponse stores the incoming token from the OAuth flow.
// This token will later be exchanged via the OBO grant for a downstream service token.
func (a *AgentApp) handleTokenResponse(ctx *TurnContext) {
	convID := ctx.Activity.Conversation.ID

	valueBytes, err := json.Marshal(ctx.Activity.Value)
	if err != nil {
		log.Printf("Error marshaling token value: %v", err)
		return
	}
	var tr tokenResponsePayload
	if err := json.Unmarshal(valueBytes, &tr); err != nil {
		log.Printf("Error decoding token response: %v", err)
		return
	}

	// Store the incoming token keyed by conversation ID
	a.tokenStore.Store(convID, tr.Token)
	log.Printf("Stored incoming token for conversation %s (connection: %s)", convID, tr.ConnectionName)

	if err := ctx.SendActivity("You have been signed in. Send any message to perform the OBO token exchange."); err != nil {
		log.Printf("Error sending sign-in confirmation: %v", err)
	}
}

// oboTokenResponse is the JSON body returned by the Azure AD token endpoint
type oboTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

// ExchangeTokenOBO performs an On-Behalf-Of token exchange against the Azure AD v2
// token endpoint. It trades the incoming userAssertion token for a new token scoped
// to targetScopes.
//
// OBO flow steps:
//  1. Send a POST to https://login.microsoftonline.com/{tenant}/oauth2/v2.0/token
//  2. Use grant_type=urn:ietf:params:oauth:grant-type:jwt-bearer
//  3. Include client_id, client_secret, the incoming token as assertion, and the
//     requested scope(s) for the downstream service.
//  4. Azure AD validates the assertion and returns a new access token.
//
// TODO: Replace this raw HTTP implementation with the MSAL Go library's
// ConfidentialClientApplication.AcquireTokenOnBehalfOf once that API is
// available in github.com/AzureAD/microsoft-authentication-library-for-go.
func ExchangeTokenOBO(userAssertion string, targetScopes []string) (string, error) {
	tenantID := os.Getenv("CONNECTIONS__SERVICE_CONNECTION__SETTINGS__TENANTID")
	clientID := os.Getenv("CONNECTIONS__SERVICE_CONNECTION__SETTINGS__CLIENTID")
	clientSecret := os.Getenv("CONNECTIONS__SERVICE_CONNECTION__SETTINGS__CLIENTSECRET")

	if tenantID == "" || clientID == "" || clientSecret == "" {
		return "", fmt.Errorf("OBO exchange requires TENANTID, CLIENTID, and CLIENTSECRET environment variables")
	}

	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenantID)

	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	formData.Set("client_id", clientID)
	formData.Set("client_secret", clientSecret)
	formData.Set("assertion", userAssertion)
	formData.Set("requested_token_use", "on_behalf_of")
	formData.Set("scope", strings.Join(targetScopes, " "))

	resp, err := http.PostForm(tokenURL, formData)
	if err != nil {
		return "", fmt.Errorf("OBO token request failed: %w", err)
	}
	defer resp.Body.Close()

	var tokenResp oboTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode OBO token response: %w", err)
	}

	if tokenResp.Error != "" {
		return "", fmt.Errorf("OBO token error %s: %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	return tokenResp.AccessToken, nil
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

// RegisterHandlers wires up all message and event handlers for the OBO agent
func RegisterHandlers(app *AgentApp) {
	// The connection name used to obtain the initial user token
	connectionName := os.Getenv("CONNECTIONS__SERVICE_CONNECTION__SETTINGS__CONNECTIONNAME")
	if connectionName == "" {
		connectionName = "MCS"
	}

	// Welcome message on new conversation
	app.OnConversationUpdate("membersAdded", func(ctx *TurnContext) {
		if err := ctx.SendActivity(
			"Welcome to the OBO Authorization sample!\n" +
				"Send any message to initiate the sign-in and OBO token exchange flow.\n" +
				"Send /signout to sign out.",
		); err != nil {
			log.Printf("Error sending welcome: %v", err)
		}
	})

	// Sign-out command
	app.OnMessage(regexp.MustCompile(`(?i)^/signout$`), func(ctx *TurnContext) {
		convID := ctx.Activity.Conversation.ID
		app.tokenStore.Delete(convID)
		if err := ctx.SendActivity("You have been signed out."); err != nil {
			log.Printf("Error sending signout confirmation: %v", err)
		}
	})

	// Catch-all: if no token is stored, initiate sign-in; otherwise perform OBO exchange
	app.OnMessage(nil, func(ctx *TurnContext) {
		convID := ctx.Activity.Conversation.ID

		// Check if we already have an incoming token for this conversation
		val, ok := app.tokenStore.Load(convID)
		if !ok {
			// Step 1: No token yet — send an OAuthCard to initiate sign-in
			log.Printf("No token for conversation %s, initiating sign-in", convID)
			if err := sendOAuthCard(ctx, connectionName, "Please sign in to continue."); err != nil {
				log.Printf("Error sending OAuth card: %v", err)
			}
			return
		}

		incomingToken, ok := val.(string)
		if !ok || incomingToken == "" {
			if err := ctx.SendActivity("Token is invalid. Please sign in again by sending any message."); err != nil {
				log.Printf("Error sending invalid token message: %v", err)
			}
			app.tokenStore.Delete(convID)
			return
		}

		// Step 2: Perform the OBO token exchange for the Copilot Studio scope.
		// The target scope is the Power Platform API audience.
		// Adjust the scope to match the downstream service you are calling.
		targetScopes := []string{"https://api.powerplatform.com/.default"}

		log.Printf("Performing OBO exchange for conversation %s", convID)
		oboToken, err := ExchangeTokenOBO(incomingToken, targetScopes)
		if err != nil {
			log.Printf("OBO exchange failed: %v", err)
			if sendErr := ctx.SendActivity(fmt.Sprintf(
				"OBO token exchange failed: %v\n\nEnsure your app registration has the required API permissions and the incoming token is valid.",
				err,
			)); sendErr != nil {
				log.Printf("Error sending OBO error message: %v", sendErr)
			}
			return
		}

		// Step 3: Use the exchanged token to call the downstream service.
		// For demonstration purposes, report the token length rather than its value.
		log.Printf("OBO exchange successful for conversation %s, token length: %d", convID, len(oboToken))
		if err := ctx.SendActivity(fmt.Sprintf(
			"OBO token exchange successful!\n"+
				"You said: %s\n"+
				"Exchanged token length: %d characters\n\n"+
				"The exchanged token can now be used to call the downstream service (e.g., Copilot Studio).",
			ctx.Activity.Text,
			len(oboToken),
		)); err != nil {
			log.Printf("Error sending OBO success message: %v", err)
		}
	})

	app.OnError(func(ctx *TurnContext, err error) {
		log.Printf("[on_turn_error] unhandled error: %v", err)
		ctx.SendActivity("The bot encountered an error or bug.")
	})
}
