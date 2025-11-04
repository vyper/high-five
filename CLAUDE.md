# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A Go application for Google Cloud Functions Gen2 that implements a Slack kudos/compliments system. The application is split into three separate Cloud Functions:

- **SlashCommand**: Handles the `/elogie` slash command
- **Interactivity**: Handles modal interactions (block actions and view submissions)
- **Reminder**: Sends weekly DM reminders to channel members (triggered by Cloud Scheduler)

## Development Commands

### Running Locally

```bash
# Run slash command function locally
go run cmd/slash-command/main.go

# Run interactivity function locally
go run cmd/interactivity/main.go

# Run reminder function locally
go run cmd/reminder/main.go

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
- Overall: **82.4%** coverage
- `internal/config`: 100%
- `internal/handlers`: 95.7%
- `internal/services`: 89.5%
- `functions/slashcommand`: 68.8%
- `functions/interactivity`: 83.3%
- `functions/reminder`: 91.3%
- Total test cases: **150+**
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
gcloud functions deploy matter-slash-command \
  --project 'parafuzo-qa-infra' \
  --gen2 \
  --entry-point SlashCommand \
  --region us-east1 \
  --runtime go125 \
  --verbosity error \
  --source . \
  --trigger-http \
  --allow-unauthenticated \
  --memory 128MiB \
  --set-env-vars "SLACK_BOT_TOKEN=$SLACK_BOT_TOKEN,SLACK_CHANNEL_ID=$SLACK_CHANNEL_ID,SLACK_SIGNING_SECRET=$SLACK_SIGNING_SECRET"

# Deploy interactivity function
gcloud functions deploy matter-interactivity \
  --project 'parafuzo-qa-infra' \
  --gen2 \
  --entry-point Interactivity \
  --region us-east1 \
  --runtime go125 \
  --verbosity error \
  --source . \
  --trigger-http \
  --allow-unauthenticated \
  --memory 128MiB \
  --set-env-vars "SLACK_BOT_TOKEN=$SLACK_BOT_TOKEN,SLACK_CHANNEL_ID=$SLACK_CHANNEL_ID,SLACK_SIGNING_SECRET=$SLACK_SIGNING_SECRET"

# Deploy reminder function (requires Pub/Sub topic to exist first)
gcloud functions deploy matter-reminder \
  --project 'parafuzo-qa-infra' \
  --gen2 \
  --entry-point Reminder \
  --region us-east1 \
  --runtime go125 \
  --verbosity error \
  --source . \
  --trigger-topic matter-reminder \
  --memory 128MiB \
  --set-env-vars "SLACK_BOT_TOKEN=$SLACK_BOT_TOKEN,SLACK_CHANNEL_ID=$SLACK_CHANNEL_ID,SLACK_SIGNING_SECRET=$SLACK_SIGNING_SECRET"
```

### Weekly Reminder Setup

The reminder function sends weekly DMs to all channel members encouraging them to give kudos. It requires Google Cloud Scheduler and Pub/Sub.

**Prerequisites:**
- Slack app must have the following permissions:
  - `channels:read` or `groups:read` - to list channel members
  - `im:write` - to send direct messages
  - `users:read` - to get user info (filter bots)
  - `chat:write` - to post messages (already required)

**Step 1: Create Pub/Sub Topic**
```bash
gcloud pubsub topics create matter-reminder \
  --project 'parafuzo-qa-infra'
```

**Step 2: Deploy Reminder Function** (see command above)

**Step 3: Create Cloud Scheduler Job**
```bash
# Weekly reminder - every Monday at 9 AM (America/Sao_Paulo timezone)
gcloud scheduler jobs create pubsub weekly-matter-reminder \
  --project 'parafuzo-qa-infra' \
  --location us-east1 \
  --schedule "0 10 * * 5" \
  --time-zone "America/Sao_Paulo" \
  --topic matter-reminder \
  --message-body '{"trigger":"weekly_reminder"}'
```

**Step 4: Test the Scheduler** (optional)
```bash
# Manually trigger the job to test
gcloud scheduler jobs run weekly-matter-reminder \
  --project 'parafuzo-qa-infra' \
  --location us-east1
```

**Managing the Reminder:**
```bash
# Pause reminders
gcloud scheduler jobs pause weekly-matter-reminder \
  --project 'parafuzo-qa-infra' \
  --location us-east1

# Resume reminders
gcloud scheduler jobs resume weekly-matter-reminder \
  --project 'parafuzo-qa-infra' \
  --location us-east1

# Update schedule
gcloud scheduler jobs update pubsub weekly-matter-reminder \
  --project 'parafuzo-qa-infra' \
  --location us-east1 \
  --schedule "0 10 * * 1"

# Delete scheduler job
gcloud scheduler jobs delete weekly-matter-reminder \
  --project 'parafuzo-qa-infra' \
  --location us-east1

# Delete Pub/Sub topic (only if removing feature entirely)
gcloud pubsub topics delete matter-reminder \
  --project 'parafuzo-qa-infra'
```

## Architecture

### Three-Function Design

The application uses **separate Cloud Functions** for different responsibilities:

**Entry Point Files (Root Directory):**
- `slash_command.go`: Exports `SlashCommand` function for Cloud Functions (package `function`)
- `interactivity.go`: Exports `Interactivity` function for Cloud Functions (package `function`)
- `reminder.go`: Exports `Reminder` function for Cloud Functions (package `function`)

These files act as adapters between Google Cloud Functions and the internal implementation.

1. **SlashCommand Function** (`functions/slashcommand/`)
   - Entry point: `SlashCommand`
   - Trigger: HTTP (Slack slash command)
   - Handles `/elogie` slash command
   - Opens the kudos modal via Slack's `views.open` API
   - Validates Slack signing secret

2. **Interactivity Function** (`functions/interactivity/`)
   - Entry point: `Interactivity`
   - Trigger: HTTP (Slack interactivity)
   - Handles all modal interactions
   - Routes to block actions handler (for dropdown changes)
   - Routes to view submission handler (for final submission)
   - Validates Slack signing secret

3. **Reminder Function** (`functions/reminder/`)
   - Entry point: `Reminder`
   - Trigger: Pub/Sub (Cloud Scheduler)
   - Sends weekly DM reminders to channel members
   - Lists channel members, filters bots/deleted users
   - Sends formatted Block Kit DMs with kudos encouragement

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
│   ├── formatting.go # Slack message formatting (Block Kit)
│   └── reminder.go  # GetChannelMembers(), SendReminderDM(), FormatReminderBlocks()
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

**Reminder Flow:**
1. Cloud Scheduler triggers weekly (cron schedule)
2. Publishes message to Pub/Sub topic `matter-reminder`
3. Reminder function receives CloudEvent
4. `services.GetChannelMembers()` lists all channel members with pagination
5. Filters out bots and deleted users using `GetUserInfo()`
6. `services.SendReminderDM()` sends DM to each member
7. DM contains Block Kit message with call-to-action button

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
