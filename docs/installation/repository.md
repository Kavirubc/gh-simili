# Repository Installation

This guide covers setting up Simili for a single repository.

## Prerequisites

- A GitHub repository where you want to enable issue intelligence
- Access to create repository secrets
- A Qdrant vector database (free tier available at [Qdrant Cloud](https://cloud.qdrant.io/))
- A Gemini API key (free tier available at [Google AI Studio](https://ai.google.dev/))

## Step 1: Set Up Qdrant Vector Database

1. Sign up at [Qdrant Cloud](https://cloud.qdrant.io/)
2. Create a new cluster (free tier works fine for most repos)
3. Copy your cluster URL (e.g., `https://your-cluster.qdrant.io`)
4. Create an API key and save it securely

## Step 2: Get Embedding API Key

1. Go to [Google AI Studio](https://ai.google.dev/)
2. Create an API key
3. Save the key securely

## Step 3: Add Repository Secrets

Go to your repository **Settings > Secrets and variables > Actions** and add:

| Secret Name | Description |
|-------------|-------------|
| `GEMINI_API_KEY` | Your Gemini API key |
| `QDRANT_URL` | Your Qdrant cluster URL (include `https://`) |
| `QDRANT_API_KEY` | Your Qdrant API key |

## Step 4: Create Configuration File

Create `.github/simili.yaml` in your repository:

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
  - org: "your-username-or-org"
    repo: "your-repo-name"
    enabled: true
```

Replace `your-username-or-org` and `your-repo-name` with your actual values.

## Step 5: Create Workflow File

Create `.github/workflows/issue-intelligence.yml`:

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

      - name: Run Simili Issue Intelligence
        uses: Kavirubc/gh-simili@v2
        with:
          config_path: '.github/simili.yaml'
        env:
          GEMINI_API_KEY: ${{ secrets.GEMINI_API_KEY }}
          QDRANT_URL: ${{ secrets.QDRANT_URL }}
          QDRANT_API_KEY: ${{ secrets.QDRANT_API_KEY }}
```

## Step 6: Index Existing Issues (Optional)

To find similar issues among your existing issues, you need to index them first:

```bash
# Install the CLI extension
gh extension install Kavirubc/gh-simili

# Set environment variables
export GEMINI_API_KEY="your-gemini-api-key"
export QDRANT_URL="https://your-cluster.qdrant.io"
export QDRANT_API_KEY="your-qdrant-api-key"

# Index existing issues
gh simili index --repo your-org/your-repo --config .github/simili.yaml
```

## Step 7: Test the Setup

1. Create a new issue in your repository
2. Check the Actions tab to see the workflow run
3. If similar issues exist, a comment will be posted on the new issue

## Using a Custom Bot Name (Optional)

By default, comments appear from `github-actions[bot]`. To use a custom bot name:

### Create a GitHub App

1. Go to **Settings > Developer settings > GitHub Apps > New GitHub App**
2. Set the name (e.g., "My Issue Bot")
3. Set Homepage URL to your repo URL
4. Uncheck "Active" under Webhook
5. Set permissions:
   - **Issues**: Read & Write
   - **Contents**: Read
6. Click "Create GitHub App"
7. Note the **App ID**
8. Generate and download a **private key**
9. Install the app on your repository

### Add App Secrets

Add these secrets to your repository:

| Secret Name | Description |
|-------------|-------------|
| `APP_ID` | Your GitHub App ID |
| `APP_PRIVATE_KEY` | Contents of the private key file |

### Update Workflow

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

      - name: Run Simili Issue Intelligence
        uses: Kavirubc/gh-simili@v2
        with:
          config_path: '.github/simili.yaml'
          github_token: ${{ steps.app-token.outputs.token }}
        env:
          GEMINI_API_KEY: ${{ secrets.GEMINI_API_KEY }}
          QDRANT_URL: ${{ secrets.QDRANT_URL }}
          QDRANT_API_KEY: ${{ secrets.QDRANT_API_KEY }}
```

## Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `similarity_threshold` | Minimum similarity score (0-1) to show | `0.65` |
| `max_similar_to_show` | Maximum number of similar issues to display | `5` |
| `closed_issue_weight` | Weight multiplier for closed issues (lower = less prominent) | `0.9` |
| `comment_cooldown_hours` | Hours to wait before posting another comment on same issue | `1` |

## Pipeline Configuration (Advanced)

By default, Simili runs the standard pipeline:
1. Check Repository (Gatekeeper)
2. Ensure Vector DB (VectorDBPrep)
3. Similarity Search
4. Transfer Check
5. Triage Analysis
6. Unified Response
7. Action Executor
8. Indexer

You can optimize or customize this flow by adding a `pipeline` section to `simili.yaml`. For example, to prioritize transfers and check triaging before similarity search:

```yaml
pipeline:
  steps:
    - gatekeeper
    - transfer_check      # Run transfer logic early
    - similarity_search   # Skipped if transfer matches
    - triage
    - response_builder
    - action_executor
    - indexer
```

## Troubleshooting

### Workflow fails with "config file not found"
- Ensure `.github/simili.yaml` exists
- Check that `actions/checkout@v4` runs before Simili

### No similar issues found
- Run the index command to index existing issues
- Lower the `similarity_threshold` (try `0.50`)
- Check that the Qdrant connection is working

### Comments not appearing
- Check repository permissions allow the action to write issues
- Verify the workflow has `issues: write` permission
- Check the cooldown period hasn't been triggered

## Next Steps

- [Organization Installation](./organization.md) - Set up cross-repo search and transfers
- [Transfer Rules](../configuration/transfer-rules.md) - Auto-route issues to correct repos
