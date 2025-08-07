package query

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/perbu/pr-analyzer/models"
)

type Query struct {
	dataDir string
}

type CommentResult struct {
	PRNumber    int    `json:"pr_number"`
	PRTitle     string `json:"pr_title"`
	Author      string `json:"author"`
	CommentType string `json:"comment_type"`
	Body        string `json:"body"`
	CreatedAt   string `json:"created_at"`
	URL         string `json:"url"`
	Path        string `json:"path,omitempty"`
	Line        *int   `json:"line,omitempty"`
}

func New() *Query {
	return &Query{
		dataDir: "data",
	}
}

func (q *Query) FilterByAuthors(authorsStr, outputFormat string) (string, error) {
	// Parse authors
	authors := make(map[string]bool)
	for _, author := range strings.Split(authorsStr, ",") {
		authors[strings.TrimSpace(author)] = true
	}

	// Load metadata
	metadata, err := q.loadMetadata()
	if err != nil {
		return "", fmt.Errorf("failed to load metadata: %w", err)
	}

	// Collect all comments from selected authors
	var results []CommentResult

	// Read all PR directories
	pullsDir := filepath.Join(q.dataDir, "pulls")
	entries, err := os.ReadDir(pullsDir)
	if err != nil {
		return "", fmt.Errorf("failed to read pulls directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		prDir := filepath.Join(pullsDir, entry.Name())

		// Load PR data
		pr, err := q.loadPR(prDir)
		if err != nil {
			continue
		}

		// Load comments
		comments, err := q.loadComments(prDir)
		if err != nil {
			continue
		}

		// Filter comments by author
		for _, comment := range comments {
			if authors[comment.User.Login] {
				result := CommentResult{
					PRNumber:    pr.Number,
					PRTitle:     pr.Title,
					Author:      comment.User.Login,
					CommentType: comment.Type,
					Body:        comment.Body,
					CreatedAt:   comment.CreatedAt.Format("2006-01-02 15:04:05"),
					URL:         comment.HTMLURL,
					Path:        comment.Path,
					Line:        comment.Line,
				}
				results = append(results, result)
			}
		}

		// Load reviews
		reviews, err := q.loadReviews(prDir)
		if err != nil {
			continue
		}

		// Filter review comments by author
		for _, review := range reviews {
			if authors[review.User.Login] && review.Body != "" {
				result := CommentResult{
					PRNumber:    pr.Number,
					PRTitle:     pr.Title,
					Author:      review.User.Login,
					CommentType: "review",
					Body:        review.Body,
					CreatedAt:   review.SubmittedAt.Format("2006-01-02 15:04:05"),
					URL:         review.HTMLURL,
				}
				results = append(results, result)
			}
		}
	}

	// Sort results by PR number and date
	sort.Slice(results, func(i, j int) bool {
		if results[i].PRNumber != results[j].PRNumber {
			return results[i].PRNumber < results[j].PRNumber
		}
		return results[i].CreatedAt < results[j].CreatedAt
	})

	// Format output
	switch outputFormat {
	case "json":
		return q.formatJSON(results)
	case "csv":
		return q.formatCSV(results)
	default:
		return q.formatStdout(results, metadata, authors)
	}
}

func (q *Query) loadMetadata() (*models.Metadata, error) {
	path := filepath.Join(q.dataDir, "metadata.json")
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var metadata models.Metadata
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&metadata); err != nil {
		return nil, err
	}

	return &metadata, nil
}

func (q *Query) loadPR(prDir string) (*models.PullRequest, error) {
	path := filepath.Join(prDir, "pr.json")
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var pr models.PullRequest
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&pr); err != nil {
		return nil, err
	}

	return &pr, nil
}

func (q *Query) loadComments(prDir string) ([]models.Comment, error) {
	path := filepath.Join(prDir, "comments.json")
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var comments []models.Comment
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&comments); err != nil {
		return nil, err
	}

	return comments, nil
}

func (q *Query) loadReviews(prDir string) ([]models.Review, error) {
	path := filepath.Join(prDir, "reviews.json")
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var reviews []models.Review
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&reviews); err != nil {
		return nil, err
	}

	return reviews, nil
}

func (q *Query) formatJSON(results []CommentResult) (string, error) {
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (q *Query) formatCSV(results []CommentResult) (string, error) {
	var buf strings.Builder
	writer := csv.NewWriter(&buf)

	// Write header
	header := []string{"PR Number", "PR Title", "Author", "Type", "Body", "Created At", "URL", "Path", "Line"}
	if err := writer.Write(header); err != nil {
		return "", err
	}

	// Write data
	for _, r := range results {
		line := ""
		if r.Line != nil {
			line = fmt.Sprintf("%d", *r.Line)
		}

		record := []string{
			fmt.Sprintf("%d", r.PRNumber),
			r.PRTitle,
			r.Author,
			r.CommentType,
			r.Body,
			r.CreatedAt,
			r.URL,
			r.Path,
			line,
		}
		if err := writer.Write(record); err != nil {
			return "", err
		}
	}

	writer.Flush()
	return buf.String(), nil
}

func (q *Query) formatStdout(results []CommentResult, metadata *models.Metadata, authors map[string]bool) (string, error) {
	var buf strings.Builder

	buf.WriteString(fmt.Sprintf("Repository: %s/%s\n", metadata.Owner, metadata.Repository))
	buf.WriteString(fmt.Sprintf("Total PRs: %d\n", metadata.TotalPRs))
	buf.WriteString(fmt.Sprintf("Last Updated: %s\n", metadata.LastUpdated.Format("2006-01-02 15:04:05")))
	buf.WriteString("\n")

	// Show stats for requested authors
	buf.WriteString("Author Statistics:\n")
	for author := range authors {
		count := metadata.AuthorStats[author]
		buf.WriteString(fmt.Sprintf("  %s: %d comments\n", author, count))
	}
	buf.WriteString("\n")

	// Group results by PR
	prGroups := make(map[int][]CommentResult)
	for _, result := range results {
		prGroups[result.PRNumber] = append(prGroups[result.PRNumber], result)
	}

	// Print results grouped by PR
	buf.WriteString(fmt.Sprintf("Found %d comments from selected authors in %d PRs:\n\n", len(results), len(prGroups)))

	for prNumber, comments := range prGroups {
		if len(comments) > 0 {
			buf.WriteString(fmt.Sprintf("PR #%d: %s\n", prNumber, comments[0].PRTitle))
			buf.WriteString(strings.Repeat("-", 80) + "\n")

			for _, comment := range comments {
				buf.WriteString(fmt.Sprintf("Author: %s | Type: %s | Date: %s\n",
					comment.Author, comment.CommentType, comment.CreatedAt))

				if comment.Path != "" {
					buf.WriteString(fmt.Sprintf("File: %s", comment.Path))
					if comment.Line != nil {
						buf.WriteString(fmt.Sprintf(" (line %d)", *comment.Line))
					}
					buf.WriteString("\n")
				}

				buf.WriteString(fmt.Sprintf("URL: %s\n", comment.URL))
				buf.WriteString("\n")

				// Truncate long comments
				body := comment.Body
				if len(body) > 500 {
					body = body[:497] + "..."
				}
				buf.WriteString(body)
				buf.WriteString("\n\n")
			}
			buf.WriteString("\n")
		}
	}

	return buf.String(), nil
}
