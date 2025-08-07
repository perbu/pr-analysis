package github

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v56/github"
	"github.com/perbu/pr-analyzer/models"
	"golang.org/x/oauth2"
	"golang.org/x/time/rate"
)

type Client struct {
	client  *github.Client
	owner   string
	repo    string
	limiter *rate.Limiter
}

func NewClient(token, owner, repo string) *Client {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	// Rate limiter: 5000 requests per hour = ~83 per minute = ~1.4 per second
	// Set to 1 per second to be conservative
	limiter := rate.NewLimiter(rate.Every(time.Second), 1)

	return &Client{
		client:  client,
		owner:   owner,
		repo:    repo,
		limiter: limiter,
	}
}

func (c *Client) GetPullRequests(ctx context.Context, state string) ([]*models.PullRequest, error) {
	var allPRs []*models.PullRequest

	opts := &github.PullRequestListOptions{
		State:     state,
		Sort:      "created",
		Direction: "desc",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	for {
		// Rate limiting
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limiter error: %w", err)
		}

		prs, resp, err := c.client.PullRequests.List(ctx, c.owner, c.repo, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list PRs: %w", err)
		}

		for _, pr := range prs {
			modelPR := convertPR(pr)
			allPRs = append(allPRs, modelPR)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allPRs, nil
}

func (c *Client) GetPRDetails(ctx context.Context, prNumber int) (*models.PullRequest, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter error: %w", err)
	}

	pr, _, err := c.client.PullRequests.Get(ctx, c.owner, c.repo, prNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get PR %d: %w", prNumber, err)
	}

	return convertPR(pr), nil
}

func (c *Client) GetPRCommits(ctx context.Context, prNumber int) ([]models.Commit, error) {
	var allCommits []models.Commit

	opts := &github.ListOptions{
		PerPage: 100,
	}

	for {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limiter error: %w", err)
		}

		commits, resp, err := c.client.PullRequests.ListCommits(ctx, c.owner, c.repo, prNumber, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list commits for PR %d: %w", prNumber, err)
		}

		for _, commit := range commits {
			modelCommit := models.Commit{
				SHA:     commit.GetSHA(),
				Message: commit.GetCommit().GetMessage(),
				URL:     commit.GetHTMLURL(),
			}

			if commit.GetAuthor() != nil {
				modelCommit.Author = convertUser(commit.GetAuthor())
			}
			if commit.GetCommitter() != nil {
				modelCommit.Committer = convertUser(commit.GetCommitter())
			}
			if commit.GetCommit().GetAuthor() != nil {
				modelCommit.Date = commit.GetCommit().GetAuthor().GetDate().Time
			}

			allCommits = append(allCommits, modelCommit)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allCommits, nil
}

func (c *Client) GetPRComments(ctx context.Context, prNumber int) ([]models.Comment, error) {
	var allComments []models.Comment

	// Get issue comments
	issueComments, err := c.getIssueComments(ctx, prNumber)
	if err != nil {
		return nil, err
	}
	allComments = append(allComments, issueComments...)

	// Get review comments
	reviewComments, err := c.getReviewComments(ctx, prNumber)
	if err != nil {
		return nil, err
	}
	allComments = append(allComments, reviewComments...)

	return allComments, nil
}

func (c *Client) getIssueComments(ctx context.Context, prNumber int) ([]models.Comment, error) {
	var allComments []models.Comment

	opts := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	for {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limiter error: %w", err)
		}

		comments, resp, err := c.client.Issues.ListComments(ctx, c.owner, c.repo, prNumber, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list issue comments for PR %d: %w", prNumber, err)
		}

		for _, comment := range comments {
			modelComment := models.Comment{
				ID:        comment.GetID(),
				Body:      comment.GetBody(),
				User:      convertUser(comment.GetUser()),
				CreatedAt: comment.GetCreatedAt().Time,
				UpdatedAt: comment.GetUpdatedAt().Time,
				URL:       comment.GetURL(),
				HTMLURL:   comment.GetHTMLURL(),
				Type:      "issue",
			}
			allComments = append(allComments, modelComment)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allComments, nil
}

func (c *Client) getReviewComments(ctx context.Context, prNumber int) ([]models.Comment, error) {
	var allComments []models.Comment

	opts := &github.PullRequestListCommentsOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	for {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limiter error: %w", err)
		}

		comments, resp, err := c.client.PullRequests.ListComments(ctx, c.owner, c.repo, prNumber, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list review comments for PR %d: %w", prNumber, err)
		}

		for _, comment := range comments {
			modelComment := models.Comment{
				ID:                comment.GetID(),
				Body:              comment.GetBody(),
				User:              convertUser(comment.GetUser()),
				CreatedAt:         comment.GetCreatedAt().Time,
				UpdatedAt:         comment.GetUpdatedAt().Time,
				URL:               comment.GetURL(),
				HTMLURL:           comment.GetHTMLURL(),
				Type:              "review",
				Path:              comment.GetPath(),
				Position:          comment.Position,
				Line:              comment.Line,
				StartLine:         comment.StartLine,
				OriginalPosition:  comment.OriginalPosition,
				OriginalStartLine: comment.OriginalStartLine,
				CommitID:          comment.GetCommitID(),
				OriginalCommitID:  comment.GetOriginalCommitID(),
				DiffHunk:          comment.GetDiffHunk(),
				InReplyToID:       comment.InReplyTo,
			}
			allComments = append(allComments, modelComment)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allComments, nil
}

func (c *Client) GetPRReviews(ctx context.Context, prNumber int) ([]models.Review, error) {
	var allReviews []models.Review

	opts := &github.ListOptions{
		PerPage: 100,
	}

	for {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limiter error: %w", err)
		}

		reviews, resp, err := c.client.PullRequests.ListReviews(ctx, c.owner, c.repo, prNumber, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list reviews for PR %d: %w", prNumber, err)
		}

		for _, review := range reviews {
			modelReview := models.Review{
				ID:          review.GetID(),
				User:        convertUser(review.GetUser()),
				Body:        review.GetBody(),
				State:       review.GetState(),
				HTMLURL:     review.GetHTMLURL(),
				SubmittedAt: review.GetSubmittedAt().Time,
				CommitID:    review.GetCommitID(),
			}
			allReviews = append(allReviews, modelReview)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allReviews, nil
}

func convertPR(pr *github.PullRequest) *models.PullRequest {
	modelPR := &models.PullRequest{
		Number:         pr.GetNumber(),
		Title:          pr.GetTitle(),
		State:          pr.GetState(),
		Body:           pr.GetBody(),
		CreatedAt:      pr.GetCreatedAt().Time,
		UpdatedAt:      pr.GetUpdatedAt().Time,
		User:           convertUser(pr.GetUser()),
		URL:            pr.GetURL(),
		HTMLURL:        pr.GetHTMLURL(),
		CommentsURL:    pr.GetCommentsURL(),
		ReviewComments: pr.GetReviewComments(),
		Comments:       pr.GetComments(),
		Commits:        pr.GetCommits(),
		Additions:      pr.GetAdditions(),
		Deletions:      pr.GetDeletions(),
		ChangedFiles:   pr.GetChangedFiles(),
	}

	if pr.ClosedAt != nil {
		t := pr.ClosedAt.Time
		modelPR.ClosedAt = &t
	}
	if pr.MergedAt != nil {
		t := pr.MergedAt.Time
		modelPR.MergedAt = &t
	}

	if pr.GetBase() != nil {
		modelPR.Base = models.Branch{
			Label: pr.GetBase().GetLabel(),
			Ref:   pr.GetBase().GetRef(),
			SHA:   pr.GetBase().GetSHA(),
		}
	}
	if pr.GetHead() != nil {
		modelPR.Head = models.Branch{
			Label: pr.GetHead().GetLabel(),
			Ref:   pr.GetHead().GetRef(),
			SHA:   pr.GetHead().GetSHA(),
		}
	}

	return modelPR
}

func convertUser(user *github.User) models.User {
	return models.User{
		Login:     user.GetLogin(),
		ID:        user.GetID(),
		AvatarURL: user.GetAvatarURL(),
		HTMLURL:   user.GetHTMLURL(),
		Type:      user.GetType(),
	}
}
