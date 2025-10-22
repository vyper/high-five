# Phase 2: Extract Services

## Objetivo
Extrair lógica de negócio de `function.go` para serviços reutilizáveis: `modal` (gerenciamento de modais) e `kudos` (formatação de mensagens).

## Pré-requisitos
- Phase 1 completada
- Testes passando
- Estar no branch `refactor/split-cloud-functions`

## Passos

### 1. Criar `internal/services/formatting.go`

**Criar arquivo:**
```bash
touch internal/services/formatting.go
```

**Conteúdo** (copiar funções de formatação de `function.go`):
```go
package services

import (
	"fmt"
	"strings"

	"github.com/slack-go/slack"
)

// FormatUsersForSlack formats a list of user IDs as Slack mentions
// Example: ["U123", "U456"] -> "<@U123>, <@U456>"
func FormatUsersForSlack(userIDs []string) string {
	var usersFormatted []string
	for _, userID := range userIDs {
		usersFormatted = append(usersFormatted, fmt.Sprintf("<@%s>", userID))
	}
	return strings.Join(usersFormatted, ", ")
}

// FormatAsSlackQuote formats a message as a Slack quote block
// Adds "> " prefix to each line to maintain quote formatting
func FormatAsSlackQuote(message string) string {
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

// FormatKudosAsBlocks creates a Slack Block Kit message for kudos
func FormatKudosAsBlocks(senderID string, recipientIDs []string, kudoTypeEmoji string, kudoTypeText string, message string) []slack.Block {
	recipientsFormatted := FormatUsersForSlack(recipientIDs)
	quotedMessage := FormatAsSlackQuote(message)

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
```

### 2. Criar `internal/services/modal.go`

**Criar arquivo:**
```bash
touch internal/services/modal.go
```

**Conteúdo:**
```go
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
```

### 3. Criar `internal/services/kudos.go`

**Criar arquivo:**
```bash
touch internal/services/kudos.go
```

**Conteúdo:**
```go
package services

import (
	"fmt"
	"log"
	"strings"

	"github.com/slack-go/slack"
	"github.com/vyper/my-matter/internal/config"
)

// PostKudos sends a kudos message to Slack channel
func PostKudos(senderID string, recipientIDs []string, kudoTypeEmoji, kudoTypeText, message string, cfg *config.Config) error {
	blocks := FormatKudosAsBlocks(senderID, recipientIDs, kudoTypeEmoji, kudoTypeText, message)

	usersString := FormatUsersForSlack(recipientIDs)
	fallbackText := fmt.Sprintf(
		"%s elogiou %s: %s %s",
		fmt.Sprintf("<@%s>", senderID),
		usersString,
		kudoTypeEmoji,
		kudoTypeText,
	)

	respChannelID, timestamp, err := cfg.SlackAPI.PostMessage(
		cfg.SlackChannelID,
		slack.MsgOptionBlocks(blocks...),
		slack.MsgOptionText(fallbackText, false),
	)
	if err != nil {
		return fmt.Errorf("error posting message: %w", err)
	}

	log.Printf("Message posted to channel %s at %s", respChannelID, timestamp)
	return nil
}

// ParseKudoTypeText splits emoji and text from kudo type full text
// Example: ":zap: Resolvedor(a) de Problemas" -> (":zap:", "Resolvedor(a) de Problemas")
func ParseKudoTypeText(kudoTypeFullText string) (emoji, text string) {
	if idx := strings.Index(kudoTypeFullText, " "); idx > 0 {
		return kudoTypeFullText[:idx], kudoTypeFullText[idx+1:]
	}
	return "", kudoTypeFullText
}
```

### 4. Atualizar `function.go`

**Adicionar import:**
```go
import (
	// ... outros imports
	"github.com/vyper/my-matter/internal/services"
)
```

**Substituir chamadas diretas por serviços:**

Exemplo na função `handleKudos` (linha ~268-309):
```go
// ANTES:
var viewRequest map[string]interface{}
if err := json.Unmarshal([]byte(giveKudosViewTemplate), &viewRequest); err != nil {
	// ...
}
// ... código de abertura de modal ...

// DEPOIS:
if err := services.OpenModal(r.FormValue("trigger_id"), giveKudosViewTemplate, config); err != nil {
	log.Printf("Error opening modal: %v", err)
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	return
}
```

Exemplo na função que posta kudos (linha ~227-266):
```go
// ANTES:
kudoTypeEmoji := ""
kudoTypeText := kudoTypeFullText
if idx := strings.Index(kudoTypeFullText, " "); idx > 0 {
	// ...
}
blocks := formatKudosAsBlocks(...)
// ... PostMessage ...

// DEPOIS:
kudoTypeEmoji, kudoTypeText := services.ParseKudoTypeText(kudoTypeFullText)
if err := services.PostKudos(i.User.ID, selectedUsers, kudoTypeEmoji, kudoTypeText, kudoMessage, config); err != nil {
	log.Printf("Error posting kudos: %v", err)
}
```

Exemplo na função `updateView` (linha ~366-482):
```go
// ANTES:
func updateView(viewID, hash, selectedKudoType, messageValue string, config *Config) error {
	// ... toda a lógica ...
}

// DEPOIS:
func updateView(viewID, hash, selectedKudoType, messageValue string, config *config.Config) error {
	return services.UpdateModal(viewID, hash, selectedKudoType, messageValue, giveKudosViewTemplate, config)
}
```

**Remover funções antigas** de `function.go`:
- `formatUsersForSlack` (linha 105-113)
- `formatAsSlackQuote` (linha 115-127)
- `formatKudosAsBlocks` (linha 129-188)

### 5. Verificar e corrigir imports
```bash
go mod tidy
```

### 6. Commit
```bash
git add internal/services/ function.go
git commit -m "refactor: extract services for modal and kudos management"
```

## Como testar

### Rodar testes
```bash
SLACK_BOT_TOKEN=xoxb-test SLACK_CHANNEL_ID=C123 SLACK_SIGNING_SECRET=secret go test -v
```

### Rodar localmente
```bash
LOCAL_ONLY=true go run cmd/main.go
```

### Verificar compilação
```bash
go build -o /tmp/test-build ./cmd/main.go
```

## Como fazer deploy

✅ **Deploy é seguro nesta fase**

```bash
gcloud functions deploy matter-give-kudos \
  --project 'parafuzo-qa-infra' \
  --gen2 \
  --entry-point GiveKudos \
  --region us-east1 \
  --runtime go125 \
  --verbosity error \
  --source . \
  --trigger-http \
  --allow-unauthenticated \
  --memory 128MiB \
  --set-env-vars "SLACK_BOT_TOKEN=$SLACK_BOT_TOKEN,SLACK_CHANNEL_ID=$SLACK_CHANNEL_ID,SLACK_SIGNING_SECRET=$SLACK_SIGNING_SECRET"
```

## Como fazer rollback

### Git rollback
```bash
git revert HEAD
git push
```

### Redeploy versão anterior
```bash
git checkout HEAD~1
# Deploy novamente
git checkout refactor/split-cloud-functions
```

## Critérios de sucesso
- ✅ `internal/services/formatting.go` criado
- ✅ `internal/services/modal.go` criado
- ✅ `internal/services/kudos.go` criado
- ✅ `function.go` usa os services
- ✅ Funções antigas removidas de `function.go`
- ✅ Todos os testes passam
- ✅ Aplicação roda localmente
- ✅ Deploy bem-sucedido (se executado)

## Próxima fase
➡️ **Phase 3:** `03-extract-handlers.md`
