package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/perbu/pr-analyzer/gemini"
	"github.com/perbu/pr-analyzer/models"
)

type Processor struct {
	geminiClient *gemini.Client
	dataDir      string
}

func New(apiKey string, model string) (*Processor, error) {
	client, err := gemini.NewClient(apiKey, model)
	if err != nil {
		return nil, err
	}

	return &Processor{
		geminiClient: client,
		dataDir:      "data",
	}, nil
}

func (p *Processor) Close() error {
	return p.geminiClient.Close()
}

func (p *Processor) ProcessAllPRs(ctx context.Context) error {
	log.Println("Starting PR processing with Gemini...")

	// Load processing status
	status, err := gemini.LoadProcessingStatus(p.dataDir)
	if err != nil {
		return fmt.Errorf("failed to load status: %w", err)
	}

	// Get all PR numbers
	prNumbers, err := p.getAllPRNumbers()
	if err != nil {
		return fmt.Errorf("failed to get PR numbers: %w", err)
	}

	status.TotalPRs = len(prNumbers)
	log.Printf("Found %d total PRs", status.TotalPRs)

	// Sort PR numbers to process in order
	sort.Ints(prNumbers)

	// Find starting point
	startIdx := 0
	if status.LastPR > 0 {
		for i, num := range prNumbers {
			if num > status.LastPR {
				startIdx = i
				break
			}
		}
		log.Printf("Resuming from PR #%d (already processed %d PRs)", prNumbers[startIdx], startIdx)
	}

	// Process PRs
	for i := startIdx; i < len(prNumbers); i++ {
		prNumber := prNumbers[i]
		log.Printf("Processing PR #%d (%d/%d)...", prNumber, i+1, len(prNumbers))

		// Load PR data
		prData, err := p.loadPRData(prNumber)
		if err != nil {
			log.Printf("Error loading PR #%d: %v", prNumber, err)
			continue
		}

		// Skip if no comments/reviews
		if len(prData.Comments) == 0 && len(prData.Reviews) == 0 {
			log.Printf("Skipping PR #%d (no comments or reviews)", prNumber)
			continue
		}

		// Skip if no diff_hunk (focus on PRs with code review context)
		if !p.hasDiffHunk(prData) {
			log.Printf("Skipping PR #%d (no diff_hunk - likely not a code review)", prNumber)
			continue
		}

		// Process with Gemini
		learning, err := p.geminiClient.ProcessPR(ctx, prData)
		if err != nil {
			log.Printf("Error processing PR #%d with Gemini: %v", prNumber, err)
			continue
		}

		// Save learning
		if err := gemini.SaveLearning(p.dataDir, learning); err != nil {
			log.Printf("Error saving learning for PR #%d: %v", prNumber, err)
			continue
		}

		// Update status
		status.ProcessedPRs++
		status.LastPR = prNumber
		status.UpdatedAt = time.Now().Format(time.RFC3339)

		if err := gemini.SaveProcessingStatus(p.dataDir, status); err != nil {
			log.Printf("Error saving status: %v", err)
		}

		// Log progress
		if len(learning.Learnings) > 0 {
			log.Printf("  Found %d learnings in %d topics", len(learning.Learnings), len(learning.Topics))
		} else {
			log.Printf("  No style learnings found")
		}

		// Rate limiting - Gemini has generous limits but let's be nice
		if i < len(prNumbers)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	log.Printf("Processing complete! Processed %d PRs", status.ProcessedPRs)
	return nil
}

func (p *Processor) SynthesizeStyleGuide(ctx context.Context) error {
	log.Println("Loading all learnings...")

	learnings, err := gemini.LoadAllLearnings(p.dataDir)
	if err != nil {
		return fmt.Errorf("failed to load learnings: %w", err)
	}

	if len(learnings) == 0 {
		return fmt.Errorf("no learnings found - run 'process-prs' first")
	}

	log.Printf("Found %d PR learnings to synthesize", len(learnings))

	// Count total learnings
	totalLearnings := 0
	for _, l := range learnings {
		totalLearnings += len(l.Learnings)
	}
	log.Printf("Total individual learnings: %d", totalLearnings)

	log.Println("Synthesizing style guide with Gemini...")
	styleGuide, err := p.geminiClient.SynthesizeStyleGuide(ctx, learnings)
	if err != nil {
		return fmt.Errorf("failed to synthesize style guide: %w", err)
	}

	// Save style guide
	outputPath := "STYLE_GUIDE.md"
	if err := os.WriteFile(outputPath, []byte(styleGuide), 0644); err != nil {
		return fmt.Errorf("failed to save style guide: %w", err)
	}

	log.Printf("Style guide saved to %s", outputPath)
	return nil
}

func (p *Processor) getAllPRNumbers() ([]int, error) {
	pullsDir := filepath.Join(p.dataDir, "pulls")
	entries, err := os.ReadDir(pullsDir)
	if err != nil {
		return nil, err
	}

	var numbers []int
	for _, entry := range entries {
		if entry.IsDir() {
			var num int
			if _, err := fmt.Sscanf(entry.Name(), "%d", &num); err == nil {
				numbers = append(numbers, num)
			}
		}
	}

	return numbers, nil
}

func (p *Processor) loadPRData(prNumber int) (*models.PRData, error) {
	prDir := filepath.Join(p.dataDir, "pulls", fmt.Sprintf("%d", prNumber))

	// Load PR metadata
	pr, err := p.loadJSON(filepath.Join(prDir, "pr.json"), &models.PullRequest{})
	if err != nil {
		return nil, err
	}

	// Load commits
	var commits []models.Commit
	if err := p.loadJSONSlice(filepath.Join(prDir, "commits.json"), &commits); err != nil {
		log.Printf("Warning: failed to load commits for PR #%d: %v", prNumber, err)
	}

	// Load comments
	var comments []models.Comment
	if err := p.loadJSONSlice(filepath.Join(prDir, "comments.json"), &comments); err != nil {
		log.Printf("Warning: failed to load comments for PR #%d: %v", prNumber, err)
	}

	// Load reviews
	var reviews []models.Review
	if err := p.loadJSONSlice(filepath.Join(prDir, "reviews.json"), &reviews); err != nil {
		log.Printf("Warning: failed to load reviews for PR #%d: %v", prNumber, err)
	}

	return &models.PRData{
		PR:       *pr.(*models.PullRequest),
		Commits:  commits,
		Comments: comments,
		Reviews:  reviews,
	}, nil
}

func (p *Processor) loadJSON(path string, v interface{}) (interface{}, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(v); err != nil {
		return nil, err
	}

	return v, nil
}

func (p *Processor) loadJSONSlice(path string, v interface{}) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	return json.NewDecoder(file).Decode(v)
}

func (p *Processor) hasDiffHunk(prData *models.PRData) bool {
	// Check if any comment has a diff_hunk (indicates code review)
	for _, comment := range prData.Comments {
		if comment.DiffHunk != "" {
			return true
		}
	}
	return false
}
