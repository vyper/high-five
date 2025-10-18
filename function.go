package function

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/slack-go/slack"
)

// HTTPClient interface for mocking HTTP calls
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// SlackClient interface for mocking Slack API calls
type SlackClient interface {
	PostMessage(channelID string, options ...slack.MsgOption) (string, string, error)
}

// Config holds the configuration for the function
type Config struct {
	SlackBotToken  string
	SlackChannelID string
	SigningSecret  string
	SlackAPI       SlackClient
	HTTPClient     HTTPClient
}

var globalConfig *Config

//go:embed screens/give-kudos.json
var giveKudosViewTemplate string

var kudoSuggestedMessages = map[string]string{
	"entrega-excepcional":     "Sua dedicação e capricho na entrega fizeram toda a diferença!",
	"espirito-de-equipe":      "Obrigado por estar sempre a disposição para ajudar o time!",
	"ideia-brilhante":         "Sua ideia trouxe uma perspectiva nova e valiosa para o problema!",
	"acima-e-alem":            "Você foi além das expectativas e isso não passou despercebido!",
	"mestre-em-ensinar":       "Obrigado por compartilhar seu conhecimento e ajudar o time a crescer!",
	"resolvedor-de-problemas": "Sua habilidade de resolver problemas salvou o dia!",
	"atitude-positiva":        "Sua energia positiva contagia e motiva todo o time!",
	"crescimento-continuo":    "Inspirador ver sua dedicação em sempre aprender e evoluir!",
	"conquista-do-time":       "Parabéns pela conquista! Sucesso de todos nós!",
	"resiliencia":             "Sua persistência diante dos desafios é admirável!",
}

