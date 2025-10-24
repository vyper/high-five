# Phase 4: Create New Functions

## Objetivo
Criar as duas novas Cloud Functions independentes (`slash-command` e `interactivity`) que rodarão em paralelo com a função existente para validação antes do cutover.

## Pré-requisitos
- Phase 3 completada
- Testes passando
- Estar no branch `refactor/split-cloud-functions`
- Acesso ao GCP e permissões para deploy

## Arquitetura

```
┌─────────────────────┐
│   Slack Platform    │
└──────────┬──────────┘
           │
           ├─────────────────────────────────┐
           │                                 │
           ▼                                 ▼
┌──────────────────────┐         ┌─────────────────────┐
│ matter-slash-command │         │ matter-interactivity│
│   (nova function)    │         │   (nova function)   │
└──────────────────────┘         └─────────────────────┘
           │                                 │
           │                                 │
           ├─────────────────────────────────┤
           │                                 │
           ▼                                 ▼
     ┌──────────────────────────────────────────┐
     │         internal/* (shared code)         │
     └──────────────────────────────────────────┘
```

## Passos

### 1. Criar arquivos de registro no root (IMPORTANTE!)

Para que o Cloud Functions Gen2 encontre os entry points, precisamos criar arquivos de registro no root do projeto que importam os packages das functions:

**Criar `slash_command.go`:**
```bash
touch slash_command.go
```

**Conteúdo:**
```go
package function

import (
	_ "github.com/vyper/my-matter/functions/slashcommand"
)
```

**Criar `interactivity.go`:**
```bash
touch interactivity.go
```

**Conteúdo:**
```go
package function

import (
	_ "github.com/vyper/my-matter/functions/interactivity"
)
```

⚠️ **IMPORTANTE:** Estes arquivos devem usar `package function` (mesmo package do `function.go` existente) para evitar conflitos de package.

### 2. Criar `cmd/slash-command/main.go`

**Criar arquivo:**
```bash
mkdir -p cmd/slash-command
touch cmd/slash-command/main.go
```

**Conteúdo:**
```go
package main

import (
	"log"
	"os"

	"github.com/GoogleCloudPlatform/functions-framework-go/funcframework"
	_ "github.com/vyper/my-matter/functions/slashcommand"
)

func main() {
	port := "8080"
	if envPort := os.Getenv("PORT"); envPort != "" {
		port = envPort
	}

	hostname := ""
	if localOnly := os.Getenv("LOCAL_ONLY"); localOnly == "true" {
		hostname = "127.0.0.1"
	}

	if err := funcframework.StartHostPort(hostname, port); err != nil {
		log.Fatalf("funcframework.StartHostPort: %v\n", err)
	}
}
```

### 3. Criar `functions/slashcommand/function.go`

**Criar arquivo:**
```bash
mkdir -p functions/slashcommand
touch functions/slashcommand/function.go
```

**Conteúdo:**
```go
package slashcommand

import (
	_ "embed"
	"log"
	"net/http"
	"os"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/slack-go/slack"
	"github.com/vyper/my-matter/internal/config"
	"github.com/vyper/my-matter/internal/handlers"
)

var globalConfig *config.Config

//go:embed ../../screens/give-kudos.json
var giveKudosViewTemplate string

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
	handlers.HandleSlashCommand(w, r, giveKudosViewTemplate, globalConfig)
}
```

### 4. Criar `cmd/interactivity/main.go`

**Criar arquivo:**
```bash
mkdir -p cmd/interactivity
touch cmd/interactivity/main.go
```

**Conteúdo:**
```go
package main

import (
	"log"
	"os"

	"github.com/GoogleCloudPlatform/functions-framework-go/funcframework"
	_ "github.com/vyper/my-matter/functions/interactivity"
)

func main() {
	port := "8080"
	if envPort := os.Getenv("PORT"); envPort != "" {
		port = envPort
	}

	hostname := ""
	if localOnly := os.Getenv("LOCAL_ONLY"); localOnly == "true" {
		hostname = "127.0.0.1"
	}

	if err := funcframework.StartHostPort(hostname, port); err != nil {
		log.Fatalf("funcframework.StartHostPort: %v\n", err)
	}
}
```

### 5. Criar `functions/interactivity/function.go`

**Criar arquivo:**
```bash
mkdir -p functions/interactivity
touch functions/interactivity/function.go
```

**Conteúdo:**
```go
package interactivity

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

//go:embed ../../screens/give-kudos.json
var giveKudosViewTemplate string

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
		handlers.HandleBlockActions(w, &callback, giveKudosViewTemplate, globalConfig)
	case slack.InteractionTypeViewSubmission:
		handlers.HandleViewSubmission(w, &callback, globalConfig)
	default:
		log.Printf("Unknown interaction type: %s", callback.Type)
		w.WriteHeader(http.StatusOK)
	}
}
```

### 6. Verificar e corrigir imports
```bash
go mod tidy
```

### 7. Commit
```bash
git add cmd/ functions/ slash_command.go interactivity.go
git commit -m "feat: create separate cloud functions for slash command and interactivity"
```

## Como testar localmente

### Testar slash-command function
```bash
cd cmd/slash-command
LOCAL_ONLY=true PORT=8081 go run main.go
```

Em outro terminal:
```bash
curl -X POST http://localhost:8081/SlashCommand \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "trigger_id=test123"
```

### Testar interactivity function
```bash
cd cmd/interactivity
LOCAL_ONLY=true PORT=8082 go run main.go
```

