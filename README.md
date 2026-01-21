# gh-simili

GitHub CLI extension for issue intelligence - auto-transfers issues to the correct repository based on classification rules and detects duplicate/similar issues using semantic search.

## Features

- **Issue Transfer**: Automatically route issues from intake repos to specialized repos based on labels, title, or body content
- **Duplicate Detection**: Find similar issues across all repos in an organization using vector similarity search
- **Cross-Repo Search**: Similarity search spans all enabled repositories in an organization
- **Dual Mode**: Works as both a CLI extension and a GitHub Action

## Installation

### As CLI Extension

```bash
gh extension install yourname/gh-simili
```

### As GitHub Action

Add to your workflow:

```yaml
on:
  issues:
    types: [opened, edited, closed, reopened, deleted]

jobs:
  process:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: yourname/gh-simili@v1
        with:
          config_path: .github/simili.yaml
        env:
          GEMINI_API_KEY: ${{ secrets.GEMINI_API_KEY }}
          QDRANT_URL: ${{ secrets.QDRANT_URL }}
          QDRANT_API_KEY: ${{ secrets.QDRANT_API_KEY }}
```

## Configuration

Create `.github/simili.yaml`:

```yaml
qdrant:
  url: "${QDRANT_URL}"
  api_key: "${QDRANT_API_KEY}"
  use_grpc: true

embedding:
  primary:
    provider: "gemini"
    model: "gemini-embedding-001"
    api_key: "${GEMINI_API_KEY}"
    dimensions: 768
  fallback:
    provider: "openai"
    model: "text-embedding-3-small"
    api_key: "${OPENAI_API_KEY}"
    dimensions: 768

defaults:
  similarity_threshold: 0.82
  max_similar_to_show: 5
  closed_issue_weight: 0.9
  comment_cooldown_hours: 1

repositories:
  - org: "myorg"
    repo: "main-issues"
    enabled: true
    transfer_rules:
      - match:
          labels: ["backend", "api"]
        target: "myorg/backend-service"
        priority: 1
```

## CLI Commands

```bash
# Index existing issues
gh simili index --repo owner/repo

# Process a single issue event
gh simili process --event-path /path/to/event.json

# Sync recent updates
gh simili sync --repo owner/repo --since 24h

# Search for similar issues
gh simili search "login bug" --repo owner/repo

# Validate configuration
gh simili config validate
```

## Transfer Rules

Issues can be automatically transferred based on:

- **Labels**: `labels: ["backend", "api"]`
- **Title keywords**: `title_contains: ["frontend", "UI"]`
- **Body keywords**: `body_contains: ["database", "SQL"]`
- **Author**: `author: "username"`

Rules are evaluated by priority (lower = higher priority). First matching rule wins.

## Requirements

- [Qdrant](https://qdrant.tech/) vector database (Cloud or self-hosted)
- [Gemini API key](https://ai.google.dev/) for embeddings
- GitHub CLI (`gh`) for local usage

## License

MIT
