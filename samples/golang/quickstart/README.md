# Quickstart Agent (Go)

This is the reference implementation for a Go-based Bot Framework agent. It demonstrates the core `AgentApp` pattern, message routing, and conversation update handling.

## Prerequisites

- [Go 1.26+](https://go.dev/dl/)
- A Microsoft Azure Bot registration (for production use; local testing does not require one)

## Setup

1. Copy `env.TEMPLATE` to `.env` and fill in your values:

   ```bash
   cp env.TEMPLATE .env
   ```

2. Edit `.env` with your Azure Bot credentials:

   ```
   CONNECTIONS__SERVICE_CONNECTION__SETTINGS__CLIENTID=<your-client-id>
   CONNECTIONS__SERVICE_CONNECTION__SETTINGS__CLIENTSECRET=<your-client-secret>
   CONNECTIONS__SERVICE_CONNECTION__SETTINGS__TENANTID=<your-tenant-id>
   BOT_ID=<your-bot-app-id>
   BOT_NAME=QuickstartBot
   PORT=3978
   ```

## Running the Sample

```bash
go run .
```

The agent will start listening on port `3978` (or the value of the `PORT` environment variable).

## What It Does

- Sends a welcome message when a new user joins the conversation.
- Responds with `Hello!` when the user sends the word "hello" (case-insensitive).
- Echoes back any other message: `you said: <message>`.

## File Structure

| File           | Description                                              |
|----------------|----------------------------------------------------------|
| `main.go`      | Entry point: loads env, initializes app, starts server   |
| `agent.go`     | All types, `AgentApp` routing, and `RegisterHandlers`    |
| `go.mod`       | Module definition                                        |
| `env.TEMPLATE` | Template for required environment variables              |

## Testing Locally

You can use [Bot Framework Emulator](https://github.com/microsoft/BotFramework-Emulator) to connect to `http://localhost:3978/api/messages` and interact with the bot.
