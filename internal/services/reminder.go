package services

import (
	"fmt"
	"log"

	"github.com/slack-go/slack"
	"github.com/vyper/my-matter/internal/config"
)

// GetChannelMembers retrieves all active members from a Slack channel
// It handles pagination and filters out bots and deleted users
func GetChannelMembers(client config.SlackClient, channelID string) ([]string, error) {
	var allMembers []string
	cursor := ""

	for {
		params := &slack.GetUsersInConversationParameters{
			ChannelID: channelID,
			Cursor:    cursor,
			Limit:     200, // Maximum allowed by Slack API
		}

		members, nextCursor, err := client.GetUsersInConversation(params)
		if err != nil {
			return nil, fmt.Errorf("failed to get channel members: %w", err)
		}

		// Filter out bots and deleted users
		for _, userID := range members {
			userInfo, err := client.GetUserInfo(userID)
			if err != nil {
				log.Printf("Warning: could not get user info for %s: %v", userID, err)
				continue
			}

			// Skip bots and deleted users
			if userInfo.IsBot || userInfo.Deleted {
				continue
			}

			allMembers = append(allMembers, userID)
		}

		// Check if there are more pages
		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	return allMembers, nil
}

// SendReminderDM sends a kudos reminder DM to a specific user
func SendReminderDM(client config.SlackClient, userID string) error {
	blocks := FormatReminderBlocks()

	_, _, err := client.PostMessage(
		userID,
		slack.MsgOptionBlocks(blocks...),
		slack.MsgOptionText("Lembrete semanal: envie um elogio para seus colegas!", false),
	)

	if err != nil {
		return fmt.Errorf("failed to send DM to user %s: %w", userID, err)
	}

	return nil
}

// FormatReminderBlocks creates the Block Kit structure for the reminder message
func FormatReminderBlocks() []slack.Block {
	return []slack.Block{
		// Header
		slack.NewHeaderBlock(
			&slack.TextBlockObject{
				Type: slack.PlainTextType,
				Text: "👋 Lembrete Semanal de Kudos",
			},
		),

		// Main message section
		slack.NewSectionBlock(
			&slack.TextBlockObject{
				Type: slack.MarkdownType,
				Text: "Esta semana você reconheceu algum colega pelo trabalho excepcional?\n\nUse `/elogie` para enviar um elogio e valorizar sua equipe!",
			},
			nil,
			nil,
		),

		// Call-to-action button
		slack.NewActionBlock(
			"reminder_actions",
			slack.NewButtonBlockElement(
				"open_kudos_modal",
				"open_modal",
				&slack.TextBlockObject{
					Type: slack.PlainTextType,
					Text: "📝 Enviar Elogio Agora",
				},
			).WithStyle(slack.StylePrimary),
		),

		// Divider
		slack.NewDividerBlock(),

		// Helpful tip in context
		slack.NewContextBlock(
			"reminder_context",
			&slack.TextBlockObject{
				Type: slack.MarkdownType,
				Text: "💡 *Dica:* Elogios específicos e detalhados têm mais impacto!",
			},
		),
	}
}
