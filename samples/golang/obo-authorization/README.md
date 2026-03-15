# OBO Authorization Sample (Go)

This sample demonstrates the On-Behalf-Of (OBO) OAuth flow using the Microsoft Agents SDK for Go. The agent obtains an initial user token via the Bot Framework OAuth flow, then exchanges it for a token scoped to a downstream service (such as Copilot Studio) using the OBO grant.

## Overview

The OBO flow proceeds in three steps:

1. The user sends a message; the bot sends an OAuthCard to initiate sign-in.
2. After sign-in, the channel delivers a `tokens/response` event with the user's token.
3. The bot exchanges this token for a new token scoped to the downstream service by calling the Azure AD v2 token endpoint with the `urn:ietf:params:oauth:grant-type:jwt-bearer` grant type.

## Commands

- Any message — triggers sign-in (if not signed in) or performs the OBO token exchange and reports the result.
- `/signout` — clears the stored token and signs out.

## Setup

1. Copy `env.TEMPLATE` to `.env` and fill in your values.
2. Register your app in Azure AD with the required API permissions for the target service.
3. Configure an OAuth connection in Azure Bot Service.
4. Run the bot:

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
| `BOT_ID` | Bot application ID |
| `BOT_NAME` | Display name for the bot |
| `PORT` | HTTP port (default: `3978`) |

## Notes

The OBO token exchange is implemented directly against the Azure AD v2 token endpoint (`https://login.microsoftonline.com/{tenant}/oauth2/v2.0/token`). Once the MSAL Go library (`github.com/AzureAD/microsoft-authentication-library-for-go`) exposes a first-class OBO API via `ConfidentialClientApplication.AcquireTokenOnBehalfOf`, the `ExchangeTokenOBO` function in `agent.go` should be updated to use that instead.
