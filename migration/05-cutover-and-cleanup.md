# Phase 5: Cutover and Cleanup

## Objetivo
Fazer o cutover para as novas Cloud Functions no Slack, validar funcionamento em produção, e remover código legado.

## Pré-requisitos
- Phase 4 completada
- Novas functions deployadas e saudáveis
- URLs das novas functions capturadas
- **IMPORTANTE:** Fazer backup do manifest atual do Slack

## ⚠️ ATENÇÃO: Esta fase impacta produção!

Este é o momento onde:
- Slack começará a usar as novas functions
- Função original será desativada
- Código legado será removido

**Recomendação:** Execute em horário de baixo uso e com equipe disponível.

---

## Passos

### Fase 5.1: Backup e Preparação

#### 1. Fazer backup do manifest Slack atual
```bash
# Salvar manifest atual em arquivo
cat > slack-manifest-backup.json << 'EOF'
{
    "display_information": {
        "name": "Senhor Feedback",
        "description": "O cara do feedback da Parafuzo!",
        "background_color": "#000000"
    },
    "features": {
        "bot_user": {
            "display_name": "Senhor Feedback",
            "always_online": true
        },
        "slash_commands": [
            {
                "command": "/elogie",
                "url": "https://us-east1-parafuzo-qa-infra.cloudfunctions.net/matter-give-kudos",
                "description": "Elogie alguém! 🚀",
                "should_escape": false
            }
        ]
    },
    "oauth_config": {
        "scopes": {
            "bot": [
                "commands",
                "incoming-webhook",
                "chat:write"
            ]
        }
    },
    "settings": {
        "interactivity": {
            "is_enabled": true,
            "request_url": "https://us-east1-parafuzo-qa-infra.cloudfunctions.net/matter-give-kudos"
        },
        "org_deploy_enabled": true,
        "socket_mode_enabled": false,
        "token_rotation_enabled": false
    }
}
EOF

git add slack-manifest-backup.json
git commit -m "backup: save current Slack manifest before cutover"
```

#### 2. Capturar URLs das novas functions (se ainda não tiver)
```bash
SLASH_URL=$(gcloud functions describe matter-slash-command \
  --project parafuzo-qa-infra \
  --region us-east1 \
  --gen2 \
  --format='value(serviceConfig.uri)')

INTERACTIVITY_URL=$(gcloud functions describe matter-interactivity \
  --project parafuzo-qa-infra \
  --region us-east1 \
  --gen2 \
  --format='value(serviceConfig.uri)')

echo "Slash Command URL: $SLASH_URL/SlashCommand"
echo "Interactivity URL: $INTERACTIVITY_URL/Interactivity"
```

#### 3. Criar novo manifest
```bash
cat > slack-manifest-new.json << EOF
{
    "display_information": {
        "name": "Senhor Feedback",
        "description": "O cara do feedback da Parafuzo!",
        "background_color": "#000000"
    },
    "features": {
        "bot_user": {
            "display_name": "Senhor Feedback",
            "always_online": true
        },
        "slash_commands": [
            {
                "command": "/elogie",
                "url": "$SLASH_URL/SlashCommand",
                "description": "Elogie alguém! 🚀",
                "should_escape": false
            }
        ]
    },
    "oauth_config": {
        "scopes": {
            "bot": [
                "commands",
                "incoming-webhook",
                "chat:write"
            ]
        }
    },
    "settings": {
        "interactivity": {
            "is_enabled": true,
            "request_url": "$INTERACTIVITY_URL/Interactivity"
        },
        "org_deploy_enabled": true,
        "socket_mode_enabled": false,
        "token_rotation_enabled": false
    }
}
EOF
```

---

### Fase 5.2: Cutover Gradual

#### Estratégia: Cutover em 2 etapas para reduzir risco

#### Etapa 1: Atualizar apenas Slash Command (5.2.1)

1. **Acessar Slack App Configuration:**
   - Ir para https://api.slack.com/apps
   - Selecionar "Senhor Feedback"
   - Ir em "Slash Commands"

2. **Atualizar URL do comando `/elogie`:**
   - Antiga: `https://us-east1-parafuzo-qa-infra.cloudfunctions.net/matter-give-kudos`
   - Nova: `$SLASH_URL/SlashCommand`
   - Salvar

3. **Testar comando no Slack:**
   ```
   /elogie
   ```
   **Esperado:** Modal abre normalmente

4. **⚠️ IMPORTANTE:** NÃO submeta o modal ainda - apenas verifique se abre

5. **Se falhar:**
   - Reverter URL para a antiga
   - Investigar logs:
   ```bash
   gcloud functions logs read matter-slash-command \
     --project parafuzo-qa-infra \
     --region us-east1 \
     --gen2 \
     --limit 100
   ```

