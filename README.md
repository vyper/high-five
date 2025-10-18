# Matter Give Kudos - Cloud Function

A Go application for Google Cloud Functions Gen2 that handles Slack kudos/compliments functionality.

## Setup

1. **Install dependencies:**
   
```bash
go mod tidy
```

2. **Set up environment variables:**

```bash
export SLACK_BOT_TOKEN=xoxb-your-slack-bot-token-here
export SLACK_CHANNEL_ID=C09CDF3RQJG
export SLACK_SIGNING_SECRET=your-slack-signing-secret-here
```

3. **Deploy to Google Cloud Functions:**

```bash
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

## Local Development

```bash
go run main.go
```

The function will be available at `http://localhost:8080`

### Running tests

```bash
# Run all tests
SLACK_BOT_TOKEN=xoxb-test SLACK_CHANNEL_ID=C123 SLACK_SIGNING_SECRET=secret go test -v

# Generate coverage report
SLACK_BOT_TOKEN=xoxb-test SLACK_CHANNEL_ID=C123 SLACK_SIGNING_SECRET=secret go test -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html

# Run benchmarks
SLACK_BOT_TOKEN=xoxb-test SLACK_CHANNEL_ID=C123 SLACK_SIGNING_SECRET=secret go test -bench=. -benchmem
```
