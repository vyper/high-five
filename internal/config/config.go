package config

import (
	"fmt"
	"net/http"
	"time"

	"github.com/slack-go/slack"
)

// HTTPClient interface for mocking HTTP calls
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// SlackClient interface for mocking Slack API calls
type SlackClient interface {
	PostMessage(channelID string, options ...slack.MsgOption) (string, string, error)
	InviteUsersToConversation(channelID string, users ...string) (*slack.Channel, error)
	GetUsersInConversation(params *slack.GetUsersInConversationParameters) ([]string, string, error)
	GetUserInfo(user string) (*slack.User, error)
}

// Config holds the configuration for the function
type Config struct {
	SlackBotToken  string
	SlackChannelID string
	SigningSecret  string
	SlackAPI       SlackClient
	HTTPClient     HTTPClient
}

// LoadConfig loads configuration from environment variables
func LoadConfig(getenv func(string) string) (*Config, error) {
	slackBotToken := getenv("SLACK_BOT_TOKEN")
	if slackBotToken == "" {
		return nil, fmt.Errorf("SLACK_BOT_TOKEN environment variable is required")
	}

	slackChannelID := getenv("SLACK_CHANNEL_ID")
	if slackChannelID == "" {
		return nil, fmt.Errorf("SLACK_CHANNEL_ID environment variable is required")
	}

	signingSecret := getenv("SLACK_SIGNING_SECRET")
	if signingSecret == "" {
		return nil, fmt.Errorf("SLACK_SIGNING_SECRET environment variable is required")
	}

	return &Config{
		SlackBotToken:  slackBotToken,
		SlackChannelID: slackChannelID,
		SigningSecret:  signingSecret,
		SlackAPI:       slack.New(slackBotToken, slack.OptionDebug(true)),
		HTTPClient:     &http.Client{Timeout: time.Second * 10},
	}, nil
}