#### Etapa 2: Atualizar Interactivity (5.2.2)

**Só execute se Etapa 1 funcionou perfeitamente!**

1. **Acessar Interactivity & Shortcuts:**
   - No Slack App config, ir em "Interactivity & Shortcuts"

2. **Atualizar Request URL:**
   - Antiga: `https://us-east1-parafuzo-qa-infra.cloudfunctions.net/matter-give-kudos`
   - Nova: `$INTERACTIVITY_URL/Interactivity`
   - Salvar

3. **Teste completo no Slack:**
   ```
   /elogie
   ```
   - Selecionar usuários
   - Selecionar tipo de elogio (verificar descrição aparece)
   - Adicionar mensagem
   - Submeter
   - **Verificar mensagem aparece no canal**

4. **Se falhar:**
   - Reverter URL para a antiga
   - Investigar logs:
   ```bash
   gcloud functions logs read matter-interactivity \
     --project parafuzo-qa-infra \
     --region us-east1 \
     --gen2 \
     --limit 100
   ```

---

### Fase 5.3: Validação em Produção

#### Executar testes manuais completos

1. **Teste básico:**
   - `/elogie` → abre modal ✅
   - Selecionar tipo → descrição aparece ✅
   - Submeter → mensagem postada ✅

2. **Teste de edge cases:**
   - Modal sem mensagem (usa sugestão) ✅
   - Múltiplos usuários ✅
   - Cancelar modal ✅

3. **Monitorar logs por 30 minutos:**
```bash
# Terminal 1: Slash Command
gcloud functions logs tail matter-slash-command \
  --project parafuzo-qa-infra \
  --region us-east1 \
  --gen2

# Terminal 2: Interactivity
gcloud functions logs tail matter-interactivity \
  --project parafuzo-qa-infra \
  --region us-east1 \
  --gen2
```

#### Métricas de sucesso:
- Zero erros 500
- Latência < 2s
- Todos os fluxos funcionando

---

### Fase 5.4: Deletar Function Antiga

**⚠️ Só execute após 24-48h de validação bem-sucedida!**

#### 1. Verificar que nenhuma requisição vai para função antiga
```bash
gcloud functions logs read matter-give-kudos \
  --project parafuzo-qa-infra \
  --region us-east1 \
  --gen2 \
  --limit 50

# Se não houver logs recentes (últimas 24h), pode deletar
```

#### 2. Deletar function antiga
```bash
gcloud functions delete matter-give-kudos \
  --project parafuzo-qa-infra \
  --region us-east1 \
  --gen2 \
  --quiet
```

#### 3. Confirmar deleção
```bash
gcloud functions list --project parafuzo-qa-infra --region us-east1 --gen2 | grep matter
```

**Esperado:** Apenas `matter-slash-command` e `matter-interactivity`

---

### Fase 5.5: Limpar Código Legado

#### 1. Deletar arquivos antigos
```bash
git rm function.go
git rm cmd/main.go
git rm test_helpers.go  # Se não for mais usado
```

#### 2. Atualizar `CLAUDE.md`
Substituir seção de deployment por:

```markdown
### Deployment

#### Deploy Slash Command Function
\`\`\`bash
gcloud functions deploy matter-slash-command \\
  --project 'parafuzo-qa-infra' \\
  --gen2 \\
  --entry-point SlashCommand \\
  --region us-east1 \\
  --runtime go125 \\
  --verbosity error \\
  --source . \\
  --trigger-http \\
  --allow-unauthenticated \\
  --memory 128MiB \\
  --set-env-vars "SLACK_BOT_TOKEN=$SLACK_BOT_TOKEN,SLACK_CHANNEL_ID=$SLACK_CHANNEL_ID,SLACK_SIGNING_SECRET=$SLACK_SIGNING_SECRET"
\`\`\`

#### Deploy Interactivity Function
\`\`\`bash
gcloud functions deploy matter-interactivity \\
  --project 'parafuzo-qa-infra' \\
  --gen2 \\
  --entry-point Interactivity \\
  --region us-east1 \\
  --runtime go125 \\
  --verbosity error \\
  --source . \\
  --trigger-http \\
  --allow-unauthenticated \\
  --memory 128MiB \\
  --set-env-vars "SLACK_BOT_TOKEN=$SLACK_BOT_TOKEN,SLACK_CHANNEL_ID=$SLACK_CHANNEL_ID,SLACK_SIGNING_SECRET=$SLACK_SIGNING_SECRET"
\`\`\`

#### Deploy Both Functions (convenience script)
\`\`\`bash
./deploy.sh
\`\`\`
```

