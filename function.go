package function

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"io"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/slack-go/slack"
)

var slackBotToken string
var signingSecret string

func init() {
	functions.HTTP("GiveKudos", giveKudos)

	slackBotToken = os.Getenv("SLACK_BOT_TOKEN")
	if slackBotToken == "" {
		log.Fatal("SLACK_BOT_TOKEN environment variable is required")
	}

	signingSecret = os.Getenv("SLACK_SIGNING_SECRET")
	if signingSecret == "" {
		log.Fatal("SLACK_SIGNING_SECRET environment variable is required")
	}
}

// giveKudos is an HTTP Cloud Function.
func giveKudos(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Method: %s\n", r.Method)
	fmt.Printf("Content-Type: %s\n", r.Header.Get("Content-Type"))

	_, err := slack.NewSecretsVerifier(r.Header, signingSecret)
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
				fmt.Printf("Payload: %s\n", payloadStr)
			} else {
				client := &http.Client{
					Timeout: time.Second * 10,
				}

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
				"label": {
					"type": "plain_text",
					"text": "Para",
					"emoji": true
				},
				"element": {
					"type": "multi_users_select",
					"placeholder": {
						"type": "plain_text",
						"text": "Selecione os elogiados",
						"emoji": true
					}
				}
			},
			{
				"type": "input",
				"label": {
					"type": "plain_text",
					"text": "Tipo",
					"emoji": true
				},
				"element": {
					"type": "static_select",
					"placeholder": {
						"type": "plain_text",
						"text": "Selecione o tipo de elogio",
						"emoji": true
					},
					"options": [
						{
							"text": {
								"type": "plain_text",
								"text": ":tada: Trabalho incrível",
								"emoji": true
							},
							"value": "value-0"
						},
						{
							"text": {
								"type": "plain_text",
								"text": "🤖 Uma máquina",
								"emoji": true
							},
							"value": "value-1"
						}
					]
				}
			},
			{
				"type": "input",
				"label": {
					"type": "plain_text",
					"text": "Mensagem",
					"emoji": true
				},
				"element": {
					"type": "plain_text_input",
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
				req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", slackBotToken))

				resp, err := client.Do(req)
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
