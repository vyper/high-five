package handlers

import (
	"log"
	"net/http"

	"github.com/slack-go/slack"
	"github.com/vyper/my-matter/internal/config"
	"github.com/vyper/my-matter/internal/models"
	"github.com/vyper/my-matter/internal/services"
)

// HandleBlockActions processes block_actions interactions for dynamic modal updates
func HandleBlockActions(w http.ResponseWriter, callback *slack.InteractionCallback, viewTemplate string, cfg *config.Config) {
	// Check if this is a kudo_type selection
	for _, action := range callback.ActionCallback.BlockActions {
		if action.ActionID == "kudo_type" && action.SelectedOption.Value != "" {
			// Get current message value (preserve if user already typed something)
			currentMessage := ""
			if callback.View.State != nil {
				if messageBlock, ok := callback.View.State.Values["kudo_message"]; ok {
					if messageValue, ok := messageBlock["kudo_message"]; ok {
						currentMessage = messageValue.Value
					}
				}
			}

			// Only suggest message if field is empty (preserve user input)
			// For custom type, never suggest a message
			suggestedMessage := ""
			if action.SelectedOption.Value == "custom" {
				// For custom type, preserve current message but don't suggest anything
				suggestedMessage = currentMessage
			} else if currentMessage == "" {
				// For predefined types, suggest message only if empty
				if msg, ok := models.KudoSuggestedMessages[action.SelectedOption.Value]; ok {
					suggestedMessage = msg
				}
			} else {
				// Preserve user's current message
				suggestedMessage = currentMessage
			}

			// Update the view with the suggested message
			err := services.UpdateModal(
				callback.View.ID,
				callback.View.Hash,
				action.SelectedOption.Value,
				suggestedMessage,
				viewTemplate,
				cfg,
			)
			if err != nil {
				log.Printf("Error updating view: %v", err)
				http.Error(w, "Error updating modal", http.StatusInternalServerError)
				return
			}

			// Acknowledge the action
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	// If no matching action found, just acknowledge
	w.WriteHeader(http.StatusOK)
}
