# Cards Sample — Go

This sample demonstrates rich card types supported by the Bot Framework: Hero Cards, Thumbnail Cards, and Adaptive Cards.

## Prerequisites

- Go 1.21+
- A registered Bot Framework bot (App ID and password)

## Setup

1. Copy `env.TEMPLATE` to `.env` and fill in your bot credentials.
2. Run `go mod tidy` to fetch dependencies.

## Running

```bash
go run .
```

The agent listens on port `3978` by default (override with `PORT` environment variable).

## Usage

Send any of the following messages to the bot:

| Command     | Result                              |
|-------------|-------------------------------------|
| `hero`      | Sends a Hero Card                   |
| `thumbnail` | Sends a Thumbnail Card              |
| `adaptive`  | Sends an Adaptive Card              |
| `all`       | Sends all card types in one message |

Any other input returns usage instructions.
