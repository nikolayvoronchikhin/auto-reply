package deprecate

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/google/go-github/github"
	"github.com/parkr/auto-reply/ctx"
)

const (
	closedState = `closed`
)

type RepoDeprecation struct {
	// Name with organization, e.g. "jekyll/jekyll-help"
	Nwo string

	// Comment to send when closing the issue.
	Message string
}

type DeprecateHandler struct {
	context  *ctx.Context
	repos    []RepoDeprecation
	messages map[string]string
}

func deprecationsToMap(deprecations []RepoDeprecation) map[string]string {
	deps := map[string]string{}
	for _, dep := range deprecations {
		deps[dep.Nwo] = dep.Message
	}
	return deps
}

// NewHandler returns an HTTP handler which deprecates repositories
// by closing new issues with a comment directing attention elsewhere.
func NewHandler(context *ctx.Context, deprecations []RepoDeprecation) *DeprecateHandler {
	return &DeprecateHandler{
		context:  context,
		repos:    deprecations,
		messages: deprecationsToMap(deprecations),
	}
}

func (dh *DeprecateHandler) HandlePayload(w http.ResponseWriter, r *http.Request, payload []byte) {
	if r.Header.Get("X-GitHub-Event") != "issues" {
		http.Error(w, "ignored this one.", 200)
		return
	}

	var issue github.IssueActivityEvent
	err := json.Unmarshal(payload, &issue)
	if err != nil {
		log.Println("error unmarshalling IssueActivityEvent:", err)
		log.Println("payload:", payload)
		http.Error(w, "bad json", 400)
		return
	}

	if *issue.Action != "opened" {
		http.Error(w, "ignored", 200)
		return
	}

	if msg, ok := dh.messages[*issue.Repo.FullName]; ok {
		err = dh.leaveComment(issue, msg)
		if err != nil {
			log.Println("error leaving comment:", err)
			http.Error(w, "couldnt leave comment", 500)
			return
		}
		err = dh.closeIssue(issue)
		if err != nil {
			log.Println("error closing comment:", err)
			http.Error(w, "couldnt close comment", 500)
			return
		}
	} else {
		log.Printf("looks like '%s' repo isn't deprecated", *issue.Repo.FullName)
		http.Error(w, "non-deprecated repo", 200)
		return
	}

	w.Write([]byte(`sorry ur deprecated`))
}

func (dh *DeprecateHandler) leaveComment(issue github.IssueActivityEvent, msg string) error {
	_, _, err := dh.context.GitHub.Issues.CreateComment(
		*issue.Repo.Owner.Login,
		*issue.Repo.Name,
		*issue.Issue.Number,
		&github.IssueComment{Body: github.String(msg)},
	)
	return err
}

func (dh *DeprecateHandler) closeIssue(issue github.IssueActivityEvent) error {
	_, _, err := dh.context.GitHub.Issues.Edit(
		*issue.Repo.Owner.Login,
		*issue.Repo.Name,
		*issue.Issue.Number,
		&github.IssueRequest{State: github.String(closedState)},
	)
	return err
}
