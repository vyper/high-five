package handlers

import (
	"log"
	"net/http"

	"github.com/slack-go/slack"
	"github.com/vyper/my-matter/internal/config"
	"github.com/vyper/my-matter/internal/models"
	"github.com/vyper/my-matter/internal/services"
)

// HandleViewSubmission processes modal submission and posts the kudos message
func HandleViewSubmission(w http.ResponseWriter, callback *slack.InteractionCallback, cfg *config.Config) {
	// Extract values from modal submission
	selectedUsers := callback.View.State.Values["kudo_users"]["kudo_users"].SelectedUsers
	kudoMessage := callback.View.State.Values["kudo_message"]["kudo_message"].Value
	kudoTypeFullText := callback.View.State.Values["kudo_type"]["kudo_type"].SelectedOption.Text.Text
	kudoTypeValue := callback.View.State.Values["kudo_type"]["kudo_type"].SelectedOption.Value

	// If the user didn't interact with the message field, use the suggested message
	if kudoMessage == "" {
		if suggestedMsg, ok := models.KudoSuggestedMessages[kudoTypeValue]; ok {
			kudoMessage = suggestedMsg
		}
	}

	// Parse emoji and text from kudo type
	kudoTypeEmoji, kudoTypeText := services.ParseKudoTypeText(kudoTypeFullText)

	// Post the kudos message to Slack
	err := services.PostKudos(
		callback.User.ID,
		selectedUsers,
		kudoTypeEmoji,
		kudoTypeText,
		kudoMessage,
		cfg,
	)
	if err != nil {
		log.Printf("Error posting kudos: %v", err)
		// Note: We don't return error to user as modal already closed
	}

	// Acknowledge submission (modal will close)
	w.WriteHeader(http.StatusOK)
}
