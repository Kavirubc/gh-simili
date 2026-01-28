# Simili Issue Intelligence - Setup Guide

This guide explains how to configure Simili Issue Intelligence in your repository.

## Quick Start

1. **Copy workflow files** to your repository:
   ```bash
   cp .github/workflows/issue-*.yml YOUR_REPO/.github/workflows/
   cp .github/workflows/pending-actions.yml YOUR_REPO/.github/workflows/
   ```

2. **Configure secrets** in your repository settings:
   - `APP_ID` - GitHub App ID
   - `APP_PRIVATE_KEY` - GitHub App private key
   - `TRANSFER_PAT` - Personal Access Token with transfer permissions
   - `GEMINI_API_KEY` - Google Gemini API key
   - `QDRANT_URL` - Qdrant vector database URL
   - `QDRANT_API_KEY` - Qdrant API key

3. **Create configuration file** at `.github/simili.yaml` (see example below)

4. **Test** by creating a new issue

## Workflow Files

### `issue-opened.yml`
**Triggers**: When a new issue is opened  
**Purpose**: Full intelligence pipeline
- Similarity detection across repositories
- AI-powered triage and labeling
- Automatic transfer based on routing rules
- Vector database indexing

### `issue-updated.yml`
**Triggers**: When an issue is edited, closed, reopened, or deleted  
**Purpose**: Keep vector database synchronized
- Update embeddings on edits
- Update status on close/reopen
- Remove from index on delete

### `issue-comment.yml`
**Triggers**: When a comment is created  
**Purpose**: Handle user reactions
- Check for revert reactions (👎 on transfer comments)
- Process pending delayed actions
- Execute optimistic transfer reverts

### `pending-actions.yml`
**Triggers**: Hourly schedule + manual dispatch  
**Purpose**: Process expired delayed actions
- Execute transfers after delay period
- Execute duplicate closes after delay period
- Clean up cancelled actions

## Configuration Example

```yaml
# .github/simili.yaml
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

defaults:
  similarity_threshold: 0.65
  max_similar_to_show: 5
  cross_repo_search: true
  delayed_actions:
    enabled: true
    delay_hours: 24
    approve_reaction: "+1"
    cancel_reaction: "-1"
    optimistic_transfers: true

triage:
  enabled: true
  llm:
    provider: "gemini"
    model: "gemini-2.5-flash-lite"
    api_key: "${GEMINI_API_KEY}"

repositories:
  - org: "your-org"
    repo: "your-repo"
    enabled: true
    transfer_rules:
      - match:
          title_contains: ["docs", "documentation"]
        target: "your-org/docs-repo"
        priority: 1
```

## Required Secrets

### GitHub App (Recommended)
Create a GitHub App with these permissions:
- **Issues**: Read & Write
- **Contents**: Read

Set these secrets:
- `APP_ID`: Your GitHub App ID
- `APP_PRIVATE_KEY`: Your GitHub App private key (PEM format)

### Transfer Token
For cross-repository transfers, you need elevated permissions:
- `TRANSFER_PAT`: Personal Access Token or GitHub App token with `repo` scope

### External Services
- `GEMINI_API_KEY`: Get from [Google AI Studio](https://makersuite.google.com/app/apikey)
- `QDRANT_URL`: Your Qdrant instance URL (e.g., `https://your-instance.qdrant.io`)
- `QDRANT_API_KEY`: Your Qdrant API key

## Concurrency Groups

All workflows use repository-specific concurrency groups to prevent conflicts:

```yaml
concurrency:
  group: simili-${{ github.repository }}-issue-${{ github.event.issue.number }}
```

This ensures that:
- ✅ Transfers don't cancel workflows in other repositories
- ✅ Multiple issues can be processed simultaneously
- ✅ Updates to the same issue are serialized

## Testing

1. **Create a test issue** in your repository
2. **Check Actions tab** to see workflow execution
3. **Verify** the issue intelligence comment appears
4. **Test transfer** by creating an issue matching transfer rules
5. **Test revert** by adding 👎 reaction to transfer comment

## Troubleshooting

### Workflow doesn't trigger
- Check that workflow files are in `.github/workflows/`
- Verify file names match exactly
- Ensure repository has Actions enabled

### Permission errors
- Verify GitHub App has correct permissions
- Check that `TRANSFER_PAT` has `repo` scope
- Ensure App is installed on the repository

### API errors
- Verify all secrets are set correctly
- Check API key validity
- Ensure Qdrant instance is accessible

### Transfer failures
- Verify target repository exists
- Check `TRANSFER_PAT` has access to target repo
- Review transfer rules in `simili.yaml`

## Migration from Old Workflows

If you're upgrading from the old unified workflow:

1. **Backup** existing workflows:
   ```bash
   cp .github/workflows/issue-intelligence.yml .github/workflows/issue-intelligence.yml.backup
   ```

2. **Remove** old workflow files:
   ```bash
   rm .github/workflows/issue-intelligence.yml
   rm .github/workflows/issue-full-process.yml
   ```

3. **Add** new workflow files (see Quick Start)

4. **Test** with a new issue

5. **Delete** backups once confirmed working

## Support

For issues or questions:
- GitHub: https://github.com/Kavirubc/gh-simili
- Issues: https://github.com/Kavirubc/gh-simili/issues
