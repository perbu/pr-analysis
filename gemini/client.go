package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/perbu/pr-analyzer/models"
	"google.golang.org/api/option"
)

type Client struct {
	client    *genai.Client
	model     *genai.GenerativeModel
	modelName string
}

type Learning struct {
	PRNumber    int      `json:"pr_number"`
	PRTitle     string   `json:"pr_title"`
	Learnings   []string `json:"learnings"`
	Topics      []string `json:"topics"`
	ProcessedAt string   `json:"processed_at"`
}

type ProcessingStatus struct {
	TotalPRs     int    `json:"total_prs"`
	ProcessedPRs int    `json:"processed_prs"`
	LastPR       int    `json:"last_pr"`
	UpdatedAt    string `json:"updated_at"`
}

func NewClient(apiKey string, modelName string) (*Client, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	// Use provided model or default to gemini-2.5-flash
	if modelName == "" {
		modelName = "gemini-2.5-flash"
	}

	log.Printf("Using Gemini model: %s", modelName)
	model := client.GenerativeModel(modelName)

	// Configure model for consistent output
	model.SetTemperature(0.3)
	model.SetTopK(40)
	model.SetTopP(0.95)

	return &Client{
		client:    client,
		model:     model,
		modelName: modelName,
	}, nil
}

func (c *Client) Close() error {
	return c.client.Close()
}

func (c *Client) ProcessPR(ctx context.Context, prData *models.PRData) (*Learning, error) {
	// Build PR context
	prContext := c.buildPRContext(prData)

	prompt := `Analyze this pull request and extract coding style learnings, conventions, and best practices discussed by the reviewers. 

**Pay special attention to the diff_hunk sections** which show the actual code being reviewed along with the reviewers' specific feedback about coding style, patterns, and conventions.

Focus on:

1. Code style preferences (formatting, naming, structure)
2. Architecture patterns and design decisions
3. Error handling approaches
4. Performance considerations
5. Testing requirements and patterns
6. Documentation standards
7. Language-specific patterns and conventions

Extract only concrete, actionable learnings that could guide future contributors. Ignore discussions about bugs or feature-specific logic.

Format your response as JSON with this structure:
{
  "learnings": ["learning 1", "learning 2", ...],
  "topics": ["topic1", "topic2", ...]
}

Pull Request Data:
` + prContext

	resp, err := c.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}

	// Extract JSON from response
	var result struct {
		Learnings []string `json:"learnings"`
		Topics    []string `json:"topics"`
	}

	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		text := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])

		// Try to extract JSON from the response
		jsonStart := strings.Index(text, "{")
		jsonEnd := strings.LastIndex(text, "}")
		if jsonStart != -1 && jsonEnd != -1 && jsonEnd > jsonStart {
			jsonText := text[jsonStart : jsonEnd+1]
			if err := json.Unmarshal([]byte(jsonText), &result); err != nil {
				log.Printf("Failed to parse JSON response for PR #%d: %v", prData.PR.Number, err)
				// Return empty learning instead of failing
				return &Learning{
					PRNumber:    prData.PR.Number,
					PRTitle:     prData.PR.Title,
					Learnings:   []string{},
					Topics:      []string{},
					ProcessedAt: time.Now().Format(time.RFC3339),
				}, nil
			}
		}
	}

	return &Learning{
		PRNumber:    prData.PR.Number,
		PRTitle:     prData.PR.Title,
		Learnings:   result.Learnings,
		Topics:      result.Topics,
		ProcessedAt: time.Now().Format(time.RFC3339),
	}, nil
}

