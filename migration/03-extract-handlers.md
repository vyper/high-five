# Phase 3: Extract Handlers

## Objetivo
Extrair lógica de roteamento e processamento de interações Slack para handlers dedicados, reduzindo complexidade de `function.go`.

## Pré-requisitos
- Phase 2 completada
- Testes passando
- Estar no branch `refactor/split-cloud-functions`

## Passos

### 1. Criar `internal/handlers/block_actions.go`

**Criar arquivo:**
```bash
touch internal/handlers/block_actions.go
```

**Conteúdo:**
```go
package handlers

import (
	"log"
	"net/http"

	"github.com/slack-go/slack"
	"github.com/vyper/my-matter/internal/config"
	"github.com/vyper/my-matter/internal/models"
	"github.com/vyper/my-matter/internal/services"
)

// HandleBlockActions processes block_actions interactions for dynamic modal updates
func HandleBlockActions(w http.ResponseWriter, callback *slack.InteractionCallback, viewTemplate string, cfg *config.Config) {
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
			err := services.UpdateModal(
				callback.View.ID,
				callback.View.Hash,
				action.SelectedOption.Value,
				suggestedMessage,
				viewTemplate,
				cfg,
			)
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
```

### 2. Criar `internal/handlers/view_submission.go`

**Criar arquivo:**
```bash
touch internal/handlers/view_submission.go
```

**Conteúdo:**
```go
package handlers

import (
	"log"
	"net/http"

	"github.com/slack-go/slack"
	"github.com/vyper/my-matter/internal/config"
	"github.com/vyper/my-matter/internal/models"
	"github.com/vyper/my-matter/internal/services"
)

// HandleViewSubmission processes modal submission and posts the kudos message
func HandleViewSubmission(w http.ResponseWriter, callback *slack.InteractionCallback, cfg *config.Config) {
	// Extract values from modal submission
	selectedUsers := callback.View.State.Values["kudo_users"]["kudo_users"].SelectedUsers
	kudoMessage := callback.View.State.Values["kudo_message"]["kudo_message"].Value
	kudoTypeFullText := callback.View.State.Values["kudo_type"]["kudo_type"].SelectedOption.Text.Text
	kudoTypeValue := callback.View.State.Values["kudo_type"]["kudo_type"].SelectedOption.Value

	// If the user didn't interact with the message field, use the suggested message
	if kudoMessage == "" {
		if suggestedMsg, ok := models.KudoSuggestedMessages[kudoTypeValue]; ok {
			kudoMessage = suggestedMsg
		}
	}

	// Parse emoji and text from kudo type
	kudoTypeEmoji, kudoTypeText := services.ParseKudoTypeText(kudoTypeFullText)

	// Post the kudos message to Slack
	err := services.PostKudos(
		callback.User.ID,
		selectedUsers,
		kudoTypeEmoji,
		kudoTypeText,
		kudoMessage,
		cfg,
	)
	if err != nil {
		log.Printf("Error posting kudos: %v", err)
		// Note: We don't return error to user as modal already closed
	}

	// Acknowledge submission (modal will close)
	w.WriteHeader(http.StatusOK)
}
```

### 3. Criar `internal/handlers/slash_command.go`

**Criar arquivo:**
```bash
touch internal/handlers/slash_command.go
```

**Conteúdo:**
```go
package handlers

import (
	"log"
	"net/http"

	"github.com/vyper/my-matter/internal/config"
	"github.com/vyper/my-matter/internal/services"
)

// HandleSlashCommand processes the /elogie slash command and opens the kudos modal
func HandleSlashCommand(w http.ResponseWriter, r *http.Request, viewTemplate string, cfg *config.Config) {
	triggerID := r.FormValue("trigger_id")
	if triggerID == "" {
		log.Printf("Missing trigger_id in slash command")
		http.Error(w, "Missing trigger_id", http.StatusBadRequest)
		return
	}

	err := services.OpenModal(triggerID, viewTemplate, cfg)
	if err != nil {
		log.Printf("Error opening modal: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
```

### 4. Atualizar `function.go` para usar os handlers

**Adicionar import:**
```go
import (
	// ... outros imports
	"github.com/vyper/my-matter/internal/handlers"
)
```

**Simplificar `handleKudos` (linha ~196-320):**

```go
// handleKudos processes the kudos request with injectable config
func handleKudos(w http.ResponseWriter, r *http.Request, config *config.Config) {
	// Verify Slack signing secret
	_, err := slack.NewSecretsVerifier(r.Header, config.SigningSecret)
	if err != nil {
		log.Printf("Invalid Slack Signing Secret: %v", err)
		http.Error(w, "Invalid Slack Signing Secret", http.StatusUnauthorized)
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
```

**Remover funções antigas** de `function.go`:
- `handleBlockActions` (linha ~322-363)
- `updateView` (linha ~365-482)
- Toda lógica de processamento inline substituída pelos handlers

### 5. Verificar e corrigir imports
```bash
go mod tidy
```

### 6. Commit
```bash
git add internal/handlers/ function.go
git commit -m "refactor: extract handlers for slash command and interactions"
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

### Teste manual com curl (slash command)
```bash
# Simular slash command (requer signature válida)
curl -X POST http://localhost:8080/GiveKudos \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "trigger_id=12345.67890.abcdef"
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
- ✅ `internal/handlers/block_actions.go` criado
- ✅ `internal/handlers/view_submission.go` criado
- ✅ `internal/handlers/slash_command.go` criado
- ✅ `function.go` significativamente menor (~100 linhas)
- ✅ Lógica de roteamento simples e clara
- ✅ Todos os testes passam
- ✅ Aplicação roda localmente
- ✅ Deploy bem-sucedido (se executado)

## Métricas de sucesso
**Antes (Phase 2):**
- `function.go`: ~483 linhas

**Depois (Phase 3):**
- `function.go`: ~100 linhas (redução de ~80%)
- Handlers: ~150 linhas distribuídas em 3 arquivos
- Código mais testável e manutenível

## Próxima fase
➡️ **Phase 4:** `04-create-new-functions.md`
