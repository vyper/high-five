package services

import (
	"fmt"
	"log"
	"strings"

	"github.com/slack-go/slack"
	"github.com/vyper/my-matter/internal/config"
)

// PostKudos sends a kudos message to Slack channel
func PostKudos(senderID string, recipientIDs []string, kudoTypeEmoji, kudoTypeText, message string, cfg *config.Config) error {
	blocks := FormatKudosAsBlocks(senderID, recipientIDs, kudoTypeEmoji, kudoTypeText, message)

	usersString := FormatUsersForSlack(recipientIDs)
	fallbackText := fmt.Sprintf(
		"%s elogiou %s: %s %s",
		fmt.Sprintf("<@%s>", senderID),
		usersString,
		kudoTypeEmoji,
		kudoTypeText,
	)

	respChannelID, timestamp, err := cfg.SlackAPI.PostMessage(
		cfg.SlackChannelID,
		slack.MsgOptionBlocks(blocks...),
		slack.MsgOptionText(fallbackText, false),
	)
	if err != nil {
		return fmt.Errorf("error posting message: %w", err)
	}

	log.Printf("Message posted to channel %s at %s", respChannelID, timestamp)
	return nil
}

// ParseKudoTypeText splits emoji and text from kudo type full text
// Example: ":zap: Resolvedor(a) de Problemas" -> (":zap:", "Resolvedor(a) de Problemas")
func ParseKudoTypeText(kudoTypeFullText string) (emoji, text string) {
	if idx := strings.Index(kudoTypeFullText, " "); idx > 0 {
		return kudoTypeFullText[:idx], kudoTypeFullText[idx+1:]
	}
	return "", kudoTypeFullText
}
