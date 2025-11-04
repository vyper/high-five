package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/slack-go/slack"
	"github.com/vyper/my-matter/internal/config"
	"github.com/vyper/my-matter/internal/services"
)

// HandleReminderButton handles the button click from reminder DMs
// Opens the kudos modal when user clicks "Enviar Elogio Agora"
func HandleReminderButton(w http.ResponseWriter, callback *slack.InteractionCallback, viewTemplate string, cfg *config.Config) {
	// Extract trigger_id from the callback
	triggerID := callback.TriggerID
	if triggerID == "" {
		log.Printf("Missing trigger_id in reminder button interaction")
		http.Error(w, "Missing trigger_id", http.StatusBadRequest)
		return
	}

	// Open the modal using the same service as slash command
	err := services.OpenModal(triggerID, viewTemplate, cfg)
	if err != nil {
		log.Printf("Error opening modal from reminder button: %v", err)
		// Return a visible error to the user
		errorResponse := map[string]interface{}{
			"response_action": "errors",
			"errors": map[string]string{
				"reminder_actions": "Não foi possível abrir o modal. Tente usar o comando /elogie",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	// Return success - modal opened
	w.WriteHeader(http.StatusOK)
}
