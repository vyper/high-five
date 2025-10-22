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

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/slack-go/slack"
	"github.com/vyper/my-matter/internal/config"
	"github.com/vyper/my-matter/internal/models"
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

// formatKudosAsBlocks creates a Slack Block Kit message for kudos
func formatKudosAsBlocks(senderID string, recipientIDs []string, kudoTypeEmoji string, kudoTypeText string, message string) []slack.Block {
	recipientsFormatted := formatUsersForSlack(recipientIDs)
	quotedMessage := formatAsSlackQuote(message)

	emojiTrue := true
	blocks := []slack.Block{
		slack.NewHeaderBlock(
			&slack.TextBlockObject{
				Type:  slack.PlainTextType,
				Text:  "🎉 Novo Elogio! 🎉",
				Emoji: &emojiTrue,
			},
		),

		slack.NewSectionBlock(
			nil,
			[]*slack.TextBlockObject{
				{
					Type: slack.MarkdownType,
					Text: fmt.Sprintf("*De:*\n<@%s>", senderID),
				},
				{
					Type: slack.MarkdownType,
					Text: fmt.Sprintf("*Para:*\n%s", recipientsFormatted),
				},
			},
			nil,
		),

		slack.NewDividerBlock(),

		slack.NewSectionBlock(
			&slack.TextBlockObject{
				Type: slack.MarkdownType,
				Text: fmt.Sprintf("%s *%s*", kudoTypeEmoji, kudoTypeText),
			},
			nil,
			nil,
		),

		slack.NewSectionBlock(
			&slack.TextBlockObject{
				Type: slack.MarkdownType,
				Text: quotedMessage,
			},
			nil,
			nil,
		),

		slack.NewDividerBlock(),

		slack.NewContextBlock(
			"",
			slack.NewTextBlockObject(slack.MarkdownType, "✨ _Continue fazendo a diferença!_ ✨", false, false),
		),
	}

	return blocks
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

				kudoTypeEmoji := ""
				kudoTypeText := kudoTypeFullText
				if idx := strings.Index(kudoTypeFullText, " "); idx > 0 {
					kudoTypeEmoji = kudoTypeFullText[:idx]  // ":zap:"
					kudoTypeText = kudoTypeFullText[idx+1:] // "Resolvedor(a) de Problemas"
				}

				blocks := formatKudosAsBlocks(i.User.ID, selectedUsers, kudoTypeEmoji, kudoTypeText, kudoMessage)

				usersString := formatUsersForSlack(selectedUsers)
				fallbackText := fmt.Sprintf(
					"%s elogiou %s: %s %s",
					fmt.Sprintf("<@%s>", i.User.ID),
					usersString,
					kudoTypeEmoji,
					kudoTypeText,
				)

				respChannelID, timestamp, err := config.SlackAPI.PostMessage(
					config.SlackChannelID,
					slack.MsgOptionBlocks(blocks...),
					slack.MsgOptionText(fallbackText, false),
				)
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

	kudoTypeIndex := -1
	descriptionBlockIndex := -1

	for i, block := range blocks {
		blockMap, ok := block.(map[string]interface{})
		if !ok {
			continue
		}

		if blockMap["block_id"] == "kudo_type" {
			kudoTypeIndex = i
		}

		if blockMap["block_id"] == "kudo_description" {
			descriptionBlockIndex = i
		}

		if blockMap["block_id"] == "kudo_message" {
			element, ok := blockMap["element"].(map[string]interface{})
			if ok && messageValue != "" {
				element["initial_value"] = messageValue
			}
		}
	}

	description := models.KudoDescriptions[selectedKudoType]
	if description == "" {
		description = "Tipo de elogio selecionado"
	}

	descriptionBlock := map[string]interface{}{
		"type":     "context",
		"block_id": "kudo_description",
		"elements": []interface{}{
			map[string]interface{}{
				"type": "mrkdwn",
				"text": fmt.Sprintf("💡 _%s_", description),
			},
		},
	}

	if descriptionBlockIndex == -1 && kudoTypeIndex != -1 {
		insertPosition := kudoTypeIndex + 1
		newBlocks := make([]interface{}, 0, len(blocks)+1)
		newBlocks = append(newBlocks, blocks[:insertPosition]...)
		newBlocks = append(newBlocks, descriptionBlock)
		newBlocks = append(newBlocks, blocks[insertPosition:]...)
		view["blocks"] = newBlocks
	} else if descriptionBlockIndex != -1 {
		blocks[descriptionBlockIndex] = descriptionBlock
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
