# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This is a Go application that implements a Google Cloud Functions Gen2 service for handling Slack kudos/compliments functionality. The function receives Slack interactions, opens a modal for users to give kudos, and posts the kudos message to a configured Slack channel.

## Commands

### Development
```bash
# Install dependencies
go mod tidy

# Run locally (starts server on localhost:8080)
go run cmd/main.go

# Run locally on specific interface (avoids firewall warnings)
LOCAL_ONLY=true go run cmd/main.go

# Run with custom port
PORT=3000 go run cmd/main.go
```

### Running Tests

```bash
# Run all tests
SLACK_BOT_TOKEN=xoxb-test SLACK_CHANNEL_ID=C123 SLACK_SIGNING_SECRET=secret go test -v

# Generate coverage report
SLACK_BOT_TOKEN=xoxb-test SLACK_CHANNEL_ID=C123 SLACK_SIGNING_SECRET=secret go test -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html

# Run benchmarks
SLACK_BOT_TOKEN=xoxb-test SLACK_CHANNEL_ID=C123 SLACK_SIGNING_SECRET=secret go test -bench=. -benchmem
```

### Deployment
```bash
# Deploy to Google Cloud Functions Gen2
gcloud beta functions deploy matter-give-kudos \
  --project 'parafuzo-qa-infra' \
  --gen2 \
  --entry-point GiveKudos \
  --region us-east1 \
  --runtime go125 \
  --verbosity error \
  --source . \
  --trigger-http \
  --allow-unauthenticated \
  --memory 128MiB \
  --set-env-vars "SLACK_BOT_TOKEN=$SLACK_BOT_TOKEN,SLACK_CHANNEL_ID=$SLACK_CHANNEL_ID,SLACK_SIGNING_SECRET=$SLACK_SIGNING_SECRET"
```

## Architecture

### Entry Points
- **function.go**: Main Cloud Function implementation with the `GiveKudos` HTTP handler
  - Entry point registered in `init()` as `GiveKudos`
  - All Slack interaction logic lives here
- **cmd/main.go**: Local development server that wraps the function using Google's functions framework
  - Imports the function package to register handlers
  - Configurable via PORT and LOCAL_ONLY environment variables

### Request Flow
1. **Slack Command Trigger**: User initiates kudos via Slack slash command
   - Request comes with `trigger_id` in form data
   - Function verifies Slack signing secret using `slack.NewSecretsVerifier`
   - Opens modal using Slack API `views.open` with embedded JSON view definition

2. **Modal Submission**: User fills out and submits the modal
   - Slack sends interaction callback as `application/x-www-form-urlencoded` with `payload` field
   - Payload contains `InteractionCallback` with user selections:
     - `kudo_users`: Multi-user select (who to praise)
     - `kudo_type`: Static select (type of kudos)
     - `kudo_message`: Text input (optional message)
   - Function posts formatted message to configured channel using `slackApi.PostMessage`

### Configuration
Required environment variables loaded in `init()`:
- `SLACK_BOT_TOKEN`: Bot token for Slack API authentication
- `SLACK_CHANNEL_ID`: Target channel for kudos messages
- `SLACK_SIGNING_SECRET`: Used to verify requests come from Slack

### Dependencies
- `github.com/GoogleCloudPlatform/functions-framework-go`: Functions framework for local dev and deployment
- `github.com/slack-go/slack`: Official Slack SDK for Go
- Go 1.25 runtime

### File Structure
- `function.go`: Core function logic (173 LOC includes inline modal JSON)
- `cmd/main.go`: Local development wrapper
- `requests/slack.http`: HTTP request examples for testing
- `screens/give-kudos.json`: Modal view definition (reference/documentation)

### Security
- All requests validated using Slack signing secret before processing
- Returns 401 Unauthorized if verification fails
- Signing secret verification happens at function.go:49
