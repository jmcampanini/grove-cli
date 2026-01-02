package github

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type PRState string

const (
	PRStateOpen   PRState = "OPEN"
	PRStateClosed PRState = "CLOSED"
	PRStateMerged PRState = "MERGED"
	PRStateDraft  PRState = "DRAFT" // Virtual state: GitHub returns OPEN + isDraft=true
)

func (s PRState) String() string {
	return string(s)
}

func (s PRState) IsValid() bool {
	switch s {
	case PRStateOpen, PRStateClosed, PRStateMerged, PRStateDraft:
		return true
	}
	return false
}

// PRQuery specifies filters for listing pull requests.
// TODO: add a ignore-users field, and thread it through from config
// TODO: add default updated within days from config
type PRQuery struct {
	ClosedWithinDays  int     // 0 = no filter, uses closed:>= in search
	MergedWithinDays  int     // 0 = no filter, uses merged:>= in search
	State             PRState // Defaults to PRStateOpen if empty
	UpdatedWithinDays int     // 0 = no filter, uses updated:>= in search
}

// ToSearchQuery converts the query to a GitHub search string for use with `gh pr list --search`.
func (q PRQuery) ToSearchQuery() string {
	state := q.State
	if state == "" {
		state = PRStateOpen
	}

	var parts []string

	switch state {
	case PRStateOpen:
		parts = append(parts, "is:pr", "is:open", "draft:false")
	case PRStateDraft:
		parts = append(parts, "is:pr", "is:open", "draft:true")
	case PRStateClosed:
		parts = append(parts, "is:pr", "is:closed", "is:unmerged")
	case PRStateMerged:
		parts = append(parts, "is:pr", "is:merged")
	}

	if q.ClosedWithinDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -q.ClosedWithinDays)
		parts = append(parts, fmt.Sprintf("closed:>=%s", cutoff.Format("2006-01-02")))
	}

	if q.MergedWithinDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -q.MergedWithinDays)
		parts = append(parts, fmt.Sprintf("merged:>=%s", cutoff.Format("2006-01-02")))
	}

	if q.UpdatedWithinDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -q.UpdatedWithinDays)
		parts = append(parts, fmt.Sprintf("updated:>=%s", cutoff.Format("2006-01-02")))
	}

	return strings.Join(parts, " ")
}

type PullRequest struct {
	Number       int
	BranchName   string
	State        PRState
	Title        string
	AuthorLogin  string
	AuthorName   string // May be empty if user hasn't set display name
	Body         string
	CreatedAt    time.Time
	LinesAdded   int
	LinesDeleted int
	FilesChanged int
}

const prJsonFields = "number,headRefName,state,isDraft,title,author,body,createdAt,additions,deletions,changedFiles"

func (pr *PullRequest) UnmarshalJSON(data []byte) error {
	type rawPR struct {
		Number       int       `json:"number"`
		HeadRefName  string    `json:"headRefName"`
		State        string    `json:"state"`
		IsDraft      bool      `json:"isDraft"`
		Title        string    `json:"title"`
		Body         string    `json:"body"`
		CreatedAt    time.Time `json:"createdAt"`
		Additions    int       `json:"additions"`
		Deletions    int       `json:"deletions"`
		ChangedFiles int       `json:"changedFiles"`
		Author       struct {
			Login string `json:"login"`
			Name  string `json:"name"`
		} `json:"author"`
	}
	var raw rawPR
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	pr.Number = raw.Number
	pr.BranchName = raw.HeadRefName
	pr.Title = raw.Title
	pr.Body = raw.Body
	pr.CreatedAt = raw.CreatedAt
	pr.LinesAdded = raw.Additions
	pr.LinesDeleted = raw.Deletions
	pr.FilesChanged = raw.ChangedFiles
	pr.AuthorLogin = raw.Author.Login
	pr.AuthorName = raw.Author.Name

	if raw.IsDraft && raw.State == "OPEN" {
		pr.State = PRStateDraft
	} else {
		switch raw.State {
		case "OPEN":
			pr.State = PRStateOpen
		case "CLOSED":
			pr.State = PRStateClosed
		case "MERGED":
			pr.State = PRStateMerged
		default:
			return fmt.Errorf("unknown PR state: %s", raw.State)
		}
	}

	return nil
}
