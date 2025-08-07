package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/perbu/pr-analyzer/github"
	"github.com/perbu/pr-analyzer/models"
)

type Downloader struct {
	client   *github.Client
	dataDir  string
	metadata *models.Metadata
}

func New(token, owner, repo string) *Downloader {
	return &Downloader{
		client:  github.NewClient(token, owner, repo),
		dataDir: "data",
		metadata: &models.Metadata{
			Owner:       owner,
			Repository:  repo,
			AuthorStats: make(map[string]int),
		},
	}
}

func (d *Downloader) DownloadAll(ctx context.Context) error {
	log.Println("Starting PR download...")

	// Create data directory structure
	if err := d.createDirectories(); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// Load existing metadata if available
	if err := d.loadMetadata(); err != nil {
		log.Printf("No existing metadata found, starting fresh: %v", err)
	}

	// Get all closed PRs
	log.Println("Fetching closed PRs...")
	closedPRs, err := d.client.GetPullRequests(ctx, "closed")
	if err != nil {
		return fmt.Errorf("failed to get closed PRs: %w", err)
	}
	log.Printf("Found %d closed PRs", len(closedPRs))

	// Get all open PRs
	log.Println("Fetching open PRs...")
	openPRs, err := d.client.GetPullRequests(ctx, "open")
	if err != nil {
		return fmt.Errorf("failed to get open PRs: %w", err)
	}
	log.Printf("Found %d open PRs", len(openPRs))

	// Combine all PRs
	allPRs := append(closedPRs, openPRs...)
	d.metadata.TotalPRs = len(allPRs)

	// Download detailed data for each PR
	for i, pr := range allPRs {
		log.Printf("Processing PR #%d (%d/%d)...", pr.Number, i+1, len(allPRs))

		prData, err := d.downloadPRData(ctx, pr.Number)
		if err != nil {
			log.Printf("Error downloading PR #%d: %v", pr.Number, err)
			continue
		}

		// Save PR data
		if err := d.savePRData(pr.Number, prData); err != nil {
			log.Printf("Error saving PR #%d: %v", pr.Number, err)
			continue
		}

		// Update author stats
		d.updateAuthorStats(prData)

		// Add a small delay to be nice to GitHub
		if i < len(allPRs)-1 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	// Save metadata
	d.metadata.LastUpdated = time.Now()
	if err := d.saveMetadata(); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	log.Println("Download complete!")
	log.Printf("Total PRs: %d", d.metadata.TotalPRs)
	log.Printf("Total authors: %d", len(d.metadata.AuthorStats))

	return nil
}

func (d *Downloader) downloadPRData(ctx context.Context, prNumber int) (*models.PRData, error) {
	// Get full PR details
	pr, err := d.client.GetPRDetails(ctx, prNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get PR details: %w", err)
	}

	// Get commits
	commits, err := d.client.GetPRCommits(ctx, prNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get commits: %w", err)
	}

	// Get comments (both issue and review comments)
	comments, err := d.client.GetPRComments(ctx, prNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get comments: %w", err)
	}

	// Get reviews
	reviews, err := d.client.GetPRReviews(ctx, prNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get reviews: %w", err)
	}

	return &models.PRData{
		PR:       *pr,
		Commits:  commits,
		Comments: comments,
		Reviews:  reviews,
	}, nil
}

func (d *Downloader) createDirectories() error {
	dirs := []string{
		d.dataDir,
		filepath.Join(d.dataDir, "pulls"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

func (d *Downloader) savePRData(prNumber int, data *models.PRData) error {
	prDir := filepath.Join(d.dataDir, "pulls", fmt.Sprintf("%d", prNumber))
	if err := os.MkdirAll(prDir, 0755); err != nil {
		return fmt.Errorf("failed to create PR directory: %w", err)
	}

	// Save PR metadata
	if err := d.saveJSON(filepath.Join(prDir, "pr.json"), data.PR); err != nil {
		return fmt.Errorf("failed to save PR metadata: %w", err)
	}

	// Save commits
	if err := d.saveJSON(filepath.Join(prDir, "commits.json"), data.Commits); err != nil {
		return fmt.Errorf("failed to save commits: %w", err)
	}

	// Save comments
	if err := d.saveJSON(filepath.Join(prDir, "comments.json"), data.Comments); err != nil {
		return fmt.Errorf("failed to save comments: %w", err)
	}

	// Save reviews
	if err := d.saveJSON(filepath.Join(prDir, "reviews.json"), data.Reviews); err != nil {
		return fmt.Errorf("failed to save reviews: %w", err)
	}

	return nil
}

func (d *Downloader) saveJSON(path string, data interface{}) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}

func (d *Downloader) loadMetadata() error {
	path := filepath.Join(d.dataDir, "metadata.json")
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	return decoder.Decode(d.metadata)
}

func (d *Downloader) saveMetadata() error {
	return d.saveJSON(filepath.Join(d.dataDir, "metadata.json"), d.metadata)
}

func (d *Downloader) updateAuthorStats(data *models.PRData) {
	// Count comments by author
	for _, comment := range data.Comments {
		d.metadata.AuthorStats[comment.User.Login]++
	}

	// Count review body comments
	for _, review := range data.Reviews {
		if review.Body != "" {
			d.metadata.AuthorStats[review.User.Login]++
		}
	}
}
