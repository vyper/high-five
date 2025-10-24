# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A Go application for Google Cloud Functions Gen2 that implements a Slack kudos/compliments system. The application is split into two separate Cloud Functions:

- **SlashCommand**: Handles the `/elogie` slash command
- **Interactivity**: Handles modal interactions (block actions and view submissions)

## Development Commands

### Running Locally

```bash
# Run slash command function locally
go run cmd/slash-command/main.go

# Run interactivity function locally
go run cmd/interactivity/main.go

# Function available at http://localhost:8080
# Use PORT env var to change port
# Use LOCAL_ONLY=true to bind to 127.0.0.1 only
```

### Testing

**Important**: Some tests require environment variables to be set. Always use `./...` to test all packages.

```bash
# Set required environment variables first
export SLACK_BOT_TOKEN=xoxb-test
export SLACK_CHANNEL_ID=C123
export SLACK_SIGNING_SECRET=secret

# Run all tests with verbose output
go test -v ./...

# Run all tests (non-verbose)
go test ./...

# Run tests for specific package
go test -v ./internal/services
go test -v ./functions/slashcommand

# Generate coverage report for all packages
go test ./... -coverprofile=coverage.out -covermode=atomic
go tool cover -html=coverage.out -o coverage.html

# View coverage summary
go tool cover -func=coverage.out | tail -1

# Run benchmarks
go test ./... -bench=. -benchmem
```

**One-liner to run all tests:**
```bash
SLACK_BOT_TOKEN=xoxb-test SLACK_CHANNEL_ID=C123 SLACK_SIGNING_SECRET=secret go test -v ./...
```

**Test Coverage:**
- Overall: **85.5%** coverage
- `internal/config`: 100%
- `internal/handlers`: 100%
- `internal/services`: 92.5%
- `functions/slashcommand`: 73.3%
- `functions/interactivity`: 86.2%
- Total test cases: **137**
```

### Building & Dependencies
```bash
# Install/update dependencies
go mod tidy

# Build (if needed)
go build ./...
```

### Deployment

Deploy each function separately to Google Cloud:

```bash
# Deploy slash command function
gcloud beta functions deploy matter-give-kudos-slash \
  --project 'parafuzo-qa-infra' \
  --gen2 \
  --entry-point SlashCommand \
  --region us-east1 \
  --runtime go125 \
  --source . \
  --trigger-http \
  --allow-unauthenticated \
  --memory 128MiB \
  --set-env-vars "SLACK_BOT_TOKEN=$SLACK_BOT_TOKEN,SLACK_CHANNEL_ID=$SLACK_CHANNEL_ID,SLACK_SIGNING_SECRET=$SLACK_SIGNING_SECRET"

# Deploy interactivity function
gcloud beta functions deploy matter-give-kudos-interactivity \
  --project 'parafuzo-qa-infra' \
  --gen2 \
  --entry-point Interactivity \
  --region us-east1 \
  --runtime go125 \
  --source . \
  --trigger-http \
  --allow-unauthenticated \
  --memory 128MiB \
  --set-env-vars "SLACK_BOT_TOKEN=$SLACK_BOT_TOKEN,SLACK_CHANNEL_ID=$SLACK_CHANNEL_ID,SLACK_SIGNING_SECRET=$SLACK_SIGNING_SECRET"
