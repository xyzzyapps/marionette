# Marionette Tutorial

A hands-on guide to extracting, querying, and managing `@tag` annotations
across your codebase.

---

## Table of Contents

1. [Installation](#1-installation)
2. [Your First Index](#2-your-first-index)
3. [Writing @tag Annotations](#3-writing-tag-annotations)
4. [Querying Tags](#4-querying-tags)
5. [Hierarchical Tagging & Taxonomies](#5-hierarchical-tagging--taxonomies)
6. [Multiline Values & Templates](#6-multiline-values--templates)
7. [Tag Mutation (Add/Edit/Remove)](#7-tag-mutation-addeditremove)
8. [Branch Diff & CI](#8-branch-diff--ci)
9. [Migration & Renaming](#9-migration--renaming)
10. [Configuration & Validation](#10-configuration--validation)
11. [MCP Server for AI Agents](#11-mcp-server-for-ai-agents)
12. [Real-World Examples](#12-real-world-examples)
13. [Call Graph & Impact Analysis](#13-call-graph--impact-analysis)
14. [Health Dashboard & CI Gating](#14-health-dashboard--ci-gating)
15. [Full-Text Search & Analytics](#15-full-text-search--analytics)
16. [Reference: All 18 Commands](#16-reference-all-18-commands)

---

## 1. Installation

### Prerequisites

- Go 1.26+ (for building from source)
- Git

### Build from source

```bash
git clone <repo-url> marionette
cd marionette
go build -o marionette.exe ./src/cmd/marionette/
```

Verify it works:

```bash
marionette --help
```

You should see 18 commands: `init`, `index`, `query`, `tag`, `callers`, `callees`,
`impact`, `diff`, `migrate`, `depends`, `health`, `search`, `analytics`,
`lint`, `lsp`, `serve`, `watch`, `stats`.

---

## 2. Your First Index

Let's create a sample project and index it.

### Step 1: Create a small project

Create a file `hello.js`:

```javascript
/**
 * @tag-block api-endpoint
 * @tag route: "/hello"
 * @tag method: "GET"
 * @tag security: "public"
 */
function sayHello(name) {
    // @tag-inline audit: "Greeting was logged"
    console.log("Hello, " + name);
}

/**
 * @tag-block api-endpoint
 * @tag route: "/hello/admin"
 * @tag method: "GET"
 * @tag security: "rbac"
 * @tag roles: ["admin"]
 */
function adminGreeting() {
    return "Welcome, admin!";
}
```

And a Python file `tasks.py`:

```python
# @tag-block cron-job
# @tag schedule: "0 0 * * *"
# @tag side-effects: ["purges-temp", "sends-report"]
def nightly_cleanup():
    # @tag-inline pii: "Clears session data with user emails"
    clear_sessions()
```

### Step 2: Initialize and index

```bash
marionette --db tutorial.db init
marionette --db tutorial.db index .
```

Output:
```
Indexing complete:
  Total files:  2
  Indexed:      2
  Skipped:      0
  Tags found:   5
```

### Step 3: View index stats

```bash
marionette --db tutorial.db stats
```

```
Index Version:  2.0.0
Total Files:    2
Total Tags:     5
```

### Step 4: Query all tags

```bash
marionette --db tutorial.db query --format markdown
```

```
| ID | File | Tag Type | Entity | Kind | Metadata |
|----|------|----------|--------|------|----------|
| tag_00000001 | hello.js | block | sayHello | function_declaration | route=/hello, method=GET, security=public, kind=api-endpoint |
| tag_00000002 | hello.js | inline | - | expression_statement | audit=Greeting was logged |
| tag_00000003 | hello.js | block | adminGreeting | function_declaration | route=/hello/admin, method=GET, security=rbac, roles=[admin], kind=api-endpoint |
| tag_00000004 | tasks.py | block | nightly_cleanup | function_definition | schedule=0 0 * * *, side-effects=[purges-temp sends-report], kind=cron-job |
| tag_00000005 | tasks.py | inline | - | expression_statement | pii=Clears session data with user emails |
```

5 tags indexed across 2 languages in seconds.

---

## 3. Writing @tag Annotations

### Block Tags — for functions, classes, methods

```javascript
/**
 * @tag-block <kind>
 * @tag key1: "value1"
 * @tag key2: 42
 * @tag key3: true
 * @tag key4: ["a", "b"]
 */
function myFunction() { ... }
```

`@tag-block <kind>` creates a **block tag**. The `<kind>` becomes a special
`kind` property in the metadata that you can filter with `--kind`.

### Inline Tags — for specific statements

```javascript
// @tag-inline audit: "This is an audit note"
someCriticalOperation();
```

Inline tags attach to the **next statement**. They have no `kind` — they're
purely key-value metadata on a line of code.

### Properties

After `@tag-block` or `@tag-inline`, add any number of `key: value` pairs:

| Syntax | Parsed As |
|--------|-----------|
| `name: "Alice"` | string |
| `count: 42` | integer |
| `ratio: 3.14` | float |
| `enabled: true` | boolean |
| `tags: ["a", "b"]` | array of strings |
| `name: Alice` | string (unquoted fallback) |
| `name: ````Alice```` | string (backtick) |

---

## 4. Querying Tags

### Basic filters

```bash
# By kind
marionette query --kind api-endpoint

# By key
marionette query --key security

# By key + value
marionette query --key security --value rbac

# By language
marionette query --language javascript

# By tag type
marionette query --tag-type inline

# By entity kind
marionette query --entity-kind function_declaration

# By file pattern
marionette query --file "*.js"
```

### Output formats

```bash
marionette query --format json        # default
marionette query --format markdown    # human-readable tables
marionette query --format ctags       # legacy ctags format
```

### Statistics

```bash
# Index metadata
marionette query --stats

# Aggregated tag statistics (counts by language, kind, key)
marionette query --stat
```

Example output:
```
Tag Statistics:
  Total tags:  5

  By language:
    javascript    3
    python        2

  By kind:
    api-endpoint  3
    cron-job      1

  By key (top 20):
    security       2
    route          2
    ...
```

### Dependency queries

```bash
# Find tags that reference a value
marionette query --depends-on billing
marionette query --depends-on admin
```

This searches all metadata keys and values for the given string.

---

## 5. Hierarchical Tagging & Taxonomies

Keys can use `/` as a path separator, creating a taxonomy tree:

```javascript
/**
 * @tag-block api-endpoint
 * @tag area/billing/tax-calculation: "v2"
 * @tag area/billing/invoicing: "v1"
 * @tag compliance/regions: ["US", "EU"]
 * @tag compliance/gdpr: "relevant"
 */
```

### Querying hierarchical keys

```bash
# Exact match
marionette query --key area/billing/tax-calculation

# All tags with area/billing keys
marionette query --key area/billing

# Compliance-related keys
marionette query --key compliance/regions
```

> Hierarchical key matching is exact. For prefix-based hierarchy queries,
> use the Go API's `FindByKeyHierarchy("area/billing")` method.

### Why hierarchies?

- **Organization**: Group related tags under a path
- **Discovery**: Query all tags under `area/billing/*`
- **Evolution**: Rename `area/billing/tax-calculation` to `tax/v2/calculation`
  using `marionette migrate --rename` (see section 9)

---

## 6. Multiline Values & Templates

### Multiline strings

Use triple quotes for values that span multiple lines:

```python
# @tag description: """
#   Calculates tax for a given invoice line item.
#   Supports VAT, GST, and sales tax regimes.
#   Integrates with Avalara and TaxJar providers.
# """
```

The newlines are preserved in the stored value. Query them normally:

```bash
marionette query --key description
```

### Tag templates

Define reusable tag blocks in `.marionetterc.json`:

```json
{
  "templates": [
    {
      "name": "billing-endpoint",
      "tags": {
        "type": "backend",
        "area/billing": "v1",
        "security": "admin-only",
        "compliance/gdpr": "relevant"
      }
    }
  ]
}
```

Then use `@tag-use` in your code:

```python
# @tag-use billing-endpoint
# @tag route: "POST /v1/calculate-tax"
def calculate_tax(): ...
```

The `@tag-use billing-endpoint` line is expanded inline during indexing,
pulling in all properties from the template. This means:

- **DRY**: Define common properties once
- **Consistency**: All billing endpoints automatically get the same security
  and compliance tags
- **Auditability**: Change the template and re-index to update all endpoints

---

## 7. Tag Mutation (Add/Edit/Remove)

Marionette is **read-only by default** (tags come from source code comments).
But you can manually manage tags for situations where you need to annotate
code that doesn't have `@tag` comments yet, or to override extracted tags.

### Adding a tag

```bash
marionette tag add --file src/api.js --key priority --value critical --kind api-endpoint
```

This creates a tag with:
- A generated ID (`manual_<timestamp>`)
- Entity kind: `manual`
- The key-value pair `priority=critical`
- A `kind` property: `api-endpoint`

### Editing a tag

```bash
# First, find the tag ID
marionette query --key priority --format json

# Then edit it
marionette tag edit --id manual_abc123 --key priority --value high
```

### Removing a tag

```bash
marionette tag rm --id manual_abc123
```

### Use cases for manual tags

- Annotating third-party code you can't modify
- Adding metadata during code review
- Testing tag schemas before writing comments
- Backfilling tags on legacy code

---

## 8. Branch Diff & CI

### Comparing two index databases

Marionette indexes are SQLite databases. You can compare two databases to see
what tags changed between branches:

```bash
# Index main branch
git checkout main
marionette --db main.db index .

# Index feature branch
git checkout feature
marionette --db feature.db index .

# Compare
marionette diff main.db feature.db --key security
```

Output:
```
Diff results (3 changes):
  + ADDED    pii-access
  - REMOVED  rbac
  ~ CHANGED  public -> admin-only
```

### Filtering diffs

```bash
# Only security-related changes
marionette diff main.db feature.db --key security

# Only API endpoint changes
marionette diff main.db feature.db --kind api-endpoint
```

### CI workflow

```yaml
# .github/workflows/marionette-ci.yml
name: Marionette Compliance Check
on: [pull_request]
jobs:
  marionette:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - name: Build Marionette
        run: go build -o marionette ./src/cmd/marionette/
      - name: Index base
        run: |
          git checkout main
          ./marionette --db main.db init && ./marionette --db main.db index .
      - name: Index head
        run: |
          git checkout HEAD
          ./marionette --db head.db init && ./marionette --db head.db index .
      - name: Diff PII tags
        run: |
          ./marionette diff main.db head.db --key security --format markdown
      - name: Lint
        run: ./marionette lint .
```

---

## 9. Migration & Renaming

As your tag taxonomy evolves, you'll need to rename or remove keys.

### Rename a key

```bash
marionette migrate --rename "area/billing/tax-calculation" "tax/v2/calculation"
```

This renames the key on every tag that has it, preserving the value.

### Delete a key

```bash
marionette migrate --delete-key "deprecated-feature-flag"
```

Removes the key from every tag. Other properties are preserved.

### Real-world migration scenarios

```bash
# Rename a hierarchical path
marionette migrate --rename "security/pii" "compliance/pii"

# Clean up after a feature flag removal
marionette migrate --delete-key "feature-flag/legacy-ui"

# Standardize naming
marionette migrate --rename "gdpr" "compliance/gdpr"
```

---

## 10. Configuration & Validation

### `.marionetterc.json` reference

```json
{
  "version": "2.0",
  "index": {
    "exclude": ["node_modules/**", "dist/**", ".git/**"]
  },
  "repositories": [
    {"path": "../backend-go", "label": "Go API", "language": "go"},
    {"path": "../data-pipeline", "label": "Python ETL"}
  ],
  "templates": [
    {"name": "standard-endpoint", "tags": {"type": "backend", "monitored": "true"}}
  ],
  "rules": [
    {"key": "security", "required": true, "allowed_values": ["public", "rbac", "admin-only"]},
    {"key": "gdpr-impact", "type": "string", "allowed_values": ["low", "medium", "high", "critical"]},
    {"key": "area/billing/*", "required": true}
  ],
  "output": {
    "format": "markdown",
    "color": false
  }
}
```

### Rules explained

| Rule | Effect |
|------|--------|
| `"key": "security", "required": true` | Lint error if any tag lacks `security` |
| `"allowed_values": ["public", "rbac"]` | Lint error if `security` has a different value |
| `"type": "string"` | Lint error if the value isn't the right type |
| `"key": "area/billing/*"` | Wildcard applies rule to all hierarchical children |

### Running validation

```bash
# Basic lint
marionette lint ./src

# Exit code 1 on errors (for CI)
marionette lint ./src && echo "Clean!" || echo "Issues found"
```

---

## 11. MCP Server for AI Agents

Marionette can expose its @tag annotations directly to AI coding agents
via the Model Context Protocol (MCP).

### Starting the server

```bash
# Ensure you have an index first
marionette --db demo.db init && marionette --db demo.db index .

# Start the MCP server
marionette serve
```

The server communicates via **stdin/stdout** using JSON-RPC, which is
what MCP expects.

### Available MCP tools

Once the server is running, AI agents (Claude Code, Cursor, etc.) can
call these tools:

| Tool | Purpose |
|------|---------|
| `tagdoc_search` | Search tags by kind, key, value |
| `tagdoc_pii_scan` | List all PII-tagged code |
| `tagdoc_architecture` | Architecture entry points |
| `tagdoc_compliance` | Security/GDPR compliance report |
| `tagdoc_feature_flags` | Feature flag status |
| `tagdoc_depends` | Dependency relationships |

### Agent workflow example

```javascript
// An agent using marionette's MCP tools:

// 1. Understand the architecture
const arch = await call_mcp("tagdoc_architecture")
// Returns: [{kind: "api-endpoint", route: "/users", ...}]

// 2. Check for PII before editing
const pii = await call_mcp("tagdoc_pii_scan")
// Warns: "register_user() handles PII — ensure GDPR compliance"

// 3. Find feature flags to update
const flags = await call_mcp("tagdoc_feature_flags")
// Shows: "onboarding-v2 at 10% rollout"
```

### Integrating with Claude Code

Add to your `~/.claude.json`:

```json
{
  "mcpServers": {
    "marionette": {
      "command": "/path/to/marionette",
      "args": ["--db", "/path/to/project.db", "serve"]
    }
  }
}
```

---

## 12. Real-World Examples

### Example 1: Microservice Architecture Mapping

```python
# @tag-block architecture
# @tag layer: "entrypoint"
# @tag domain: "user-management"
# @tag team-owner: "platform-core"
# @tag depends-on: ["auth-service", "notification-service"]

# @tag-block api-endpoint
# @tag route: "POST /v1/users/register"
# @tag security: "public"
# @tag security:pii-access
# @tag gdpr-impact: "high"
def register_user(request):
    ...
```

Query: `marionette query --kind architecture` — shows all service boundaries.
Query: `marionette query --key security --value pii-access` — shows all PII touchpoints.

Full example in [`examples/microservices/user_service.py`](examples/microservices/user_service.py).

### Example 2: GTD Task Manager

```python
# @tag-block api-endpoint
# @tag command: "add"
# @tag layer: "controller"
# @tag security: "local-user"
def cmd_add(args, store):
    # @tag-inline pii: "Task title may contain user-identifying information"
    task = Task(title=args.title, ...)
    store.save(task)
```

Query: `marionette query --kind api-endpoint` — shows all CLI commands.
Query: `marionette query --key pii` — shows all PII-handling code paths.

Full example in [`examples/todoapp/`](examples/todoapp/).

### Example 3: Agent Directives

```python
# @agent-todo "Add rate limiting to this endpoint"
# @agent-skill "This handler collects PII. Get GDPR consent before storage."
# @agent-ignore begin
# ... legacy code — do not modify ...
# @agent-ignore end
```

These are captured as tags and searchable with `marionette query --key audit`.

---

## 13. Call Graph & Impact Analysis

Marionette builds a call graph from indexed files, tracking function definitions
and call sites.

### Finding callers

```bash
# Who calls register_user?
marionette callers register_user

# Include @tag metadata on caller functions
marionette callers register_user --show-tags
```

### Finding callees

```bash
# What does processPayment call?
marionette callees processPayment

# Show @tag metadata on callee symbols
marionette callees processPayment --show-tags
```

### Impact analysis

```bash
# Blast radius: all tags on and references to a symbol
marionette impact register_user
```

Shows direct tags on the symbol plus all tags referencing it across the index.

---

## 14. Health Dashboard & CI Gating

Marionette's health dashboard grades your index coverage (A-F) based on file and
tag counts.

```bash
# Show health report
marionette health

# Verbose mode: top tag keys
marionette health --verbose

# CI gating: exit code 1 if grade is below threshold
marionette health --threshold B
```

Use `--threshold` in CI pipelines to enforce minimum coverage standards:

```yaml
# CI step
- name: Check health gate
  run: marionette --db demo.db health --threshold B
```

### Lint with architectural constraints

```bash
# Basic lint
marionette lint ./src

# Strict mode: enforce tag, code, and arch constraints
marionette lint --strict --db demo.db ./src
```

Constraint rules are defined in `.marionetterc.json`:
```json
{
  "constraints": [
    {"type": "tag", "name": "pii-needs-gdpr", "rule": "if-then:security=pii-access,gdpr-impact", "severity": "error"},
    {"type": "code", "name": "no-malloc", "rule": "no malloc, calloc, realloc", "severity": "error"},
    {"type": "arch", "name": "billing-boundary", "rule": "billing must not import reporting", "severity": "warning"}
  ]
}
```

---

## 15. Full-Text Search & Analytics

### Search tag metadata

```bash
# Full-text search across all tag keys and values
marionette search "pii"

# Limit results and filter by file pattern
marionette search "billing" --limit 5 --file "*.py"

# JSON output for programmatic use
marionette search "gdpr" --format json
```

### Analytics over time

Record snapshots to track tag growth:

```bash
# Record the current state
marionette analytics --snapshot

# View trend
marionette analytics

# Filter by start date
marionette analytics --since 2026-01-01
```

---

## 16. Reference: All 18 Commands

| Command | Description |
|---------|-------------|
| `marionette init` | Create a new marionette index |
| `marionette index [path...]` | Index source files (31 languages) |
| `marionette query [flags]` | Query tags with filters and stats |
| `marionette tag {add\|edit\|rm}` | Manually manage tags in the index |
| `marionette callers <symbol>` | Show who calls a symbol |
| `marionette callees <symbol>` | Show what a symbol calls |
| `marionette impact <symbol>` | Blast radius analysis |
| `marionette diff <db1> <db2>` | Compare tags between two databases |
| `marionette migrate --rename/--delete-key` | Bulk rename or delete tag keys |
| `marionette depends --on <value>` | Find tags by dependency |
| `marionette health [--threshold]` | Health dashboard with CI gating |
| `marionette search <query>` | Full-text search over tags |
| `marionette analytics [--snapshot]` | Tag analytics over time |
| `marionette lint [path] [--strict]` | Validate + enforce constraints |
| `marionette lsp` | LSP server for IDE integration |
| `marionette serve` | MCP server for AI agents |
| `marionette watch [path]` | Auto-reindex on file changes |
| `marionette stats` | Index statistics |

### Global flags

| Flag | Description |
|------|-------------|
| `--db, -d` | Path to SQLite database (default: `.marionette.db`) |
| `--format, -f` | Output format: `json`, `markdown`, `ctags` |
| `--config` | Path to `.marionetterc.json` |

---

## Next Steps

1. **Try it on your own project**: `marionette init && marionette index . && marionette query`
2. **Add @tag annotations** to your codebase — start with `@tag-block api-endpoint`
   on your API routes
3. **Configure lint rules** in `.marionetterc.json` for CI enforcement
4. **Integrate with your AI agent** using `marionette serve`
5. **Read the SKILL.md** for AI agent behavior guidelines
6. **Explore the examples** in the `examples/` directory
