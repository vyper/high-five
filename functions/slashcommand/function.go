package slashcommand

import (
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
	functions.HTTP("SlashCommand", handleSlashCommand)

	cfg, err := config.LoadConfig(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	globalConfig = cfg
}

func handleSlashCommand(w http.ResponseWriter, r *http.Request) {
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

	// Handle slash command
	handlers.HandleSlashCommand(w, r, templates.GiveKudosViewTemplate, globalConfig)
}

// HandleSlashCommand is the exported function for the Cloud Function entry point
func HandleSlashCommand(w http.ResponseWriter, r *http.Request) {
	handleSlashCommand(w, r)
}
