package function

import (
	_ "embed"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/slack-go/slack"
	"github.com/vyper/my-matter/internal/config"
	"github.com/vyper/my-matter/internal/handlers"
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
	_, err := slack.NewSecretsVerifier(r.Header, config.SigningSecret)
	if err != nil {
		log.Printf("Invalid Slack Signin Secret: %v", err)
		http.Error(w, "Invalid Slack Signin Secret", http.StatusUnauthorized)
		return
	}

	// Parse form data
	if r.Method == http.MethodPost && r.Header.Get("Content-Type") == "application/x-www-form-urlencoded" {
		if err := r.ParseForm(); err != nil {
			log.Printf("Error parsing form: %v", err)
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		// Route based on payload presence
		if payloadStr := r.FormValue("payload"); payloadStr != "" {
			// This is an interaction callback (modal interaction)
			var callback slack.InteractionCallback
			if err := json.Unmarshal([]byte(payloadStr), &callback); err != nil {
				log.Printf("Invalid Slack Interaction Callback: %v", err)
				http.Error(w, "Invalid Slack Interaction Callback", http.StatusBadRequest)
				return
			}

			// Route to appropriate handler based on interaction type
			switch callback.Type {
			case slack.InteractionTypeBlockActions:
				handlers.HandleBlockActions(w, &callback, giveKudosViewTemplate, config)
			case slack.InteractionTypeViewSubmission:
				handlers.HandleViewSubmission(w, &callback, config)
			default:
				log.Printf("Unknown interaction type: %s", callback.Type)
				w.WriteHeader(http.StatusOK)
			}
		} else {
			// This is a slash command
			handlers.HandleSlashCommand(w, r, giveKudosViewTemplate, config)
		}
	} else {
		log.Printf("Unsupported request: Method=%s, Content-Type=%s", r.Method, r.Header.Get("Content-Type"))
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}
