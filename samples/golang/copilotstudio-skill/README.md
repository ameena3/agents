# Copilot Studio Skill (Go)

A Bot Framework skill server that acts as a backend skill for Copilot Studio. It receives activities from a Copilot Studio orchestrator agent and responds accordingly.

## Prerequisites

- Go 1.24+
- An Azure Bot registration (App ID and Secret)
- A Copilot Studio agent configured to call this skill

## Setup

1. Copy `env.TEMPLATE` to `.env` and fill in your values:

```
CONNECTIONS__SERVICE_CONNECTION__SETTINGS__CLIENTID=your-client-id
CONNECTIONS__SERVICE_CONNECTION__SETTINGS__CLIENTSECRET=your-client-secret
CONNECTIONS__SERVICE_CONNECTION__SETTINGS__TENANTID=your-tenant-id
BOT_ID=your-bot-app-id
BOT_NAME=CopilotSkillBot
PORT=3978
```

2. Run the skill server:

```bash
go run .
```

## Behavior

- On `conversationUpdate` (membersAdded): sends "Copilot Studio skill is ready!"
- On `message`: echoes back with prefix "Skill received: {text}"
- On `endOfConversation`: logs the end of conversation
- On `invoke` with `skillBegin`: acknowledges skill start
- On `invoke` with `skillEnd`: acknowledges skill end

## Notes

- The skill listens on `/api/messages` (POST).
- Register the skill's endpoint in your Copilot Studio agent configuration.
- Authentication uses the standard Bot Framework JWT validation pattern.
