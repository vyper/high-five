package services

import (
	"fmt"
	"strings"

	"github.com/slack-go/slack"
)

// FormatUsersForSlack formats a list of user IDs as Slack mentions
// Example: ["U123", "U456"] -> "<@U123>, <@U456>"
func FormatUsersForSlack(userIDs []string) string {
	var usersFormatted []string
	for _, userID := range userIDs {
		usersFormatted = append(usersFormatted, fmt.Sprintf("<@%s>", userID))
	}
	return strings.Join(usersFormatted, ", ")
}

// FormatAsSlackQuote formats a message as a Slack quote block
// Adds "> " prefix to each line to maintain quote formatting
func FormatAsSlackQuote(message string) string {
	if message == "" {
		return ""
	}
	lines := strings.Split(message, "\n")
	var quotedLines []string
	for _, line := range lines {
		quotedLines = append(quotedLines, "> "+line)
	}
	return strings.Join(quotedLines, "\n")
}

// FormatKudosAsBlocks creates a Slack Block Kit message for kudos
func FormatKudosAsBlocks(senderID string, recipientIDs []string, kudoTypeEmoji string, kudoTypeText string, message string) []slack.Block {
	recipientsFormatted := FormatUsersForSlack(recipientIDs)
	quotedMessage := FormatAsSlackQuote(message)

	emojiTrue := true
	blocks := []slack.Block{
		slack.NewHeaderBlock(
			&slack.TextBlockObject{
				Type:  slack.PlainTextType,
				Text:  "🎉 Novo Elogio! 🎉",
				Emoji: &emojiTrue,
			},
		),

		slack.NewSectionBlock(
			nil,
			[]*slack.TextBlockObject{
				{
					Type: slack.MarkdownType,
					Text: fmt.Sprintf("*De:*\n<@%s>", senderID),
				},
				{
					Type: slack.MarkdownType,
					Text: fmt.Sprintf("*Para:*\n%s", recipientsFormatted),
				},
			},
			nil,
		),

		slack.NewDividerBlock(),

		slack.NewSectionBlock(
			&slack.TextBlockObject{
				Type: slack.MarkdownType,
				Text: fmt.Sprintf("%s *%s*", kudoTypeEmoji, kudoTypeText),
			},
			nil,
			nil,
		),

		slack.NewSectionBlock(
			&slack.TextBlockObject{
				Type: slack.MarkdownType,
				Text: quotedMessage,
			},
			nil,
			nil,
		),

		slack.NewDividerBlock(),

		slack.NewContextBlock(
			"",
			slack.NewTextBlockObject(slack.MarkdownType, "✨ _Continue fazendo a diferença!_ ✨", false, false),
		),
	}

	return blocks
}
