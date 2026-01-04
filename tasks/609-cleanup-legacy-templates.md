# Cleanup Legacy Template & Configuration Systems

**Priority**: MEDIUM
**Status**: not-started
**Effort**: 5 hours
**Depends On**: 608

## Context

This task has two parts:
1. Remove the old template system that used variable substitution (`{{.Device.MAC}}`)
2. Migrate ALL `/api/v1/configuration/*` endpoints to `/api/v1/config/*` for consistency

## Items to Remove

### 1. Hardcoded Example Templates
- `internal/configuration/template_examples.go`
- Contains `GetTemplateExamples()` and `GetTemplateDocumentation()`
- Returns templates with Go template syntax: `{{.Device.MAC | macLast4}}`

### 2. Base Template Files
- `internal/configuration/templates/base_gen1.json`
- `internal/configuration/templates/base_gen2.json`
- Loaded by TemplateEngine at startup
- Used for "base template inheritance"

### 3. Variable Substitution Logic
Most of `internal/configuration/template_engine.go`:
- `SubstituteVariables()` - Go template execution
- `addBuiltinFunctions()` - Sprig functions
- `getSafeSprigFunctions()` - Security filtering
- MAC/IP formatting functions
- Template caching logic
- `TemplateContext` struct with Device/Network/Auth/Location

### 4. Legacy Template API Endpoints (Remove)
- `GET /api/v1/configuration/template-examples` → `GetTemplateExamples`
- `POST /api/v1/configuration/preview-template` → `PreviewTemplate`
- `POST /api/v1/configuration/validate-template` → `ValidateTemplate` (old system)
- `GET /api/v1/configuration/templates` → legacy template list
- Routes in `router.go`

### 5. Existing Configuration Endpoints (Migrate)

Move these from `/api/v1/configuration/*` to `/api/v1/config/*`:
- `POST /api/v1/configuration/validate-typed` → `/api/v1/config/validate-typed`
- `POST /api/v1/configuration/convert-to-typed` → `/api/v1/config/convert-to-typed`
- `POST /api/v1/configuration/convert-to-raw` → `/api/v1/config/convert-to-raw`
- `GET /api/v1/configuration/schema` → `/api/v1/config/schema`
- `POST /api/v1/configuration/bulk-validate` → `/api/v1/config/bulk-validate`

Update all:
- Route definitions in `internal/api/router.go`
- Handler implementations
- API client code in UI (`ui/src/api/`)
- Any integration tests

## Items to Keep (If Useful)

Review and potentially keep:
- Utility functions that might be useful elsewhere (MAC formatting, etc.)
- Validation logic that could be repurposed

## Success Criteria

### Part 1: Remove Legacy Template System
- [ ] Remove `template_examples.go`
- [ ] Remove `templates/base_gen1.json` and `templates/base_gen2.json`
- [ ] Remove or refactor `template_engine.go`
- [ ] Remove `GetTemplateExamples` handler
- [ ] Remove `PreviewTemplate` handler (old variable substitution preview)
- [ ] Remove `ValidateTemplate` handler (old system)
- [ ] Remove any UI code that uses removed endpoints

### Part 2: Migrate API Namespace
- [ ] Move all `/configuration/*` routes to `/config/*` in router
- [ ] Update handler paths/documentation
- [ ] Update UI API client calls (`ui/src/api/*.ts`)
- [ ] Update any integration tests
- [ ] Verify no references to old `/configuration/*` paths remain

### Part 3: Validation
- [ ] All tests pass
- [ ] No dead code remaining
- [ ] No import errors
- [ ] UI still works with new API paths

## Implementation Steps

### Step 1: Identify All References

```bash
# Find all usages of removed code
rg "GetTemplateExamples" internal/ ui/
rg "template_examples" internal/ ui/
rg "PreviewTemplate" internal/ ui/
rg "base_gen1|base_gen2" internal/
rg "TemplateEngine|SubstituteVariables" internal/
rg "TemplateContext" internal/
```

### Step 2: Remove Files

```bash
rm internal/configuration/template_examples.go
rm internal/configuration/templates/base_gen1.json
rm internal/configuration/templates/base_gen2.json
rmdir internal/configuration/templates/  # if empty
```

### Step 3: Refactor template_engine.go

Either:
- **Remove entirely** if no useful code remains
- **Keep as utility** if MAC/IP formatting functions are used elsewhere

### Step 4: Update API Router

From `internal/api/router.go`:

**Remove** (legacy template endpoints):
```go
api.HandleFunc("/configuration/template-examples", handler.GetTemplateExamples).Methods("GET")
api.HandleFunc("/configuration/preview-template", handler.PreviewTemplate).Methods("POST")
api.HandleFunc("/configuration/validate-template", handler.ValidateTemplate).Methods("POST")
api.HandleFunc("/configuration/templates", handler.GetTemplates).Methods("GET")
```

**Migrate** (rename `/configuration/` → `/config/`):
```go
// Change these:
api.HandleFunc("/configuration/validate-typed", ...) → api.HandleFunc("/config/validate-typed", ...)
api.HandleFunc("/configuration/convert-to-typed", ...) → api.HandleFunc("/config/convert-to-typed", ...)
api.HandleFunc("/configuration/convert-to-raw", ...) → api.HandleFunc("/config/convert-to-raw", ...)
api.HandleFunc("/configuration/schema", ...) → api.HandleFunc("/config/schema", ...)
api.HandleFunc("/configuration/bulk-validate", ...) → api.HandleFunc("/config/bulk-validate", ...)
```

### Step 5: Update UI

**Remove** legacy template code:
- `ui/src/api/templates.ts` - `getTemplateExamples()`
- `ui/src/pages/TemplateExamplesPage.vue`

**Update** API paths from `/configuration/` to `/config/`:
```bash
# Find all UI files using old paths
rg "/api/v1/configuration/" ui/src/
# Update each occurrence to /api/v1/config/
```

Common files to check:
- `ui/src/api/configuration.ts` or similar
- `ui/src/api/devices.ts`
- Any axios/fetch calls in components

## Files to Remove

- `internal/configuration/template_examples.go`
- `internal/configuration/templates/base_gen1.json`
- `internal/configuration/templates/base_gen2.json`
- `internal/configuration/templates/` (directory)

## Files to Modify

- `internal/configuration/template_engine.go` (remove or refactor)
- `internal/api/handlers.go` (remove GetTemplateExamples, PreviewTemplate)
- `internal/api/router.go` (remove routes)
- `ui/src/api/templates.ts` (remove getTemplateExamples if unused)
- `ui/src/pages/TemplateExamplesPage.vue` (update or remove)

## Validation

```bash
make test-ci

# Verify no references to removed code
rg "GetTemplateExamples|template_examples|base_gen1|base_gen2" internal/
# Should return no results

# Verify no old API paths remain
rg '"/api/v1/configuration/' internal/ ui/
# Should return no results (all migrated to /config/)

# Verify build succeeds
make build

# Test UI still works
cd ui && npm run build
```

## Notes

This cleanup can be done after the new template system is working (task 608). The old and new systems don't conflict, so this is more about code hygiene and API consistency.

The API namespace migration (Part 2) ensures all configuration endpoints use the cleaner `/config/*` path, improving API consistency and developer experience.

Consider keeping a record of what the old system did (in this task file or a doc) in case any concepts need to be revisited.
