<p align="center">
  <img src="assets/logo.png" alt="Simili Logo" width="150">
</p>

<h1 align="center">Simili - Issue Intelligence</h1>

<p align="center">
  <a href="https://github.com/marketplace/actions/simili-issue-intelligence"><img src="https://img.shields.io/badge/Marketplace-Simili-blue?logo=github" alt="GitHub Marketplace"></a>
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
</p>

<p align="center">
  AI-powered GitHub issue intelligence that automatically detects duplicate issues and finds similar issues using semantic search. Supports cross-repo search and issue transfer rules.
</p>

## Features

- **Duplicate Detection**: Automatically find similar issues when new ones are created
- **Cross-Repo Search**: Search for similar issues across all enabled repositories in your organization
- **Issue Transfer**: Automatically route issues to the correct repository based on labels, title, or content
- **Smart Comments**: Post helpful comments linking to related issues
- **Modular Pipeline**: Customize workflow steps to optimize costs and latency
- **Dual Mode**: Works as both a GitHub Action and CLI extension

## Quick Start

### 1. Set up Qdrant Vector Database

Sign up for free at [Qdrant Cloud](https://cloud.qdrant.io/) or self-host.

### 2. Get API Keys

- [Gemini API key](https://ai.google.dev/) for embeddings (free tier available)
- Optional: [OpenAI API key](https://platform.openai.com/) as fallback

### 3. Add Secrets to Repository

Go to Settings > Secrets and variables > Actions and add:
- `GEMINI_API_KEY`
- `QDRANT_URL`
- `QDRANT_API_KEY`

### 4. Create Configuration

Create `.github/simili.yaml`:

```yaml
qdrant:
  url: "${QDRANT_URL}"
  api_key: "${QDRANT_API_KEY}"

embedding:
  primary:
    provider: "gemini"
    model: "gemini-embedding-001"
    api_key: "${GEMINI_API_KEY}"
    dimensions: 768

defaults:
  similarity_threshold: 0.65
  max_similar_to_show: 5
  closed_issue_weight: 0.9
  comment_cooldown_hours: 1

repositories:
  - org: "your-org"
    repo: "your-repo"
    enabled: true
```

### 5. Add Workflow

Create `.github/workflows/issue-intelligence.yml`:

```yaml
name: Issue Intelligence

on:
  issues:
    types: [opened, edited, closed, reopened, deleted]

jobs:
  process:
    runs-on: ubuntu-latest
    permissions:
      issues: write
      contents: read

    steps:
      - uses: actions/checkout@v4

      - uses: Kavirubc/gh-simili@v1
        with:
          config_path: .github/simili.yaml
        env:
          GEMINI_API_KEY: ${{ secrets.GEMINI_API_KEY }}
          QDRANT_URL: ${{ secrets.QDRANT_URL }}
          QDRANT_API_KEY: ${{ secrets.QDRANT_API_KEY }}
```

### 6. Index Existing Issues

Install the CLI and index your existing issues:

```bash
gh extension install Kavirubc/gh-simili
gh simili index --repo your-org/your-repo --config .github/simili.yaml
```

## Custom Bot Name (Optional)

By default, comments appear from "github-actions[bot]". To have comments appear from a custom bot name like "Simili":

### Create a GitHub App

1. Go to Settings > Developer settings > GitHub Apps > New GitHub App
2. Name it "Simili" (or your preferred name)
3. Set permissions:
   - Issues: Read & Write
   - Contents: Read
4. Generate a private key
5. Install the app on your repository

### Use the App Token

```yaml
jobs:
  process:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/create-github-app-token@v1
        id: app-token
        with:
          app-id: ${{ vars.SIMILI_APP_ID }}
          private-key: ${{ secrets.SIMILI_PRIVATE_KEY }}

      - uses: Kavirubc/gh-simili@v1
        with:
          github_token: ${{ steps.app-token.outputs.token }}
        env:
          GEMINI_API_KEY: ${{ secrets.GEMINI_API_KEY }}
          QDRANT_URL: ${{ secrets.QDRANT_URL }}
          QDRANT_API_KEY: ${{ secrets.QDRANT_API_KEY }}
```

## CLI Commands

```bash
# Index existing issues
gh simili index --repo owner/repo --config .github/simili.yaml

# Search for similar issues
gh simili search "login bug" --repo owner/repo --config .github/simili.yaml

# Sync recent updates
gh simili sync --repo owner/repo --since 24h --config .github/simili.yaml

# Validate configuration
gh simili config validate --config .github/simili.yaml
```

## Transfer Rules

Automatically transfer issues to the correct repository:

```yaml
repositories:
  - org: "myorg"
    repo: "main-issues"
    enabled: true
    transfer_rules:
      - match:
          labels: ["backend", "api"]
        target: "myorg/backend-service"
        priority: 1
      - match:
          title_contains: ["frontend", "UI"]
        target: "myorg/web-app"
        priority: 2
```

Rules support:
- **Labels**: `labels: ["backend", "api"]`
- **Title keywords**: `title_contains: ["frontend", "UI"]`
- **Body keywords**: `body_contains: ["database", "SQL"]`
- **Author**: `author: "username"`

## Configuration Reference

| Option | Description | Default |
|--------|-------------|---------|
| `similarity_threshold` | Minimum similarity score (0-1) | `0.65` |
| `max_similar_to_show` | Maximum similar issues to show | `5` |
| `closed_issue_weight` | Weight multiplier for closed issues | `0.9` |
| `comment_cooldown_hours` | Hours before posting another comment | `1` |

## License

MIT
