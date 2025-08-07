# PR Analyzer

A Go tool to download and analyze pull requests from GitHub repositories, using Gemini to extract coding knowledge
and generate style and coding guides.

## Example

Please see [VARNISH.md](VARNISH.md) for an example of the style guide generated from open source Varnish repository.

## Features

- Download all PRs (open and closed) from a GitHub repository
- Store PR data in a structured filesystem format
- Process PRs with Gemini Flash 2.5 to extract coding style learnings
- Synthesize learnings into a comprehensive style guide
- Query comments by specific authors
- Export results in multiple formats (stdout, JSON, CSV)
- Rate limiting to respect API limits
- Resume support for interrupted processing

## Installation

```bash
go build -o pr-analyzer
```

## Usage

### 1. Download PRs

```bash
# Using environment variable
export GITHUB_TOKEN=your_github_token
./pr-analyzer download -owner varnishcache -repo varnish-cache

# Or using flag
./pr-analyzer download -token your_github_token -owner varnishcache -repo varnish-cache
```

### 2. Process PRs with Gemini

```bash
# Using environment variable
export GEMINI_API_KEY=your_gemini_api_key
./pr-analyzer process-prs

# Or using flag
./pr-analyzer process-prs -key your_gemini_api_key
```

This will process each PR and extract coding style learnings. Progress is saved, so you can interrupt and resume.

### 3. Synthesize Style Guide

```bash
./pr-analyzer synthesize
```

This creates `STYLE.md` with a comprehensive style guide based on all extracted learnings. Note that you might want to
use the Gemini 2.5 Pro model for better results in this step. You can override the default model by using the `-model`
flag or setting the `GEMINI_MODEL` environment variable. So `GEMINI_MODEL=gemini-2.5-pro` for the 2.5 Pro model.

### Query Comments by Authors (Optional)

```bash
# Display to stdout (default)
./pr-analyzer query -authors "bsdphk,dridi"

# Export as JSON
./pr-analyzer query -authors "bsdphk,dridi" -output json > comments.json

# Export as CSV
./pr-analyzer query -authors "bsdphk,dridi" -output csv > comments.csv
```

## Data Structure

The tool stores PR data in the following structure:

```
data/
├── metadata.json          # Repository metadata and author statistics
├── pulls/
│   ├── 1/
│   │   ├── pr.json       # PR metadata
│   │   ├── commits.json  # Commit history
│   │   ├── comments.json # All comments (issue + review)
│   │   └── reviews.json  # Review data
│   ├── 2/
│   └── ...
└── learnings/
    ├── status.json       # Processing status (for resume)
    ├── 1.json           # Learnings from PR #1
    ├── 2.json           # Learnings from PR #2
    └── ...
```

## Requirements

- Go 1.24 or higher
- GitHub personal access token with repo access
- Gemini API key

## GitHub API Rate Limiting

The tool implements conservative rate limiting (1 request per second) to avoid hitting GitHub's aggressive
API limits. For repositories with many PRs, the initial download may take time.

## Example Workflow

1. Download all PRs from the Varnish repository:
   ```bash
   ./pr-analyzer download -owner varnishcache -repo varnish-cache
   ```

2. Process PRs with Gemini to extract learnings:
   ```bash
   ./pr-analyzer process-prs
   ```

3. Generate the style guide:
   ```bash
   ./pr-analyzer synthesize
   ```

The result will be a `STYLE.md` file containing coding conventions and best practices extracted from thousands of code
reviews.
