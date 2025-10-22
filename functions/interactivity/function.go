package interactivity

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/slack-go/slack"
	"github.com/vyper/my-matter/internal/config"
	"github.com/vyper/my-matter/internal/handlers"
	"github.com/vyper/my-matter/internal/templates"
)

var globalConfig *config.Config

func init() {
	functions.HTTP("Interactivity", handleInteractivity)

	cfg, err := config.LoadConfig(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	globalConfig = cfg
}

func handleInteractivity(w http.ResponseWriter, r *http.Request) {
	// Verify Slack signing secret
	_, err := slack.NewSecretsVerifier(r.Header, globalConfig.SigningSecret)
	if err != nil {
		log.Printf("Invalid Slack Signing Secret: %v", err)
		http.Error(w, "Invalid Slack Signing Secret", http.StatusUnauthorized)
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		log.Printf("Error parsing form: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Get payload
	payloadStr := r.FormValue("payload")
	if payloadStr == "" {
		log.Printf("Missing payload in interactivity request")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Parse interaction callback
	var callback slack.InteractionCallback
	if err := json.Unmarshal([]byte(payloadStr), &callback); err != nil {
		log.Printf("Invalid Slack Interaction Callback: %v", err)
		http.Error(w, "Invalid Slack Interaction Callback", http.StatusBadRequest)
		return
	}

	// Route to appropriate handler based on interaction type
	switch callback.Type {
	case slack.InteractionTypeBlockActions:
		handlers.HandleBlockActions(w, &callback, templates.GiveKudosViewTemplate, globalConfig)
	case slack.InteractionTypeViewSubmission:
		handlers.HandleViewSubmission(w, &callback, globalConfig)
	default:
		log.Printf("Unknown interaction type: %s", callback.Type)
		w.WriteHeader(http.StatusOK)
	}
}
