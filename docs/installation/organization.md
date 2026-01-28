# Organization Installation

This guide covers setting up Simili across multiple repositories in an organization with cross-repo search and automatic issue transfer capabilities.

## Features

- **Cross-repo similarity search**: Find similar issues across all enabled repos
- **Automatic issue transfers**: Route misfiled issues to the correct repository
- **Unified bot identity**: All comments appear from your organization's bot
- **Centralized configuration**: Manage settings across all repos

## Prerequisites

- A GitHub organization with multiple repositories
- Organization admin access (for secrets and GitHub App installation)
- A Qdrant vector database ([Qdrant Cloud](https://cloud.qdrant.io/))
- A Gemini API key ([Google AI Studio](https://ai.google.dev/))
- A Personal Access Token (PAT) with repo permissions (for issue transfers)

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     GitHub Organization                      │
├─────────────┬─────────────┬─────────────┬─────────────┬────┤
│   repo-1    │   repo-2    │   repo-3    │   repo-4    │... │
│  workflow   │  workflow   │  workflow   │  workflow   │    │
└──────┬──────┴──────┬──────┴──────┬──────┴──────┬──────┴────┘
       │             │             │             │
       └─────────────┴──────┬──────┴─────────────┘
                            │
                            ▼
                    ┌───────────────┐
                    │  Qdrant VDB   │
                    │  (shared)     │
                    └───────────────┘
```

All repositories share the same Qdrant database, enabling cross-repo similarity search.

## Step 1: Set Up Shared Infrastructure

### Qdrant Vector Database

1. Sign up at [Qdrant Cloud](https://cloud.qdrant.io/)
2. Create a cluster (1GB free tier handles ~100k issues)
3. Save the cluster URL and API key

### Gemini API Key

1. Go to [Google AI Studio](https://ai.google.dev/)
2. Create an API key
3. Save the key securely

## Step 2: Create a GitHub App

Creating a GitHub App gives your bot a custom identity and allows proper permission management.

1. Go to **Organization Settings > Developer settings > GitHub Apps > New GitHub App**

2. Configure the app:
   - **Name**: `your-org-bot` (e.g., `simili-bot`)
   - **Homepage URL**: `https://github.com/your-org`
   - **Webhook**: Uncheck "Active"

3. Set permissions:
   - **Repository permissions**:
     - Issues: **Read & Write**
     - Contents: **Read**

4. Click **Create GitHub App**

5. Note the **App ID** (shown at the top)

6. Generate a **private key** (scroll down, click "Generate a private key")

7. **Install the app** on your organization:
   - Go to "Install App" in the left sidebar
   - Select your organization
   - Choose "All repositories" or select specific repos

## Step 3: Create a Personal Access Token (PAT)

Issue transfers require elevated permissions that GitHub Apps cannot provide. Create a PAT:

1. Go to **Settings > Developer settings > Personal access tokens > Fine-grained tokens**

2. Click **Generate new token**

3. Configure:
   - **Token name**: `simili-transfer-token`
   - **Expiration**: Choose appropriate duration
   - **Resource owner**: Select your organization
   - **Repository access**: "All repositories" or select specific repos
   - **Permissions**:
     - Issues: **Read and Write**
     - Contents: **Read**

4. Click **Generate token** and save it securely

## Step 4: Add Organization Secrets

Go to **Organization Settings > Secrets and variables > Actions** and add:

| Secret Name | Description | Repository Access |
|-------------|-------------|-------------------|
| `GEMINI_API_KEY` | Gemini API key | All repositories |
| `QDRANT_URL` | Qdrant cluster URL (include `https://`) | All repositories |
| `QDRANT_API_KEY` | Qdrant API key | All repositories |
| `APP_ID` | GitHub App ID | All repositories |
| `APP_PRIVATE_KEY` | GitHub App private key (full contents) | All repositories |
| `TRANSFER_PAT` | Personal Access Token for transfers | All repositories |

## Step 5: Create Configuration for Each Repository

Each repository needs its own `.github/simili.yaml`. The configuration defines:
- Which repos to search for similar issues
- Transfer rules for routing issues

### Example: Main Product Repo

`.github/simili.yaml` for `your-org/product-core`:

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
  # Enable this repo
  - org: "your-org"
    repo: "product-core"
    enabled: true
    transfer_rules:
      - match:
          title_contains: ["docs", "documentation", "readme", "guide"]
        target: "your-org/product-docs"
        priority: 1
      - match:
          title_contains: ["CLI", "command line", "terminal"]
        target: "your-org/product-cli"
        priority: 2
      - match:
          labels: ["frontend", "ui", "css"]
        target: "your-org/product-web"
        priority: 3

  # Enable cross-repo search from these repos
  - org: "your-org"
    repo: "product-docs"
    enabled: true

  - org: "your-org"
    repo: "product-cli"
    enabled: true

  - org: "your-org"
    repo: "product-web"
    enabled: true
```

### Example: Documentation Repo

`.github/simili.yaml` for `your-org/product-docs`:

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

repositories:
  - org: "your-org"
    repo: "product-docs"
    enabled: true
    transfer_rules:
      - match:
          title_contains: ["bug", "error", "crash", "not working"]
        target: "your-org/product-core"
        priority: 1

  # Cross-repo search
  - org: "your-org"
    repo: "product-core"
    enabled: true
```

## Step 6: Create Workflow for Each Repository

Create `.github/workflows/issue-intelligence.yml` in each repository:

```yaml
name: Issue Intelligence

on:
  issues:
    types: [opened, edited, closed, reopened, deleted]

permissions:
  issues: write
  contents: read

jobs:
  process-issue:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout repository
        uses: actions/checkout@v4

      - name: Generate GitHub App Token
        id: app-token
        uses: actions/create-github-app-token@v1
        with:
          app-id: ${{ secrets.APP_ID }}
          private-key: ${{ secrets.APP_PRIVATE_KEY }}
          owner: your-org

      - name: Run Simili Issue Intelligence
        uses: Kavirubc/gh-simili@v2
        with:
          config_path: '.github/simili.yaml'
          github_token: ${{ steps.app-token.outputs.token }}
          transfer_token: ${{ secrets.TRANSFER_PAT }}
        env:
          GEMINI_API_KEY: ${{ secrets.GEMINI_API_KEY }}
          QDRANT_URL: ${{ secrets.QDRANT_URL }}
          QDRANT_API_KEY: ${{ secrets.QDRANT_API_KEY }}
```

Replace `your-org` with your actual organization name.

## Step 7: Index Existing Issues

Index issues from all repositories to enable cross-repo search:

```bash
# Install CLI extension
gh extension install Kavirubc/gh-simili

# Set environment variables
export GEMINI_API_KEY="your-gemini-api-key"
export QDRANT_URL="https://your-cluster.qdrant.io"
export QDRANT_API_KEY="your-qdrant-api-key"

# Index each repository
gh simili index --repo your-org/product-core --config .github/simili.yaml
gh simili index --repo your-org/product-docs --config .github/simili.yaml
gh simili index --repo your-org/product-cli --config .github/simili.yaml
gh simili index --repo your-org/product-web --config .github/simili.yaml
```

## Transfer Rules Reference

Transfer rules automatically move issues to the correct repository based on content.

### Match Conditions

| Condition | Description | Example |
|-----------|-------------|---------|
| `labels` | Match if issue has ANY of these labels | `labels: ["bug", "defect"]` |
| `title_contains` | Match if title contains ANY keyword | `title_contains: ["CLI", "terminal"]` |
| `body_contains` | Match if body contains ANY keyword | `body_contains: ["API", "endpoint"]` |
| `author` | Match if author matches | `author: "bot-user"` |

### Rule Priority

Lower priority numbers are checked first. When multiple rules match, the lowest priority wins:

```yaml
transfer_rules:
  - match:
      labels: ["critical"]
    target: "your-org/critical-issues"
    priority: 1  # Checked first

  - match:
      title_contains: ["docs"]
    target: "your-org/docs"
    priority: 10  # Checked later
```

### Complex Rules

Combine multiple conditions (all must match):

```yaml
transfer_rules:
  - match:
      labels: ["bug"]
      title_contains: ["API", "REST", "GraphQL"]
    target: "your-org/api-service"
    priority: 1
```

## How It Works

When an issue is created:

1. **Similarity Search**: Searches all enabled repos for similar issues
2. **Post Comment**: If similar issues found, posts a comment with links
3. **Check Transfer Rules**: Evaluates rules in priority order
4. **Transfer Issue**: If a rule matches, transfers to target repo with explanation
5. **Index Issue**: Adds to vector database for future searches

### Transfer Flow

```
User creates issue in repo-A
        │
        ▼
┌───────────────────┐
│ Find similar in   │
│ repo-A, B, C, D   │
└────────┬──────────┘
         │
         ▼
┌───────────────────┐
│ Post similarity   │
│ comment           │
└────────┬──────────┘
         │
         ▼
┌───────────────────┐     Yes    ┌─────────────────┐
│ Transfer rule     │───────────▶│ Transfer to     │
│ matches?          │            │ target repo     │
└────────┬──────────┘            └─────────────────┘
         │ No
         ▼
┌───────────────────┐
│ Index issue in    │
│ vector database   │
└───────────────────┘
```

## Enterprise Optimization: Modular Pipeline

Large organizations can optimize costs and latency by customizing the execution pipeline.

### Prune Expensive Steps
If you rely heavily on **Transfer Rules** to route issues, you can configure the pipeline to check transfers *before* performing expensive Vector DB searches or LLM calls.

Add this to your repo's `.github/simili.yaml`:

```yaml
pipeline:
  steps:
    - gatekeeper
    - transfer_check      # 1. Check transfers (Cheap)
    - similarity_search   # 2. Vector Search (Expensive) - Skipped if transfer matches
    - triage              # 3. LLM Triage (Expensive)
    - response_builder
    - action_executor
    - indexer
```

This ensures that if an issue is immediately transferred (e.g., "docs" -> "docs-repo"), Simili skips the Embedding API call and Vector Search, saving API credits and time.

## Troubleshooting

### Transfer fails with "Resource not accessible"
- Ensure `TRANSFER_PAT` has correct permissions
- Verify PAT is scoped to the organization
- Check PAT hasn't expired

### Cross-repo search not finding issues
- Verify all repos have been indexed
- Check all repos are listed in the config
- Ensure Qdrant connection is working

### Comments showing wrong bot name
- Verify GitHub App is installed on the organization
- Check `owner` parameter in `create-github-app-token` action
- Ensure `APP_ID` and `APP_PRIVATE_KEY` are correct

### Workflow not triggering
- Check workflow file is in `.github/workflows/`
- Verify `on: issues` trigger is configured
- Check Actions are enabled for the repository

## Security Considerations

- **PAT Scope**: Use fine-grained tokens with minimal permissions
- **Secret Rotation**: Rotate PAT periodically
- **App Permissions**: Only grant necessary permissions to GitHub App
- **Audit Logs**: Monitor organization audit logs for transfer activity

## Example Repository Structure

```
your-org/
├── product-core/
│   └── .github/
│       ├── simili.yaml
│       └── workflows/
│           └── issue-intelligence.yml
├── product-docs/
│   └── .github/
│       ├── simili.yaml
│       └── workflows/
│           └── issue-intelligence.yml
├── product-cli/
│   └── .github/
│       ├── simili.yaml
│       └── workflows/
│           └── issue-intelligence.yml
└── product-web/
    └── .github/
        ├── simili.yaml
        └── workflows/
            └── issue-intelligence.yml
```

## Next Steps

- Review [Configuration Reference](../configuration/reference.md) for all options
- Set up [Transfer Rules](../configuration/transfer-rules.md) for your workflow
- Configure [Embedding Providers](../configuration/embedding.md) for optimal results
