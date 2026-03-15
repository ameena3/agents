# Go SDK Conversion — Shared Patterns

This document is the canonical reference for all Go sample implementations in this repository. Every implementation agent MUST follow these patterns exactly.

---

## 1. Go Version

All `go.mod` files must use:

```
go 1.24
```

---

## 2. Package Convention

Each sample is its own standalone program:

```go
package main
```

---

## 3. Module Naming

```
github.com/microsoft/agents-for-go/samples/<sample-name>
```

Replace `<sample-name>` with the directory name of the sample (e.g., `quickstart`, `echo-bot`, etc.).

---

## 4. Core Type Definitions

Every agent MUST use these exact struct definitions:

```go
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
    Activity    *Activity
    writer      http.ResponseWriter
    serviceURL  string
    botID       string
    botName     string
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
```

---

## 5. AgentApp Pattern

Every agent MUST implement the following `AgentApp` pattern:

```go
type MessageHandler func(ctx *TurnContext)
type ConversationUpdateHandler func(ctx *TurnContext)
type ErrorHandler func(ctx *TurnContext, err error)

type AgentApp struct {
    messageHandlers            []messageEntry
    conversationUpdateHandlers map[string]ConversationUpdateHandler
    errorHandler               ErrorHandler
    botID                      string
    botName                    string
}

type messageEntry struct {
    pattern *regexp.Regexp // nil = catch-all
    handler MessageHandler
}

func NewAgentApp() *AgentApp {
    return &AgentApp{
        conversationUpdateHandlers: make(map[string]ConversationUpdateHandler),
        botID:   os.Getenv("BOT_ID"),
        botName: os.Getenv("BOT_NAME"),
    }
}

func (a *AgentApp) OnMessage(pattern *regexp.Regexp, handler MessageHandler) {
    a.messageHandlers = append(a.messageHandlers, messageEntry{pattern, handler})
}

func (a *AgentApp) OnConversationUpdate(event string, handler ConversationUpdateHandler) {
    a.conversationUpdateHandlers[event] = handler
}

func (a *AgentApp) OnError(handler ErrorHandler) {
    a.errorHandler = handler
}

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
```

---

## 6. go.mod Template

```
module github.com/microsoft/agents-for-go/samples/<SAMPLE-NAME>

go 1.24

require (
    github.com/joho/godotenv v1.5.1
)
```

Replace `<SAMPLE-NAME>` with the sample directory name.

---

## 7. Error Handling Convention

- Use `log.Printf` for logging errors.
- Return early after handling errors.
- Do not panic in handler code; use the `OnError` handler for unexpected errors.

Example:

```go
if err := ctx.SendActivity("Hello!"); err != nil {
    log.Printf("Error sending message: %v", err)
    return
}
```

---

## 8. env.TEMPLATE Format

Every sample MUST include an `env.TEMPLATE` file with the following format:

```
CONNECTIONS__SERVICE_CONNECTION__SETTINGS__CLIENTID=your-client-id
CONNECTIONS__SERVICE_CONNECTION__SETTINGS__CLIENTSECRET=your-client-secret
CONNECTIONS__SERVICE_CONNECTION__SETTINGS__TENANTID=your-tenant-id
BOT_ID=your-bot-app-id
BOT_NAME=YourBotName
PORT=3978
```

Update `BOT_NAME` to match the sample's bot name.

---

## Reference Implementation

The canonical reference implementation is in `samples/golang/quickstart/`. All other samples should follow the same file layout:

- `main.go` — entry point, loads env, starts server
- `agent.go` — all shared types, `AgentApp`, and `RegisterHandlers`
- `go.mod` — module definition
- `env.TEMPLATE` — environment variable template
- `README.md` — brief usage instructions

---

## Agent Decisions Log

- Agent 0 (Foundation): Established all core types, AgentApp pattern, and quickstart reference implementation.
- Agent 2 (cards): AdaptiveCard body uses map[string]interface{}. Card content types follow Bot Framework spec. CardFactory functions return Attachment.
- Agent 4 (azureai-streaming): Azure OpenAI called via net/http REST (no SDK dep). Streaming implemented as collect-then-send due to Bot Framework protocol constraints. SSE parsing done manually on response body.
- Agent 5 (copilotstudio-client, copilotstudio-skill): Client uses Direct Line REST API with polling (no WebSocket). Skill uses standard AgentApp with added OnInvoke handler. Token management is simplified - production use requires proper MSAL integration.
