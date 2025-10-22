# Phase 0: Setup

## Objetivo
Preparar o ambiente para a migração incremental, criando o branch de trabalho e a estrutura inicial de diretórios sem alterar nenhum código existente.

## Pré-requisitos
- Git instalado
- Repositório com working directory limpo
- Estar na branch `master`

## Passos

### 1. Verificar status atual
```bash
git status
```
**Esperado:** Working tree clean

### 2. Criar branch de refatoração
```bash
git checkout -b refactor/split-cloud-functions
```

### 3. Criar estrutura de diretórios
```bash
mkdir -p internal/config
mkdir -p internal/models
mkdir -p internal/services
mkdir -p internal/handlers
```

### 4. Commit inicial
```bash
git add internal/
git commit -m "chore: create internal package structure for refactoring"
```

### 5. Push do branch
```bash
git push -u origin refactor/split-cloud-functions
```

## Como testar

### Verificar estrutura criada
```bash
tree internal/
```

**Esperado:**
```
internal/
├── config/
├── handlers/
├── models/
└── services/
```

### Verificar que aplicação ainda funciona
```bash
# Run tests
SLACK_BOT_TOKEN=xoxb-test SLACK_CHANNEL_ID=C123 SLACK_SIGNING_SECRET=secret go test -v

# Run locally
LOCAL_ONLY=true go run cmd/main.go
```

**Esperado:** Tudo funciona normalmente (diretórios vazios não afetam)

## Como fazer deploy
❌ **Não fazer deploy nesta fase** - apenas setup local

## Como fazer rollback
Se quiser descartar e recomeçar:
```bash
git checkout master
git branch -D refactor/split-cloud-functions
```

## Critérios de sucesso
- ✅ Branch `refactor/split-cloud-functions` criado
- ✅ Diretórios `internal/` criados
- ✅ Commit realizado
- ✅ Testes existentes continuam passando
- ✅ Aplicação roda localmente sem erros

## Próxima fase
➡️ **Phase 1:** `01-extract-internal-packages.md`
