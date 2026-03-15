# Azure OpenAI Streaming Sample (Go)

This sample demonstrates calling the Azure OpenAI API with streaming enabled and returning the response through the Bot Framework.

## How It Works

1. A user sends a message containing the word **poem**.
2. The bot sends an immediate "Thinking..." acknowledgement.
3. The bot calls the Azure OpenAI chat completions API with `"stream": true`.
4. The SSE (server-sent events) response is parsed chunk-by-chunk as it arrives.
5. All chunks are collected and the assembled text is sent back as a single Bot Framework activity.

> **Note on streaming:** True token-level streaming to the end user requires the Azure Bot Service [Direct Line streaming protocol](https://docs.microsoft.com/azure/bot-service/rest-api/bot-framework-rest-direct-line-3-0-receive-activities) over WebSockets. The standard `/api/messages` HTTP response does not support SSE to the client. This sample uses a collect-then-send approach as a pragmatic alternative that works with any Bot Framework channel.

## Prerequisites

- Go 1.24 or later
- An Azure OpenAI resource with a deployed model (e.g., `gpt-4`)
- A registered Azure Bot (for Bot Framework channels)

## Setup

1. Copy `env.TEMPLATE` to `.env` and fill in your values:

   ```
   cp env.TEMPLATE .env
   ```

2. Edit `.env` with your Azure OpenAI endpoint, deployment name, and API key.

## Running Locally

```bash
go run .
```

The bot listens on port `3978` by default (configurable via `PORT`).

## Testing

Use the Bot Framework Emulator or any Bot Framework channel. Send the message `poem` to trigger an Azure OpenAI poem about Apollo from the Percy Jackson series.

## Environment Variables

| Variable | Description |
|---|---|
| `AZURE_OPENAI_ENDPOINT` | Azure OpenAI resource endpoint (e.g., `https://your-resource.openai.azure.com`) |
| `AZURE_OPENAI_DEPLOYMENT` | Deployment name (e.g., `gpt-4`) |
| `AZURE_OPENAI_API_KEY` | Azure OpenAI API key |
| `BOT_ID` | Azure Bot App ID |
| `BOT_NAME` | Display name for the bot |
| `PORT` | Port to listen on (default: `3978`) |
