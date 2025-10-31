package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/vyper/my-matter/internal/config"
	"github.com/vyper/my-matter/internal/models"
)

// OpenModal opens a Slack modal using the views.open API
func OpenModal(triggerID, viewTemplate string, cfg *config.Config) error {
	var viewRequest map[string]interface{}
	if err := json.Unmarshal([]byte(viewTemplate), &viewRequest); err != nil {
		return fmt.Errorf("error parsing view template: %w", err)
	}

	viewRequest["trigger_id"] = triggerID

	jsonBody, err := json.Marshal(viewRequest)
	if err != nil {
		return fmt.Errorf("error marshaling view request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://slack.com/api/views.open", bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.SlackBotToken))

	resp, err := cfg.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("error making POST request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %w", err)
	}

	log.Printf("Modal opened - Status: %s, Response: %s", resp.Status, string(body))
	return nil
}

// UpdateModal updates an existing Slack modal using views.update API
func UpdateModal(viewID, hash, selectedKudoType, messageValue, viewTemplate string, cfg *config.Config) error {
	var viewData map[string]interface{}
	if err := json.Unmarshal([]byte(viewTemplate), &viewData); err != nil {
		return fmt.Errorf("error parsing view template: %w", err)
	}

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

	var descriptionBlock map[string]interface{}

	// Check if custom type selected
	if selectedKudoType == "custom" {
		// Transform description block into an input field for custom kudo type
		descriptionBlock = map[string]interface{}{
			"type":     "input",
			"block_id": "kudo_description",
			"label": map[string]interface{}{
				"type":  "plain_text",
				"text":  "Nome do tipo de elogio",
				"emoji": true,
			},
			"element": map[string]interface{}{
				"type":        "plain_text_input",
				"action_id":   "kudo_description",
				"placeholder": map[string]interface{}{
					"type":  "plain_text",
					"text":  "Ex: Super Colaborador, Líder Inspirador...",
					"emoji": true,
				},
			},
		}

		// Preserve existing value if switching back to custom
		if messageValue != "" {
			// Check if there's an existing custom description value in the blocks
			if descriptionBlockIndex != -1 {
				existingBlock, ok := blocks[descriptionBlockIndex].(map[string]interface{})
				if ok {
					existingElement, ok := existingBlock["element"].(map[string]interface{})
					if ok {
						if existingValue, ok := existingElement["initial_value"].(string); ok && existingValue != "" {
							element := descriptionBlock["element"].(map[string]interface{})
							element["initial_value"] = existingValue
						}
					}
				}
			}
		}
	} else {
		// Regular kudo type - use context block with description
		description := models.KudoDescriptions[selectedKudoType]
		if description == "" {
			description = "Tipo de elogio selecionado"
		}

		descriptionBlock = map[string]interface{}{
			"type":     "context",
			"block_id": "kudo_description",
			"elements": []interface{}{
				map[string]interface{}{
					"type": "mrkdwn",
					"text": fmt.Sprintf("💡 _%s_", description),
				},
			},
		}
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
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.SlackBotToken))

	resp, err := cfg.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("error making views.update request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response: %w", err)
	}

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
