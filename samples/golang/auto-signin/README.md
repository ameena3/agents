# Auto Sign-In Sample (Go)

This sample demonstrates OAuth sign-in flows using Microsoft Graph and GitHub connections within the Microsoft Agents SDK for Go.

## Overview

The bot supports the following commands:

- `/status` — show sign-in status for Graph and GitHub connections
- `/me` or `/profile` — retrieve and display your Microsoft Graph user profile
- `/prs` or `/pull-requests` — list open GitHub pull requests (from `octocat/Hello-World`)
- `/logout` — sign out and clear all stored tokens

When a command requires authentication and no token is cached, the bot sends an OAuthCard to initiate the sign-in flow. After the user completes sign-in, the channel delivers a `tokens/response` event containing the token, which is stored in memory keyed by conversation ID.

## Setup

1. Copy `env.TEMPLATE` to `.env` and fill in your values.
2. Register your bot in Azure Bot Service and configure OAuth connections named `GRAPH` and `GITHUB`.
3. Run the bot:

```bash
go run .
```

The bot listens on `PORT` (default `3978`) at `/api/messages`.

## Environment Variables

| Variable | Description |
|---|---|
| `CONNECTIONS__SERVICE_CONNECTION__SETTINGS__CLIENTID` | Azure AD application client ID |
| `CONNECTIONS__SERVICE_CONNECTION__SETTINGS__CLIENTSECRET` | Azure AD application client secret |
| `CONNECTIONS__SERVICE_CONNECTION__SETTINGS__TENANTID` | Azure AD tenant ID |
| `CONNECTIONS__GRAPH__SETTINGS__CONNECTIONNAME` | OAuth connection name for Microsoft Graph (default: `GRAPH`) |
| `CONNECTIONS__GITHUB__SETTINGS__CONNECTIONNAME` | OAuth connection name for GitHub (default: `GITHUB`) |
| `BOT_ID` | Bot application ID |
| `BOT_NAME` | Display name for the bot |
| `PORT` | HTTP port (default: `3978`) |
