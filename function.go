package function

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/slack-go/slack"
	"github.com/vyper/my-matter/internal/config"
	"github.com/vyper/my-matter/internal/models"
	"github.com/vyper/my-matter/internal/services"
)

var globalConfig *config.Config

//go:embed screens/give-kudos.json
var giveKudosViewTemplate string

func init() {
	functions.HTTP("GiveKudos", giveKudos)

	cfg, err := config.LoadConfig(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	globalConfig = cfg
}


// giveKudos is an HTTP Cloud Function.
func giveKudos(w http.ResponseWriter, r *http.Request) {
	handleKudos(w, r, globalConfig)
}

// handleKudos processes the kudos request with injectable config
func handleKudos(w http.ResponseWriter, r *http.Request, config *config.Config) {
	fmt.Printf("Method: %s\n", r.Method)
	fmt.Printf("Content-Type: %s\n", r.Header.Get("Content-Type"))

	_, err := slack.NewSecretsVerifier(r.Header, config.SigningSecret)
	if err != nil {
		log.Printf("Invalid Slack Signin Secret: %v", err)
		http.Error(w, "Invalid Slack Signin Secret", http.StatusUnauthorized)
		return
	}

	if r.Method == http.MethodPost && r.Header.Get("Content-Type") == "application/x-www-form-urlencoded" {
		if err := r.ParseForm(); err != nil {
			log.Printf("error parsing application/x-www-form-urlencoded: %v", err)
		} else {
			if payloadStr := r.FormValue("payload"); payloadStr != "" {
				var i slack.InteractionCallback
				err = json.Unmarshal([]byte(r.FormValue("payload")), &i)
				if err != nil {
					log.Printf("Invalid Slack Interaction Callback: %v", err)
					http.Error(w, "Invalid Slack Interaction Callback", http.StatusUnauthorized)
					return
				}

				// Handle block_actions for dynamic modal updates
				if i.Type == slack.InteractionTypeBlockActions {
					handleBlockActions(w, &i, config)
					return
				}

				// Handle view_submission for final kudos message
				selectedUsers := i.View.State.Values["kudo_users"]["kudo_users"].SelectedUsers
				kudoMessage := i.View.State.Values["kudo_message"]["kudo_message"].Value
				kudoTypeFullText := i.View.State.Values["kudo_type"]["kudo_type"].SelectedOption.Text.Text
				kudoTypeValue := i.View.State.Values["kudo_type"]["kudo_type"].SelectedOption.Value

				// If the user didn't interact with the message field, use the suggested message
				if kudoMessage == "" {
					if suggestedMsg, ok := models.KudoSuggestedMessages[kudoTypeValue]; ok {
						kudoMessage = suggestedMsg
					}
				}

				kudoTypeEmoji, kudoTypeText := services.ParseKudoTypeText(kudoTypeFullText)
				if err := services.PostKudos(i.User.ID, selectedUsers, kudoTypeEmoji, kudoTypeText, kudoMessage, config); err != nil {
					log.Printf("Error posting kudos: %v", err)
				}
			} else {
				if err := services.OpenModal(r.FormValue("trigger_id"), giveKudosViewTemplate, config); err != nil {
					log.Printf("Error opening modal: %v", err)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}
			}
		}
	} else {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			fmt.Printf("Error reading request body: %v\n", err)
		} else {
			fmt.Printf("Request Body: %s\n", string(body))
		}

	}
}

// handleBlockActions processes block_actions interactions for dynamic modal updates
func handleBlockActions(w http.ResponseWriter, callback *slack.InteractionCallback, config *config.Config) {
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
			suggestedMessage := ""
			if currentMessage == "" {
				if msg, ok := models.KudoSuggestedMessages[action.SelectedOption.Value]; ok {
					suggestedMessage = msg
				}
			} else {
				suggestedMessage = currentMessage
			}

			// Update the view with the suggested message
			err := updateView(callback.View.ID, callback.View.Hash, action.SelectedOption.Value, suggestedMessage, config)
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

// updateView calls Slack's views.update API to dynamically update the modal
func updateView(viewID, hash, selectedKudoType, messageValue string, config *config.Config) error {
	return services.UpdateModal(viewID, hash, selectedKudoType, messageValue, giveKudosViewTemplate, config)
}
