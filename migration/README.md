# Migration Guide: Monolithic to Modular Architecture

## Overview

This directory contains a step-by-step migration guide to refactor the monolithic `matter-give-kudos` Cloud Function into a modular architecture with 2 independent Cloud Functions.

## Migration Strategy

**Approach:** Incremental, deployable phases with zero downtime

Each phase is:
- ✅ **Independently deployable** - Can deploy after each phase
- ✅ **Testable** - Tests pass after each phase
- ✅ **Reversible** - Clear rollback instructions
- ✅ **Safe** - Original function keeps working until final cutover

## Architecture Change

### Before (Monolithic)
```
┌─────────────────────────────────────┐
│                                     │
│     matter-give-kudos               │
│     (single function)               │
│                                     │
│  • Slash command handler            │
│  • Block actions handler            │
│  • View submission handler          │
│  • All logic in function.go         │
│    (483 lines)                      │
│                                     │
└─────────────────────────────────────┘
```

### After (Modular)
```
┌──────────────────────┐    ┌─────────────────────┐
│ matter-slash-command │    │ matter-interactivity│
│                      │    │                     │
│ • /elogie handler    │    │ • Block actions     │
│ • Opens modal        │    │ • View submission   │
└──────────┬───────────┘    └──────────┬──────────┘
           │                           │
           └───────────┬───────────────┘
                       │
           ┌───────────▼────────────┐
           │   internal/ packages   │
           │                        │
           │ • config/              │
           │ • models/              │
           │ • services/            │
           │ • handlers/            │
           └────────────────────────┘
```

## Migration Phases

Execute phases in order. Each phase can be completed independently.

| Phase | Description | Duration | Deploy Safe? |
|-------|-------------|----------|--------------|
| **[Phase 0](00-setup.md)** | Setup branch and directories | 10 min | ❌ No deploy |
| **[Phase 1](01-extract-internal-packages.md)** | Extract config & models | 30 min | ✅ Yes |
| **[Phase 2](02-extract-services.md)** | Extract business logic | 45 min | ✅ Yes |
| **[Phase 3](03-extract-handlers.md)** | Extract request handlers | 45 min | ✅ Yes |
| **[Phase 4](04-create-new-functions.md)** | Create new Cloud Functions | 60 min | ✅ Yes (parallel) |
| **[Phase 5](05-cutover-and-cleanup.md)** | Cutover & cleanup | 30 min + 48h validation | ⚠️ Production impact |

**Total estimated time:** ~4-5 hours active work + 48h validation

## Quick Start

### Prerequisites
- Go 1.25+
- gcloud CLI configured
- Access to `parafuzo-qa-infra` GCP project
- Slack app admin access
- Git repository access

### Execution

```bash
# Start with Phase 0
cd migration
cat 00-setup.md

# Follow each phase sequentially
# After each phase, validate with tests:
SLACK_BOT_TOKEN=xoxb-test SLACK_CHANNEL_ID=C123 SLACK_SIGNING_SECRET=secret go test -v

# Optionally deploy after Phases 1-4 to validate in production
gcloud functions deploy matter-give-kudos ...
```

## Phase Details

### Phase 0: Setup
- **Goal:** Create branch and directory structure
- **Changes:** New directories only, no code changes
- **Deploy:** Not applicable
- **Risk:** None

### Phase 1: Extract Internal Packages
- **Goal:** Move config and models to `internal/`
- **Changes:** Code organization, no logic changes
- **Deploy:** Safe - internal refactor only
- **Risk:** Low

### Phase 2: Extract Services
- **Goal:** Extract business logic to services
- **Changes:** Service layer created, function.go simplified
- **Deploy:** Safe - internal refactor only
- **Risk:** Low

### Phase 3: Extract Handlers
- **Goal:** Extract request handling logic
- **Changes:** function.go becomes thin router (~100 lines)
- **Deploy:** Safe - internal refactor only
- **Risk:** Low

### Phase 4: Create New Functions
- **Goal:** Create 2 new Cloud Functions
- **Changes:** New functions deployed alongside old one
- **Deploy:** Safe - functions run in parallel
- **Risk:** Medium (new infrastructure)
- **Rollback:** Delete new functions

### Phase 5: Cutover and Cleanup
- **Goal:** Switch Slack to new functions, delete old
- **Changes:** Slack config updated, old function deleted
- **Deploy:** Production impact - requires monitoring
- **Risk:** High (production cutover)
- **Rollback:** Revert Slack URLs to old function

## Safety Measures

### Built-in Safety
- ✅ Each phase is tested before proceeding
- ✅ Original function works until Phase 5
- ✅ New functions deployed in parallel (Phase 4)
- ✅ Gradual cutover (slash command → interactivity)
- ✅ 48h validation period before cleanup
- ✅ Clear rollback instructions at each step

