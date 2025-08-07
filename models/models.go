package models

import "time"

type PullRequest struct {
	Number         int        `json:"number"`
	Title          string     `json:"title"`
	State          string     `json:"state"`
	Body           string     `json:"body"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	ClosedAt       *time.Time `json:"closed_at,omitempty"`
	MergedAt       *time.Time `json:"merged_at,omitempty"`
	User           User       `json:"user"`
	Base           Branch     `json:"base"`
	Head           Branch     `json:"head"`
	URL            string     `json:"url"`
	HTMLURL        string     `json:"html_url"`
	CommentsURL    string     `json:"comments_url"`
	ReviewComments int        `json:"review_comments"`
	Comments       int        `json:"comments"`
	Commits        int        `json:"commits"`
	Additions      int        `json:"additions"`
	Deletions      int        `json:"deletions"`
	ChangedFiles   int        `json:"changed_files"`
}

type User struct {
	Login     string `json:"login"`
	ID        int64  `json:"id"`
	AvatarURL string `json:"avatar_url"`
	HTMLURL   string `json:"html_url"`
	Type      string `json:"type"`
}

type Branch struct {
	Label string `json:"label"`
	Ref   string `json:"ref"`
	SHA   string `json:"sha"`
}

type Commit struct {
	SHA       string    `json:"sha"`
	Author    User      `json:"author"`
	Committer User      `json:"committer"`
	Message   string    `json:"message"`
	URL       string    `json:"url"`
	Date      time.Time `json:"date"`
}

type Comment struct {
	ID                int64     `json:"id"`
	Body              string    `json:"body"`
	User              User      `json:"user"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	URL               string    `json:"url"`
	HTMLURL           string    `json:"html_url"`
	Type              string    `json:"type"`                 // issue, review, commit
	Path              string    `json:"path,omitempty"`       // For review comments
	Position          *int      `json:"position,omitempty"`   // For review comments
	Line              *int      `json:"line,omitempty"`       // For review comments
	StartLine         *int      `json:"start_line,omitempty"` // For review comments
	OriginalPosition  *int      `json:"original_position,omitempty"`
	OriginalStartLine *int      `json:"original_start_line,omitempty"`
	CommitID          string    `json:"commit_id,omitempty"`
	OriginalCommitID  string    `json:"original_commit_id,omitempty"`
	DiffHunk          string    `json:"diff_hunk,omitempty"`
	InReplyToID       *int64    `json:"in_reply_to_id,omitempty"`
}

type Review struct {
	ID          int64     `json:"id"`
	User        User      `json:"user"`
	Body        string    `json:"body"`
	State       string    `json:"state"` // APPROVED, CHANGES_REQUESTED, COMMENTED
	HTMLURL     string    `json:"html_url"`
	SubmittedAt time.Time `json:"submitted_at"`
	CommitID    string    `json:"commit_id"`
}

type PRData struct {
	PR       PullRequest `json:"pr"`
	Commits  []Commit    `json:"commits"`
	Comments []Comment   `json:"comments"`
	Reviews  []Review    `json:"reviews"`
}

type Metadata struct {
	LastUpdated time.Time      `json:"last_updated"`
	TotalPRs    int            `json:"total_prs"`
	Repository  string         `json:"repository"`
	Owner       string         `json:"owner"`
	AuthorStats map[string]int `json:"author_stats"` // author -> comment count
}