#### 3. Criar script de deploy conveniente
```bash
cat > deploy.sh << 'EOF'
#!/bin/bash
set -e

echo "Deploying Slack functions..."

echo "→ Deploying matter-slash-command..."
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

echo "→ Deploying matter-interactivity..."
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

echo "✅ Deployment complete!"
EOF

chmod +x deploy.sh
```

#### 4. Commit
```bash
git add .
git commit -m "chore: remove legacy code and update documentation"
```

---

### Fase 5.6: Merge para Master

#### 1. Push do branch
```bash
git push origin refactor/split-cloud-functions
```

#### 2. Criar Pull Request
- Título: `refactor: split cloud functions into modular architecture`
- Descrição:
```markdown
## Overview
Refatora função monolítica em 2 Cloud Functions independentes:
- `matter-slash-command`: Processa `/elogie`
- `matter-interactivity`: Processa interações do modal

## Changes
- Extrai código para pacotes `internal/`
- Cria handlers, services e models separados
- Remove ~400 linhas de código monolítico
- Melhora testabilidade e manutenibilidade

## Migration
Migração executada em fases incrementais (ver `migration/` directory).
Todas as fases testadas e validadas.

## Testing
- ✅ Testes unitários passando
- ✅ Testes manuais em produção
- ✅ Validado por 48h sem erros

## Deployment
Functions já deployadas e ativas:
- `matter-slash-command`
- `matter-interactivity`

Function legada deletada:
- ~~`matter-give-kudos`~~
```

#### 3. Merge após aprovação
```bash
git checkout master
git merge refactor/split-cloud-functions
git push origin master
```

#### 4. Deletar branch (opcional)
```bash
git branch -d refactor/split-cloud-functions
git push origin --delete refactor/split-cloud-functions
```

---

## Plano de Rollback

### Se problemas aparecerem após cutover:

#### Rollback Rápido (5-10 minutos)
1. **Reverter URLs no Slack App:**
   - Slack Commands → usar URL antiga
   - Interactivity → usar URL antiga
   - Usar backup salvo em `slack-manifest-backup.json`

2. **Redeploy function antiga (se deletada):**
   ```bash
   git checkout <commit-antes-da-migracao>
   gcloud functions deploy matter-give-kudos ...
   git checkout master
   ```

#### Rollback Git (se código bugado)
```bash
git revert <commit-hash-da-migracao>
git push
# Redeploy functions
```

---

## Checklist Final

### Pré-Cutover
- [ ] Phase 0-4 completadas
- [ ] Novas functions deployadas
- [ ] Testes locais passando
- [ ] URLs capturadas
- [ ] Backup do manifest feito
- [ ] Equipe disponível para suporte

### Durante Cutover
- [ ] Slash command URL atualizada
- [ ] Teste slash command funcionou
- [ ] Interactivity URL atualizada
- [ ] Teste completo funcionou
- [ ] Logs sem erros

### Pós-Cutover (24-48h)
- [ ] Zero erros em produção
- [ ] Latência aceitável
- [ ] Usuários não reportaram problemas
- [ ] Function antiga deletada
- [ ] Código legado removido
- [ ] CLAUDE.md atualizado
- [ ] Script de deploy criado
- [ ] PR mergeado para master

---

## Critérios de Sucesso Final

- ✅ **2 Cloud Functions independentes** rodando em produção
- ✅ **Zero downtime** durante migração
- ✅ **Código modular** (~400 linhas → 10 arquivos)
- ✅ **Função monolítica removida**
- ✅ **Documentação atualizada**
- ✅ **Rollback plan testado e documentado**
- ✅ **Equipe treinada** nas novas functions

---

## Métricas de Melhoria

### Antes (Monolítico)
- 1 Cloud Function
- function.go: 483 linhas
- Complexidade ciclomática: alta
- Difícil testar handlers isoladamente
- Deploy all-or-nothing

### Depois (Modular)
- 2 Cloud Functions independentes
- Código distribuído: ~10 arquivos (~40-50 linhas cada)
- Complexidade reduzida
- Handlers testáveis isoladamente
- Deploy granular

### Benefícios
- ✅ **80% redução** na complexidade do arquivo principal
- ✅ **Deploy independente** de cada função
- ✅ **Testabilidade** aumentada
- ✅ **Manutenibilidade** melhorada
- ✅ **Cold start** mais rápido (functions menores)

---

## 🎉 Migração Completa!

Parabéns! Você migrou com sucesso de uma arquitetura monolítica para uma arquitetura modular e escalável.

**Próximos passos sugeridos:**
1. Adicionar testes unitários para handlers
2. Configurar CI/CD para deploy automatizado
3. Implementar monitoring/alerting
4. Documentar arquitetura no README