func (c *Client) SynthesizeStyleGuide(ctx context.Context, learnings []Learning) (string, error) {
	// Aggregate all learnings
	var allLearnings []string
	topicCount := make(map[string]int)

	for _, l := range learnings {
		allLearnings = append(allLearnings, l.Learnings...)
		for _, topic := range l.Topics {
			topicCount[topic]++
		}
	}

	learningsText := strings.Join(allLearnings, "\n- ")

	prompt := fmt.Sprintf(`Based on %d learnings extracted from project code reviews, create a concise style guide (1-2 pages) that captures the most important coding conventions and best practices.

The style guide should be practical and actionable. Include sections on:

1. Code Style and Formatting
2. Architecture Patterns
3. Error Handling
4. Performance Guidelines
5. Testing Requirements
6. Documentation Standards

Format as Markdown with clear sections and concrete examples where helpful. Focus on the most frequently mentioned patterns and strongest preferences expressed by reviewers.

Learnings to synthesize:
- %s

Create a guide that new contributors can use to write code that fits well with this project's established style and conventions.`, len(allLearnings), learningsText)

	resp, err := c.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("failed to generate style guide: %w", err)
	}

	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		return fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0]), nil
	}

	return "", fmt.Errorf("no content generated")
}

func (c *Client) buildPRContext(prData *models.PRData) string {
	var sb strings.Builder

	// PR metadata
	sb.WriteString(fmt.Sprintf("PR #%d: %s\n", prData.PR.Number, prData.PR.Title))
	sb.WriteString(fmt.Sprintf("Author: %s\n", prData.PR.User.Login))
	sb.WriteString(fmt.Sprintf("State: %s\n", prData.PR.State))
	if prData.PR.Body != "" {
		sb.WriteString(fmt.Sprintf("\nDescription:\n%s\n", prData.PR.Body))
	}

	// Comments grouped by type
	sb.WriteString("\n--- Comments ---\n")
	for _, comment := range prData.Comments {
		sb.WriteString(fmt.Sprintf("\n[%s by %s]\n", comment.Type, comment.User.Login))
		if comment.Path != "" {
			sb.WriteString(fmt.Sprintf("File: %s", comment.Path))
			if comment.Line != nil {
				sb.WriteString(fmt.Sprintf(" (line %d)", *comment.Line))
			}
			sb.WriteString("\n")
		}
		sb.WriteString(comment.Body)
		sb.WriteString("\n")
	}

	// Reviews
	if len(prData.Reviews) > 0 {
		sb.WriteString("\n--- Reviews ---\n")
		for _, review := range prData.Reviews {
			if review.Body != "" {
				sb.WriteString(fmt.Sprintf("\n[%s review by %s]\n", review.State, review.User.Login))
				sb.WriteString(review.Body)
				sb.WriteString("\n")
			}
		}
	}

	return sb.String()
}

// LoadProcessingStatus loads the current processing status
func LoadProcessingStatus(dataDir string) (*ProcessingStatus, error) {
	path := filepath.Join(dataDir, "learnings", "status.json")
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ProcessingStatus{}, nil
		}
		return nil, err
	}
	defer file.Close()

	var status ProcessingStatus
	if err := json.NewDecoder(file).Decode(&status); err != nil {
		return nil, err
	}

	return &status, nil
}

// SaveProcessingStatus saves the current processing status
func SaveProcessingStatus(dataDir string, status *ProcessingStatus) error {
	dir := filepath.Join(dataDir, "learnings")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := filepath.Join(dir, "status.json")
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(status)
}

// SaveLearning saves a learning to disk
func SaveLearning(dataDir string, learning *Learning) error {
	dir := filepath.Join(dataDir, "learnings")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := filepath.Join(dir, fmt.Sprintf("%d.json", learning.PRNumber))
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(learning)
}

// LoadAllLearnings loads all learning files
func LoadAllLearnings(dataDir string) ([]Learning, error) {
	dir := filepath.Join(dataDir, "learnings")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var learnings []Learning
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") && entry.Name() != "status.json" {
			path := filepath.Join(dir, entry.Name())
			file, err := os.Open(path)
			if err != nil {
				continue
			}

			var learning Learning
			if err := json.NewDecoder(file).Decode(&learning); err == nil {
				learnings = append(learnings, learning)
			}
			file.Close()
		}
	}

	return learnings, nil
}
