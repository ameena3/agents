# Copilot Studio Client (Go)

A console client that connects to a Copilot Studio agent via the Direct Line REST API and conducts an interactive conversation.

## Prerequisites

- Go 1.24+
- A Copilot Studio agent with a Direct Line channel enabled
- A valid Direct Line token

## Setup

1. Copy `env.TEMPLATE` to `.env` and fill in your values:

```
COPILOT_ENVIRONMENT_ID=your-environment-id
COPILOT_AGENT_ID=your-agent-id
COPILOT_TENANT_ID=your-tenant-id
DIRECTLINE_TOKEN=your-directline-token
```

2. Run the client:

```bash
go run .
```

## Usage

The client starts a conversation with the Copilot Studio agent and enters an interactive loop. Type your message and press Enter to send it. Type `quit` or `exit` to end the session.

## Notes

- This sample uses the Direct Line REST API with simple polling (no WebSocket).
- Token management is simplified; production use requires proper MSAL integration for acquiring and refreshing tokens.
