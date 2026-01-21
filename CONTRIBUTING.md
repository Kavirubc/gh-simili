# Contributing to Simili

Thank you for your interest in contributing to Simili! This document provides guidelines and instructions for contributing.

## Getting Started

### Prerequisites

- Go 1.23 or later
- Docker (for testing the GitHub Action)
- A GitHub account
- Qdrant instance (local or cloud)
- Gemini API key

### Setting Up Development Environment

1. Fork and clone the repository:
   ```bash
   git clone https://github.com/YOUR_USERNAME/gh-simili.git
   cd gh-simili
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Create a `.env` file with your credentials:
   ```bash
   GEMINI_API_KEY=your_key
   QDRANT_URL=your_url
   QDRANT_API_KEY=your_key
   ```

4. Build the project:
   ```bash
   go build -o gh-simili ./cmd/gh-simili
   ```

5. Run tests:
   ```bash
   go test ./...
   ```

## Making Changes

### Branch Naming

Use descriptive branch names:
- `feat/feature-name` - New features
- `fix/bug-description` - Bug fixes
- `docs/what-changed` - Documentation updates
- `refactor/what-changed` - Code refactoring

### Commit Messages

Write clear, concise commit messages:
- Use present tense ("Add feature" not "Added feature")
- Use imperative mood ("Move cursor to..." not "Moves cursor to...")
- Keep the first line under 72 characters
- Reference issues and PRs where appropriate

Example:
```
Add cross-repo similarity search

- Implement org-wide collection strategy
- Add filter for excluding self from results
- Update documentation

Closes #123
```

### Code Style

- Follow standard Go conventions
- Run `go fmt` before committing
- Run `go vet` to catch common issues
- Keep functions focused and small
- Add comments for complex logic

### Testing

- Write tests for new features
- Ensure all existing tests pass
- Test with different configurations
- Test the Docker build locally:
  ```bash
  docker build -t gh-simili .
  ```

## Pull Request Process

1. Create a new branch from `main`
2. Make your changes with clear commits
3. Update documentation if needed
4. Ensure tests pass
5. Push to your fork
6. Open a Pull Request

### PR Description

Include in your PR description:
- What changes were made
- Why the changes were needed
- How to test the changes
- Screenshots if applicable

## Reporting Issues

### Bug Reports

Include:
- Clear description of the issue
- Steps to reproduce
- Expected vs actual behavior
- Environment details (Go version, OS, etc.)
- Relevant logs or error messages

### Feature Requests

Include:
- Clear description of the feature
- Use case / motivation
- Proposed implementation (if any)

## Code of Conduct

- Be respectful and inclusive
- Focus on constructive feedback
- Help others learn and grow

## Questions?

Open an issue with the `question` label or start a discussion.

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