### Rollback Strategy
Each phase includes rollback instructions. General rollback:

```bash
# Git rollback
git revert <commit-hash>
git push

# Function rollback (if deployed)
git checkout <previous-commit>
gcloud functions deploy matter-give-kudos ...
```

### Emergency Rollback (Phase 5)
If issues after cutover:

1. **Immediate:** Revert Slack App URLs to old function
2. **If old function deleted:** Redeploy from git history
3. **Code issues:** Git revert + redeploy

## Testing Strategy

### After Each Phase
```bash
# Run tests
SLACK_BOT_TOKEN=xoxb-test SLACK_CHANNEL_ID=C123 SLACK_SIGNING_SECRET=secret go test -v

# Run locally
LOCAL_ONLY=true go run cmd/main.go

# Compile check
go build ./...
```

### Phase 4 (New Functions)
```bash
# Test new functions locally
cd cmd/slash-command && LOCAL_ONLY=true PORT=8081 go run main.go
cd cmd/interactivity && LOCAL_ONLY=true PORT=8082 go run main.go

# After deploy, test endpoints
curl -X POST <function-url> -d "trigger_id=test"
```

### Phase 5 (Production)
- Manual testing in Slack
- Monitor logs for 30 minutes
- Validate for 48 hours before cleanup

## Benefits

### Code Quality
- **Before:** 483 lines in single file
- **After:** ~40-50 lines per file across 10+ files
- **Reduction:** 80% complexity decrease

### Maintainability
- ✅ Clear separation of concerns
- ✅ Each component testable in isolation
- ✅ Easy to add new commands/interactions
- ✅ Smaller deployment units

### Operations
- ✅ Independent deployment of functions
- ✅ Faster cold starts (smaller functions)
- ✅ Better error isolation
- ✅ Clearer logs and monitoring

### Development
- ✅ Easier onboarding
- ✅ Parallel development possible
- ✅ Reduced merge conflicts
- ✅ Better IDE navigation

## Common Issues & Solutions

### Issue: Tests fail after Phase 1
**Solution:** Check import paths - they should reference `internal/` packages

### Issue: Deploy fails in Phase 4
**Solution:** Ensure `go.mod` is up to date: `go mod tidy`

### Issue: Slash command works but modal doesn't submit (Phase 5)
**Solution:** Check interactivity URL is updated in Slack App config

### Issue: Function cold starts slow
**Solution:** Increase memory allocation or use min instances

## Support

### Documentation
- `CLAUDE.md` - Project overview and commands
- Each phase file - Detailed step-by-step instructions
- Inline code comments - Implementation details

### Logs & Monitoring
```bash
# View logs for specific function
gcloud functions logs read <function-name> \
  --project parafuzo-qa-infra \
  --region us-east1 \
  --gen2 \
  --limit 100

# Tail logs
gcloud functions logs tail <function-name> \
  --project parafuzo-qa-infra \
  --region us-east1 \
  --gen2
```

### Rollback Contacts
If issues arise during Phase 5 (production cutover):
1. Check #incidents channel
2. Follow emergency rollback procedure in `05-cutover-and-cleanup.md`
3. Contact DevOps team if infrastructure issues

## Success Metrics

Track these metrics to validate migration success:

### Performance
- [ ] Cold start time ≤ previous implementation
- [ ] P95 latency ≤ previous implementation
- [ ] Zero errors in production for 48h

### Code Quality
- [ ] All tests passing
- [ ] Code coverage ≥ previous coverage
- [ ] Complexity reduced by ~80%

### Operations
- [ ] Successful deploy of new functions
- [ ] Successful cutover with zero downtime
- [ ] Rollback procedure tested and documented

## Timeline Example

**Week 1:**
- Monday: Phases 0-1
- Tuesday: Phase 2
- Wednesday: Phase 3
- Thursday: Phase 4 (deploy + validate)

**Week 2:**
- Monday: Phase 5 (cutover during low traffic)
- Mon-Wed: Validation period (48h)
- Thursday: Cleanup (delete old function, merge PR)

## Post-Migration

After successful migration:

1. **Update monitoring dashboards** for new functions
2. **Update runbooks** with new deployment commands
3. **Train team** on new architecture
4. **Document learnings** in postmortem/retrospective
5. **Plan next improvements** (CI/CD, tests, monitoring)

## Questions?

Review phase files for detailed instructions. Each phase includes:
- Clear objectives
- Step-by-step commands
- Testing procedures
- Rollback instructions
- Success criteria

Good luck with your migration! 🚀
