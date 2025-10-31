package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/slack-go/slack"
	"github.com/vyper/my-matter/internal/config"
	"github.com/vyper/my-matter/internal/models"
	"github.com/vyper/my-matter/internal/services"
)

// HandleViewSubmission processes modal submission and posts the kudos message
func HandleViewSubmission(w http.ResponseWriter, callback *slack.InteractionCallback, cfg *config.Config) {
	// Check if State is properly initialized
	if callback.View.State == nil || callback.View.State.Values == nil {
		log.Printf("Invalid view state in submission")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Extract values from modal submission
	selectedUsers := callback.View.State.Values["kudo_users"]["kudo_users"].SelectedUsers
	kudoMessage := callback.View.State.Values["kudo_message"]["kudo_message"].Value
	kudoTypeFullText := callback.View.State.Values["kudo_type"]["kudo_type"].SelectedOption.Text.Text
	kudoTypeValue := callback.View.State.Values["kudo_type"]["kudo_type"].SelectedOption.Value

	var kudoTypeEmoji, kudoTypeText string

	// Handle custom kudo type
	if kudoTypeValue == "custom" {
		// Extract custom kudo description from the input field
		customDescription := ""
		if callback.View.State != nil && callback.View.State.Values != nil {
			if descBlock, ok := callback.View.State.Values["kudo_description"]; ok {
				if descValue, ok := descBlock["kudo_description"]; ok {
					customDescription = strings.TrimSpace(descValue.Value)
				}
			}
		}

		// Validate custom description
		errors := make(map[string]string)
		if customDescription == "" {
			errors["kudo_description"] = "Por favor, preencha o nome do tipo de elogio"
		} else if len(customDescription) > 150 {
			errors["kudo_description"] = "Nome do tipo de elogio muito longo (máximo 150 caracteres)"
		}

		// Validate message is required for custom type
		if kudoMessage == "" {
			errors["kudo_message"] = "A mensagem é obrigatória para elogios personalizados"
		}

		// Return validation errors if any
		if len(errors) > 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"response_action": "errors",
				"errors":          errors,
			})
			return
		}

		// Use fixed emoji and custom description
		kudoTypeEmoji = "✏️"
		kudoTypeText = customDescription
	} else {
		// Regular predefined kudo type
		// If the user didn't interact with the message field, use the suggested message
		if kudoMessage == "" {
			if suggestedMsg, ok := models.KudoSuggestedMessages[kudoTypeValue]; ok {
				kudoMessage = suggestedMsg
			}
		}

		// Parse emoji and text from kudo type
		kudoTypeEmoji, kudoTypeText = services.ParseKudoTypeText(kudoTypeFullText)
	}

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