### Rodar testes existentes
```bash
SLACK_BOT_TOKEN=xoxb-test SLACK_CHANNEL_ID=C123 SLACK_SIGNING_SECRET=secret go test -v ./...
```

## Como fazer deploy

### Deploy Slash Command Function
```bash
gcloud functions deploy matter-slash-command \
  --project 'parafuzo-qa-infra' \
  --gen2 \
  --entry-point SlashCommand \
  --region us-east1 \
  --runtime go125 \
  --verbosity error \
  --source . \
  --trigger-http \
  --allow-unauthenticated \
  --memory 128MiB \
  --set-env-vars "SLACK_BOT_TOKEN=$SLACK_BOT_TOKEN,SLACK_CHANNEL_ID=$SLACK_CHANNEL_ID,SLACK_SIGNING_SECRET=$SLACK_SIGNING_SECRET"
```

### Deploy Interactivity Function
```bash
gcloud functions deploy matter-interactivity \
  --project 'parafuzo-qa-infra' \
  --gen2 \
  --entry-point Interactivity \
  --region us-east1 \
  --runtime go125 \
  --verbosity error \
  --source . \
  --trigger-http \
  --allow-unauthenticated \
  --memory 128MiB \
  --set-env-vars "SLACK_BOT_TOKEN=$SLACK_BOT_TOKEN,SLACK_CHANNEL_ID=$SLACK_CHANNEL_ID,SLACK_SIGNING_SECRET=$SLACK_SIGNING_SECRET"
```

### Capturar URLs das novas functions
```bash
# Slash Command URL
gcloud functions describe matter-slash-command \
  --project parafuzo-qa-infra \
  --region us-east1 \
  --gen2 \
  --format='value(serviceConfig.uri)'

# Interactivity URL
gcloud functions describe matter-interactivity \
  --project parafuzo-qa-infra \
  --region us-east1 \
  --gen2 \
  --format='value(serviceConfig.uri)'
```

**Salvar essas URLs** - você precisará delas na Phase 5!

## Validação pós-deploy

### Testar Slash Command Function
```bash
SLASH_URL=$(gcloud functions describe matter-slash-command \
  --project parafuzo-qa-infra \
  --region us-east1 \
  --gen2 \
  --format='value(serviceConfig.uri)')

curl -X POST "$SLASH_URL/SlashCommand" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "trigger_id=test123"
```

**Esperado:** Status 401 (sem signature válida) ou 400 (trigger_id inválido) - OK!

### Monitorar logs
```bash
# Slash Command logs
gcloud functions logs read matter-slash-command \
  --project parafuzo-qa-infra \
  --region us-east1 \
  --gen2 \
  --limit 50

# Interactivity logs
gcloud functions logs read matter-interactivity \
  --project parafuzo-qa-infra \
  --region us-east1 \
  --gen2 \
  --limit 50
```

## Estado atual do sistema

Neste ponto você tem **3 functions rodando em paralelo**:

1. ✅ `matter-give-kudos` (original) - **ATIVO** no Slack
2. ✅ `matter-slash-command` (nova) - deployada, mas **não usada** ainda
3. ✅ `matter-interactivity` (nova) - deployada, mas **não usada** ainda

**Nenhum impacto no usuário** - função original continua funcionando!

## Troubleshooting

### Erro: "Function failed to start: no matching function found with name"

**Sintoma:**
```
Function failed to start: no matching function found with name: "SlashCommand"
```

**Causa:**
O Cloud Functions Gen2 não consegue encontrar o entry point porque a function está registrada em um package separado (`functions/slashcommand`) e não há um import no root do projeto.

**Solução:**
Criar os arquivos de registro no root (`slash_command.go` e `interactivity.go`) que importam os packages das functions. Isso foi adicionado no passo 1 deste guia.

### Erro: "multiple packages in user code directory"

**Sintoma:**
```
multiple packages in user code directory: function != highfive
```

**Causa:**
Você criou um arquivo no root com um nome de package diferente do `function.go` existente.

**Solução:**
Garantir que todos os arquivos `.go` no root do projeto usem `package function` (o mesmo package do `function.go` original).

## Como fazer rollback

### Deletar novas functions (se necessário)
```bash
gcloud functions delete matter-slash-command \
  --project parafuzo-qa-infra \
  --region us-east1 \
  --gen2 \
  --quiet

gcloud functions delete matter-interactivity \
  --project parafuzo-qa-infra \
  --region us-east1 \
  --gen2 \
  --quiet
```

### Git rollback
```bash
git revert HEAD
```

## Critérios de sucesso
- ✅ `cmd/slash-command/main.go` criado
- ✅ `functions/slashcommand/function.go` criado
- ✅ `cmd/interactivity/main.go` criado
- ✅ `functions/interactivity/function.go` criado
- ✅ Ambas functions compilam localmente
- ✅ Deploy de `matter-slash-command` bem-sucedido
- ✅ Deploy de `matter-interactivity` bem-sucedido
- ✅ URLs das novas functions capturadas
- ✅ Logs das functions acessíveis
- ✅ Function original `matter-give-kudos` ainda está ativa

## Próxima fase
➡️ **Phase 5:** `05-cutover-and-cleanup.md`

**IMPORTANTE:** Não prossiga para Phase 5 até validar que as novas functions estão deployadas e saudáveis. A Phase 5 fará o cutover no Slack e deletará a função antiga.
