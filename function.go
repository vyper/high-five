package function

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
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
	SlackBotToken   string
	SlackChannelID  string
	SigningSecret   string
	SlackAPI        SlackClient
	HTTPClient      HTTPClient
}

var globalConfig *Config

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

				message := fmt.Sprintf(
					"Olá <@%s>, obrigado por elogiar: %v!\n\nVocê selecionou: %v e deixou a mensagem: \n\n> %v",
					i.User.ID,
					i.View.State.Values["kudo_users"]["kudo_users"].SelectedUsers,
					i.View.State.Values["kudo_type"]["kudo_type"].SelectedOption.Text.Text,
					i.View.State.Values["kudo_message"]["kudo_message"].Value,
				)

				respChannelID, timestamp, err := config.SlackAPI.PostMessage(config.SlackChannelID, slack.MsgOptionText(message, false))
				if err != nil {
					log.Printf("Error posting message: %v", err)
				} else {
					fmt.Printf("Message posted to channel %s at %s\n", respChannelID, timestamp)
				}
			} else {

				jsonBody := []byte(fmt.Sprintf(`{
	"trigger_id": "%s",
	"view": {
		"type": "modal",
		"submit": {
			"type": "plain_text",
			"text": "Elogiar!",
			"emoji": true
		},
		"close": {
			"type": "plain_text",
			"text": "Cancelar",
			"emoji": true
		},
		"title": {
			"type": "plain_text",
			"text": "Dê elogio",
			"emoji": true
		},
		"blocks": [
			{
				"type": "input",
				"block_id": "kudo_users",
				"label": {
					"type": "plain_text",
					"text": "Para",
					"emoji": true
				},
				"element": {
					"type": "multi_users_select",
					"action_id": "kudo_users",
					"placeholder": {
						"type": "plain_text",
						"text": "Selecione os elogiados",
						"emoji": true
					}
				}
			},
			{
				"type": "input",
				"block_id": "kudo_type",
				"label": {
					"type": "plain_text",
					"text": "Tipo",
					"emoji": true
				},
				"element": {
					"type": "static_select",
					"action_id": "kudo_type",
					"placeholder": {
						"type": "plain_text",
						"text": "Selecione o tipo de elogio",
						"emoji": true
					},
					"options": [
						{
							"text": {
								"type": "plain_text",
								"text": ":dart: Entrega Excepcional",
								"emoji": true
							},
							"value": "value-0"
						},
						{
							"text": {
								"type": "plain_text",
								"text": ":handshake: Espírito de Equipe",
								"emoji": true
							},
							"value": "value-1"
						},
						{
							"text": {
								"type": "plain_text",
								"text": ":bulb: Ideia Brilhante",
								"emoji": true
							},
							"value": "value-2"
						},
						{
							"text": {
								"type": "plain_text",
								"text": ":rocket: Acima e Além",
								"emoji": true
							},
							"value": "value-3"
						},
						{
							"text": {
								"type": "plain_text",
								"text": ":mortar_board: Mestre(a) em Ensinar",
								"emoji": true
							},
							"value": "value-4"
						},
						{
							"text": {
								"type": "plain_text",
								"text": ":zap: Resolvedor(a) de Problemas",
								"emoji": true
							},
							"value": "value-5"
						},
						{
							"text": {
								"type": "plain_text",
								"text": ":star2: Atitude Positiva",
								"emoji": true
							},
							"value": "value-6"
						},
						{
							"text": {
								"type": "plain_text",
								"text": ":seedling: Crescimento Contínuo",
								"emoji": true
							},
							"value": "value-7"
						},
						{
							"text": {
								"type": "plain_text",
								"text": ":tada: Conquista do Time",
								"emoji": true
							},
							"value": "value-8"
						},
						{
							"text": {
								"type": "plain_text",
								"text": ":muscle: Resiliência",
								"emoji": true
							},
							"value": "value-9"
						}
					]
				}
			},
			{
				"type": "input",
				"block_id": "kudo_message",
				"label": {
					"type": "plain_text",
					"text": "Mensagem",
					"emoji": true
				},
				"element": {
					"type": "plain_text_input",
					"action_id": "kudo_message",
					"multiline": true,
					"placeholder": {
						"type": "plain_text",
						"text": "Deixe uma mensagem junto",
						"emoji": true
					}
				},
				"optional": true
			}
		]
	}
}`, r.FormValue("trigger_id")))
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