```

## Architecture

### Two-Function Design

The application uses **separate Cloud Functions** for different responsibilities:

1. **SlashCommand Function** (`functions/slashcommand/`)
   - Entry point: `SlashCommand`
   - Handles `/elogie` slash command
   - Opens the kudos modal via Slack's `views.open` API
   - Validates Slack signing secret

2. **Interactivity Function** (`functions/interactivity/`)
   - Entry point: `Interactivity`
   - Handles all modal interactions
   - Routes to block actions handler (for dropdown changes)
   - Routes to view submission handler (for final submission)
   - Validates Slack signing secret

### Package Structure

```
internal/
├── config/          # Configuration loading and interfaces
│   └── config.go    # SlackClient & HTTPClient interfaces for mocking
├── handlers/        # HTTP request handlers (business logic entry points)
│   ├── slash_command.go      # Processes /elogie command
│   ├── block_actions.go      # Updates modal when kudo type selected
│   └── view_submission.go    # Posts kudos message when modal submitted
├── services/        # Business logic services
│   ├── modal.go     # OpenModal(), UpdateModal() - Slack views API
│   ├── kudos.go     # PostKudos(), ParseKudoTypeText()
│   └── formatting.go # Slack message formatting (Block Kit)
├── models/          # Data models and constants
│   └── kudos_types.go # KudoSuggestedMessages, KudoDescriptions maps
└── templates/       # Slack modal templates
    ├── templates.go        # Embeds give-kudos.json
    └── give-kudos.json     # Slack Block Kit modal definition
```

### Request Flow

**Slash Command Flow:**
1. User types `/elogie` in Slack
2. Slack POSTs to SlashCommand function
3. `functions/slashcommand/function.go` verifies signature
4. `handlers.HandleSlashCommand()` extracts trigger_id
5. `services.OpenModal()` calls Slack's `views.open` API
6. Modal appears in Slack

**Interaction Flow (Kudo Type Selection):**
1. User selects kudo type in modal dropdown
2. Slack POSTs to Interactivity function with `block_actions` type
3. `functions/interactivity/function.go` verifies signature, parses payload
4. `handlers.HandleBlockActions()` extracts selected kudo type
5. Looks up description from `models.KudoDescriptions`
6. `services.UpdateModal()` calls Slack's `views.update` API
7. Modal updates to show description and suggested message

**Submission Flow:**
1. User fills form and clicks Submit
2. Slack POSTs to Interactivity function with `view_submission` type
3. `handlers.HandleViewSubmission()` extracts form values
4. `services.PostKudos()` formats message with `services.FormatKudosAsBlocks()`
5. Posts to configured Slack channel using Block Kit format

### Configuration

All functions require three environment variables (loaded via `internal/config`):
- `SLACK_BOT_TOKEN`: Bot user OAuth token (starts with `xoxb-`)
- `SLACK_CHANNEL_ID`: Target channel ID for kudos messages
- `SLACK_SIGNING_SECRET`: For request signature verification

Configuration uses dependency injection pattern with interfaces (`SlackClient`, `HTTPClient`) to enable mocking in tests.

### Key Design Patterns

- **Interfaces for external dependencies**: `config.SlackClient` and `config.HTTPClient` enable testing without real Slack/HTTP calls
- **Handler-Service separation**: Handlers deal with HTTP, Services contain business logic
- **Embedded templates**: `//go:embed` for Slack modal JSON template
- **Template manipulation**: Modal JSON is unmarshaled, modified (adding description block), and remarshaled for dynamic updates

### Modal Update Mechanism

When user selects a kudo type, the modal dynamically updates:
1. Preserves user's message input if they've already typed
2. Inserts a "kudo_description" context block after the dropdown
3. Pre-fills suggested message only if message field is empty
4. Uses `views.update` with view ID and hash for optimistic concurrency

### Kudos Types

10 predefined kudos types in `models.KudoSuggestedMessages` and `models.KudoDescriptions`:
- entrega-excepcional, espirito-de-equipe, ideia-brilhante, acima-e-alem, mestre-em-ensinar, resolvedor-de-problemas, atitude-positiva, crescimento-continuo, conquista-do-time, resiliencia

Each has a Portuguese emoji-prefixed display name, description, and suggested message template.

## Important Implementation Notes

- **Slack signature verification**: Both functions verify requests using `slack.NewSecretsVerifier`
- **Multiple entry points**: Each function has its own `main.go` in `cmd/` and wrapper in root directory
- **Functions Framework**: Uses Google Cloud Functions Framework for Go (`funcframework`)
- **Block Kit formatting**: Messages use Slack's Block Kit with headers, sections, dividers, and context blocks
- **Error handling**: Errors are logged but view submissions don't return errors to user (modal already closed)
