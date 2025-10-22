# Phase 1: Extract Internal Packages

## Objetivo
Extrair código compartilhado de `function.go` para pacotes internos (`config` e `models`), mantendo a função atual 100% funcional.

## Pré-requisitos
- Phase 0 completada
- Estar no branch `refactor/split-cloud-functions`
- Testes passando

## Passos

### 1. Criar `internal/config/config.go`

**Criar arquivo:**
```bash
touch internal/config/config.go
```

**Conteúdo** (copiar de `function.go` linhas 19-103):
```go
package config

import (
	"fmt"
	"net/http"
	"time"

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
```

### 2. Criar `internal/models/kudos_types.go`

**Criar arquivo:**
```bash
touch internal/models/kudos_types.go
```

**Conteúdo** (copiar de `function.go` linhas 43-67):
```go
package models

// KudoSuggestedMessages maps kudo type IDs to suggested message text
var KudoSuggestedMessages = map[string]string{
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

// KudoDescriptions maps kudo type IDs to their descriptions
var KudoDescriptions = map[string]string{
	"entrega-excepcional":     "Reconhecer entregas de alta qualidade, no prazo ou superando expectativas",
	"espirito-de-equipe":      "Colaboração, ajudar colegas, trabalho em conjunto",
	"ideia-brilhante":         "Inovação, criatividade, soluções inteligentes",
	"acima-e-alem":            "Ir além do esperado, esforço extra",
	"mestre-em-ensinar":       "Compartilhar conhecimento, mentorar, ensinar",
	"resolvedor-de-problemas": "Resolver problemas complexos, troubleshooting",
	"atitude-positiva":        "Manter o moral alto, positividade, energia boa",
	"crescimento-continuo":    "Aprendizado, desenvolvimento pessoal, adaptabilidade",
	"conquista-do-time":       "Vitórias coletivas, marcos alcançados",
	"resiliencia":             "Superar desafios, persistência, lidar com adversidades",
}
```

### 3. Atualizar `function.go`

**Remover linhas 19-67** e adicionar imports:
```go
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
```

**Atualizar referências no código:**
- `Config` → `config.Config`
- `kudoSuggestedMessages` → `models.KudoSuggestedMessages`
- `kudoDescriptions` → `models.KudoDescriptions`

### 4. Verificar e corrigir imports
```bash
go mod tidy
```

### 5. Commit
```bash
git add internal/config/ internal/models/ function.go
git commit -m "refactor: extract config and models to internal packages"
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

**Nota:** A API da função não mudou, apenas refatoração interna.

## Como fazer rollback

### Git rollback
```bash
git revert HEAD
git push
```

### Redeploy versão anterior
```bash
git checkout HEAD~1
# Deploy novamente com comando acima
git checkout refactor/split-cloud-functions
```

## Critérios de sucesso
- ✅ `internal/config/config.go` criado e funcional
- ✅ `internal/models/kudos_types.go` criado e funcional
- ✅ `function.go` usa os novos pacotes
- ✅ Todos os testes passam
- ✅ Aplicação roda localmente sem erros
- ✅ `go mod tidy` sem erros
- ✅ Deploy bem-sucedido (se executado)

## Próxima fase
➡️ **Phase 2:** `02-extract-services.md`