func init() {
	functions.HTTP("GiveKudos", giveKudos)

	config, err := LoadConfig(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	globalConfig = config
}

// LoadConfig loads configuration from environment variables
func LoadConfig(getenv func(string) string) (*Config, error) {
	slackBotToken := getenv("SLACK_BOT_TOKEN")
	if slackBotToken == "" {
		return nil, fmt.Errorf("SLACK_BOT_TOKEN environment variable is required")
	}

	slackChannelID := getenv("SLACK_CHANNEL_ID")
	if slackChannelID == "" {
		return nil, fmt.Errorf("SLACK_CHANNEL_ID environment variable is required")
	}

	signingSecret := getenv("SLACK_SIGNING_SECRET")
	if signingSecret == "" {
		return nil, fmt.Errorf("SLACK_SIGNING_SECRET environment variable is required")
	}

	return &Config{
		SlackBotToken:  slackBotToken,
		SlackChannelID: slackChannelID,
		SigningSecret:  signingSecret,
		SlackAPI:       slack.New(slackBotToken, slack.OptionDebug(true)),
		HTTPClient:     &http.Client{Timeout: time.Second * 10},
	}, nil
}

// formatUsersForSlack formats a list of user IDs as Slack mentions
// Example: ["U123", "U456"] -> "<@U123>, <@U456>"
func formatUsersForSlack(userIDs []string) string {
	var usersFormatted []string
	for _, userID := range userIDs {
		usersFormatted = append(usersFormatted, fmt.Sprintf("<@%s>", userID))
	}
	return strings.Join(usersFormatted, ", ")
}

// formatAsSlackQuote formats a message as a Slack quote block
// Adds "> " prefix to each line to maintain quote formatting
func formatAsSlackQuote(message string) string {
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

// giveKudos is an HTTP Cloud Function.
func giveKudos(w http.ResponseWriter, r *http.Request) {
	handleKudos(w, r, globalConfig)
}

// handleKudos processes the kudos request with injectable config
func handleKudos(w http.ResponseWriter, r *http.Request, config *Config) {
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
				var usersFormatted []string
				for _, userID := range i.View.State.Values["kudo_users"]["kudo_users"].SelectedUsers {
					usersFormatted = append(usersFormatted, fmt.Sprintf("<@%s>", userID))
				}
				usersString := strings.Join(usersFormatted, ", ")

				kudoMessage := i.View.State.Values["kudo_message"]["kudo_message"].Value
				quotedMessage := formatAsSlackQuote(kudoMessage)

				message := fmt.Sprintf(
					"Olá <@%s>, obrigado por elogiar: %v!\n\nVocê selecionou: %v e deixou a mensagem:\n\n%v",
					i.User.ID,
					usersString,
					i.View.State.Values["kudo_type"]["kudo_type"].SelectedOption.Text.Text,
					quotedMessage,
				)

				respChannelID, timestamp, err := config.SlackAPI.PostMessage(config.SlackChannelID, slack.MsgOptionText(message, false))
				if err != nil {
					log.Printf("Error posting message: %v", err)
				} else {
					fmt.Printf("Message posted to channel %s at %s\n", respChannelID, timestamp)
				}
			} else {
				var viewRequest map[string]interface{}
				if err := json.Unmarshal([]byte(giveKudosViewTemplate), &viewRequest); err != nil {
					log.Printf("Error parsing view template: %v", err)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}

				viewRequest["trigger_id"] = r.FormValue("trigger_id")

				jsonBody, err := json.Marshal(viewRequest)
				if err != nil {
					log.Printf("Error marshaling view request: %v", err)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}
				bodyReader := bytes.NewReader(jsonBody)

				req, err := http.NewRequest("POST", "https://slack.com/api/views.open", bodyReader)
				if err != nil {
					fmt.Printf("Error creating request: %v\n", err)
					return
				}

				req.Header.Add("Content-Type", "application/json")
				req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", config.SlackBotToken))

				resp, err := config.HTTPClient.Do(req)
				if err != nil {
					fmt.Printf("Error making POST request: %v\n", err)
					return
				}
				defer resp.Body.Close()

				body, err := io.ReadAll(resp.Body)
				if err != nil {
					fmt.Printf("Error reading response body: %v\n", err)
					return
				}

				fmt.Printf("Response Status: %s\n", resp.Status)
				fmt.Printf("Response Body:\n%s\n", body)
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
func handleBlockActions(w http.ResponseWriter, callback *slack.InteractionCallback, config *Config) {
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
				if msg, ok := kudoSuggestedMessages[action.SelectedOption.Value]; ok {
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
func updateView(viewID, hash, selectedKudoType, messageValue string, config *Config) error {
	// Parse the view template
	var viewData map[string]interface{}
	if err := json.Unmarshal([]byte(giveKudosViewTemplate), &viewData); err != nil {
		return fmt.Errorf("error parsing view template: %w", err)
	}

	// Extract and update the view blocks
	view, ok := viewData["view"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid view structure in template")
	}

	blocks, ok := view["blocks"].([]interface{})
	if !ok {
		return fmt.Errorf("invalid blocks structure in template")
	}

	// Update the message field value
	for _, block := range blocks {
		blockMap, ok := block.(map[string]interface{})
		if !ok {
			continue
		}

		if blockMap["block_id"] == "kudo_message" {
			element, ok := blockMap["element"].(map[string]interface{})
			if ok && messageValue != "" {
				element["initial_value"] = messageValue
			}
		}
	}

	// Prepare the views.update request
	updateRequest := map[string]interface{}{
		"view_id": viewID,
		"hash":    hash,
		"view":    view,
	}

	jsonBody, err := json.Marshal(updateRequest)
	if err != nil {
		return fmt.Errorf("error marshaling update request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://slack.com/api/views.update", bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.SlackBotToken))

	resp, err := config.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("error making views.update request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response: %w", err)
	}

	// Parse response to check for errors
	var slackResp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(body, &slackResp); err != nil {
		return fmt.Errorf("error parsing response: %w", err)
	}

	if !slackResp.OK {
		return fmt.Errorf("slack API error: %s", slackResp.Error)
	}

	log.Printf("View updated successfully for kudo type: %s", selectedKudoType)
	return nil
}
