# Marionette — AI Agent Skill

Marionette extracts structured `@tag` annotations from code comments and builds a
queryable index. This skill file documents how an AI agent should discover,
query, and respect @tag annotations in any codebase.

## Agent Behavior Guidelines

### 1. Before Editing Code, Check @tag Annotations

Scan for `@tag-block`, `@tag-inline`, `@agent-ignore`, and `@agent-skill`
comments before modifying a file. These contain directives about how the code
should be treated.

```
@agent-ignore  — DO NOT modify this region (legacy compat, generated code)
@agent-skill   — Context the agent should know before editing
@agent-todo    — Task the agent should complete
```

### 2. Use Tags as a Source of Truth

- `@tag security:pii-access` — This function handles PII. Do not log or expose
  the return value without checking for sensitive data.
- `@tag deprecation: "..."` — This code is deprecated. Prefer alternatives.
- `@tag feature-flag: "..."` — This code is gated behind a flag. Check status.

### 3. Reply with Marionette Commands

When asked about code structure, prefer running Marionette over grep:

```
# Instead of: grep -r "security" src/
# Use:
marionette query --key security
marionette query --kind api-endpoint
marionette query --depends-on billing
```

## Query Patterns

| Goal | Command |
|------|---------|
| Find all API endpoints | `marionette query --kind api-endpoint` |
| PII compliance scan | `marionette query --key security --value pii-access` |
| Architecture entry points | `marionette query --kind architecture` |
| Feature flags | `marionette query --key feature-flag` |
| Cross-language search | `marionette query --key type --value backend` |
| Hierarchical taxonomy | `marionette query --key area/billing` |
| Tag statistics | `marionette query --stat` |
| Index health | `marionette stats` |
| Health dashboard | `marionette health` |
| Health CI gate | `marionette health --threshold B` |
| Call graph (callers) | `marionette callers <symbol>` |
| Call graph (callees) | `marionette callees <symbol>` |
| Impact analysis | `marionette impact <symbol>` |
| Full-text search | `marionette search <query>` |
| Search with file filter | `marionette search <query> --file "*.py"` |
| Analytics snapshot | `marionette analytics --snapshot` |
| Analytics trend | `marionette analytics` |
| Lint with constraints | `marionette lint --strict` |

## Tag Mutation

```
marionette tag add --file src/api.js --key security --value rbac --kind api-endpoint
marionette tag edit --id tag_abcd1234 --key priority --value critical
marionette tag rm --id tag_abcd1234
marionette migrate --rename "old/key" "new/key"
marionette migrate --delete-key "deprecated-flag"
```

## Diff (Branch Comparison)

```
marionette diff main HEAD --key security        # PII changes in this branch
marionette diff main HEAD --kind api-endpoint   # Route changes
```

## MCP Server (AI Agent Integration)

Marionette can run as an MCP server so agents query tags directly via tools:

```
marionette serve   # starts MCP server on stdin/stdout
```

MCP tools exposed:
- `tagdoc_search` — Search tags by kind, key, value
- `tagdoc_pii_scan` — List all PII-tagged code
- `tagdoc_architecture` — Architecture entry points
- `tagdoc_compliance` — Security/GDPR compliance report
- `tagdoc_feature_flags` — Feature flag status
- `tagdoc_depends` — Dependency relationships

## Language Support

Marionette supports all languages with tree-sitter grammars. Currently indexed:
javascript, typescript, python, rust, go, java, c, cpp, csharp, ruby, php,
swift, kotlin, scala, bash, sql, html, css, yaml, toml.

## Configuration (.marionetterc.json)

```json
{
  "rules": [
    {"key": "security", "allowed_values": ["public", "rbac", "admin-only"]},
    {"key": "gdpr-impact", "required": true}
  ],
  "templates": [
    {"name": "billing-endpoint", "tags": {"type": "backend", "area/billing": "v1"}}
  ],
  "repositories": [
    {"path": "../backend", "label": "Go API"}
  ]
}
```

## Agent Workflow for Marionette

1. **Discover**: Run `marionette stats` or `marionette query --stat` to understand the
   tag landscape.
2. **Navigate**: Use `marionette query --kind architecture` to understand service
   boundaries before making changes.
3. **Comply**: Run `marionette query --key security --value pii-access` before
   touching any function that handles user data.
4. **Verify**: Run `marionette lint` after making changes to check for orphan tags
   or broken annotations.
5. **Commit**: If new tags were added, run `marionette index` to keep the index
   fresh.
