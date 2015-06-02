/***
  This file is part of sd-bot.

  Copyright (c) 2015 Peter Sztan <sztanpet@gmail.com>

  sd-bot is free software; you can redistribute it and/or modify it
  under the terms of the GNU Lesser General Public License as published by
  the Free Software Foundation; either version 3 of the License, or
  (at your option) any later version.

  sd-bot is distributed in the hope that it will be useful, but
  WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
  Lesser General Public License for more details.

  You should have received a copy of the GNU Lesser General Public License
  along with sd-bot; If not, see <http://www.gnu.org/licenses/>.
***/

package github

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"text/template"

	"github.com/sztanpet/sd-bot/config"
	"github.com/sztanpet/sd-bot/debug"
	"github.com/sztanpet/sd-bot/irc"
	"golang.org/x/net/context"
)

type gh struct {
	irc *irc.IConn
	tpl *template.Template
}

func Init(ctx context.Context) context.Context {
	t, _ := ctx.Value("maintemplate").(*template.Template)
	cfg := config.GetFromContext(ctx)
	gh := &gh{
		irc: ctx.Value("irc").(*irc.IConn),
		tpl: template.Must(t.ParseFiles(cfg.Github.TplPath)),
	}

	http.HandleFunc(cfg.Github.HookPath, gh.handler)
	return ctx
}

func (s *gh) handler(w http.ResponseWriter, r *http.Request) {
	switch r.Header.Get("X-Github-Event") {
	case "push":
		s.pushHandler(r)
	case "gollum":
		s.wikiHandler(r)
	case "pull_request":
		s.prHandler(r)
	}
}

func (s *gh) pushHandler(r *http.Request) {
	payload := r.FormValue("payload")
	if len(payload) == 0 {
		return
	}

	var data struct {
		Ref     string
		Commits []struct {
			Author struct {
				Username string
			}
			Url     string
			Message string
			Id      string
		}
		Repository struct {
			Name string
			Url  string
		}
	}

	err := json.Unmarshal([]byte(payload), &data)
	if err != nil {
		d.P("Error unmarshaling json:", err, "payload was: ", payload)
		return
	}

	pos := strings.LastIndex(data.Ref, "/") + 1
	branch := data.Ref[pos:]
	lines := make([]string, 0, len(data.Commits))
	repo := data.Repository.Name
	repourl := data.Repository.Url
	b := bytes.NewBuffer(nil)

	for _, v := range data.Commits {
		firstline := strings.TrimSpace(v.Message)
		pos = strings.Index(firstline, "\n")
		if pos > 0 {
			firstline = strings.TrimSpace(firstline[:pos])
		}

		b.Reset()
		_ = s.tpl.ExecuteTemplate(b, "push", &struct {
			Author  string // commits[0].author.username
			Url     string // commits[0].url
			Message string // commits[0].message
			ID      string // commits[0].id
			Repo    string // repository.name
			Repourl string // repository.url
			Branch  string // .ref the part after refs/heads/
		}{
			Author:  v.Author.Username,
			Url:     v.Url,
			Message: firstline,
			ID:      v.Id,
			Repo:    repo,
			Repourl: repourl,
			Branch:  branch,
		})
		lines = append(lines, b.String())
	}

	if l := len(lines); l > 5 {
		lines = lines[l-5:]
	}

	for _, line := range lines {
		s.irc.WriteLine(line)
	}
}

func (s *gh) prHandler(r *http.Request) {
	payload := r.FormValue("payload")
	if len(payload) == 0 {
		return
	}

	var data struct {
		Action       string
		Pull_request struct {
			Html_url string
			Title    string
			User     struct {
				Login string
			}
		}
	}

	err := json.Unmarshal([]byte(payload), &data)
	if err != nil {
		d.P("Error unmarshaling json:", err, "payload was: ", payload)
		return
	}

	if data.Action != "opened" {
		return
	}

	b := bytes.NewBuffer(nil)
	_ = s.tpl.ExecuteTemplate(b, "pr", &struct {
		Author string
		Title  string
		Url    string
	}{
		Author: data.Pull_request.User.Login,
		Title:  data.Pull_request.Title,
		Url:    data.Pull_request.Html_url,
	})

	s.irc.WriteLine(b.String())
}

func (s *gh) wikiHandler(r *http.Request) {
	payload := r.FormValue("payload")
	if len(payload) == 0 {
		return
	}

	var data struct {
		Pages []struct {
			Page_name string
			Action    string
			Sha       string
			Html_url  string
		}
		Sender struct {
			Login string
		}
	}

	err := json.Unmarshal([]byte(payload), &data)
	if err != nil {
		d.P("Error unmarshaling json:", err, "payload was: ", payload)
		return
	}

	lines := make([]string, 0, len(data.Pages))
	b := bytes.NewBuffer(nil)
	for _, v := range data.Pages {

		b.Reset()
		_ = s.tpl.ExecuteTemplate(b, "wiki", &struct {
			Author string
			Page   string
			Url    string
			Action string
			Sha    string
		}{
			Author: data.Sender.Login,
			Page:   v.Page_name,
			Url:    v.Html_url,
			Action: v.Action,
			Sha:    v.Sha,
		})
		lines = append(lines, b.String())
	}

	if l := len(lines); l > 5 {
		lines = lines[l-5:]
	}

	for _, line := range lines {
		s.irc.WriteLine(line)
	}
}

/*
X-Github-Event=pull_request
{
  "action": "opened",
  "number": 1,
  "pull_request": {
    "url": "https://api.github.com/repos/baxterthehacker/public-repo/pulls/1",
    "id": 34778301,
    "html_url": "https://github.com/baxterthehacker/public-repo/pull/1",
    "diff_url": "https://github.com/baxterthehacker/public-repo/pull/1.diff",
    "patch_url": "https://github.com/baxterthehacker/public-repo/pull/1.patch",
    "issue_url": "https://api.github.com/repos/baxterthehacker/public-repo/issues/1",
    "number": 1,
    "state": "open",
    "locked": false,
    "title": "Update the README with new information",
    "user": {
      "login": "baxterthehacker",
      "id": 6752317,
      "avatar_url": "https://avatars.githubusercontent.com/u/6752317?v=3",
      "gravatar_id": "",
      "url": "https://api.github.com/users/baxterthehacker",
      "html_url": "https://github.com/baxterthehacker",
      "followers_url": "https://api.github.com/users/baxterthehacker/followers",
      "following_url": "https://api.github.com/users/baxterthehacker/following{/other_user}",
      "gists_url": "https://api.github.com/users/baxterthehacker/gists{/gist_id}",
      "starred_url": "https://api.github.com/users/baxterthehacker/starred{/owner}{/repo}",
      "subscriptions_url": "https://api.github.com/users/baxterthehacker/subscriptions",
      "organizations_url": "https://api.github.com/users/baxterthehacker/orgs",
      "repos_url": "https://api.github.com/users/baxterthehacker/repos",
      "events_url": "https://api.github.com/users/baxterthehacker/events{/privacy}",
      "received_events_url": "https://api.github.com/users/baxterthehacker/received_events",
      "type": "User",
      "site_admin": false
    },
    "body": "This is a pretty simple change that we need to pull into master.",
    "created_at": "2015-05-05T23:40:27Z",
    "updated_at": "2015-05-05T23:40:27Z",
    "closed_at": null,
    "merged_at": null,
    "merge_commit_sha": null,
    "assignee": null,
    "milestone": null,
    "commits_url": "https://api.github.com/repos/baxterthehacker/public-repo/pulls/1/commits",
    "review_comments_url": "https://api.github.com/repos/baxterthehacker/public-repo/pulls/1/comments",
    "review_comment_url": "https://api.github.com/repos/baxterthehacker/public-repo/pulls/comments{/number}",
    "comments_url": "https://api.github.com/repos/baxterthehacker/public-repo/issues/1/comments",
    "statuses_url": "https://api.github.com/repos/baxterthehacker/public-repo/statuses/0d1a26e67d8f5eaf1f6ba5c57fc3c7d91ac0fd1c",
    "head": {
      "label": "baxterthehacker:changes",
      "ref": "changes",
      "sha": "0d1a26e67d8f5eaf1f6ba5c57fc3c7d91ac0fd1c",
      "user": {
        "login": "baxterthehacker",
        "id": 6752317,
        "avatar_url": "https://avatars.githubusercontent.com/u/6752317?v=3",
        "gravatar_id": "",
        "url": "https://api.github.com/users/baxterthehacker",
        "html_url": "https://github.com/baxterthehacker",
        "followers_url": "https://api.github.com/users/baxterthehacker/followers",
        "following_url": "https://api.github.com/users/baxterthehacker/following{/other_user}",
        "gists_url": "https://api.github.com/users/baxterthehacker/gists{/gist_id}",
        "starred_url": "https://api.github.com/users/baxterthehacker/starred{/owner}{/repo}",
        "subscriptions_url": "https://api.github.com/users/baxterthehacker/subscriptions",
        "organizations_url": "https://api.github.com/users/baxterthehacker/orgs",
        "repos_url": "https://api.github.com/users/baxterthehacker/repos",
        "events_url": "https://api.github.com/users/baxterthehacker/events{/privacy}",
        "received_events_url": "https://api.github.com/users/baxterthehacker/received_events",
        "type": "User",
        "site_admin": false
      },
      "repo": {
        "id": 35129377,
        "name": "public-repo",
        "full_name": "baxterthehacker/public-repo",
        "owner": {
          "login": "baxterthehacker",
          "id": 6752317,
          "avatar_url": "https://avatars.githubusercontent.com/u/6752317?v=3",
          "gravatar_id": "",
          "url": "https://api.github.com/users/baxterthehacker",
          "html_url": "https://github.com/baxterthehacker",
          "followers_url": "https://api.github.com/users/baxterthehacker/followers",
          "following_url": "https://api.github.com/users/baxterthehacker/following{/other_user}",
          "gists_url": "https://api.github.com/users/baxterthehacker/gists{/gist_id}",
          "starred_url": "https://api.github.com/users/baxterthehacker/starred{/owner}{/repo}",
          "subscriptions_url": "https://api.github.com/users/baxterthehacker/subscriptions",
          "organizations_url": "https://api.github.com/users/baxterthehacker/orgs",
          "repos_url": "https://api.github.com/users/baxterthehacker/repos",
          "events_url": "https://api.github.com/users/baxterthehacker/events{/privacy}",
          "received_events_url": "https://api.github.com/users/baxterthehacker/received_events",
          "type": "User",
          "site_admin": false
        },
        "private": false,
        "html_url": "https://github.com/baxterthehacker/public-repo",
        "description": "",
        "fork": false,
        "url": "https://api.github.com/repos/baxterthehacker/public-repo",
        "forks_url": "https://api.github.com/repos/baxterthehacker/public-repo/forks",
        "keys_url": "https://api.github.com/repos/baxterthehacker/public-repo/keys{/key_id}",
        "collaborators_url": "https://api.github.com/repos/baxterthehacker/public-repo/collaborators{/collaborator}",
        "teams_url": "https://api.github.com/repos/baxterthehacker/public-repo/teams",
        "hooks_url": "https://api.github.com/repos/baxterthehacker/public-repo/hooks",
        "issue_events_url": "https://api.github.com/repos/baxterthehacker/public-repo/issues/events{/number}",
        "events_url": "https://api.github.com/repos/baxterthehacker/public-repo/events",
        "assignees_url": "https://api.github.com/repos/baxterthehacker/public-repo/assignees{/user}",
        "branches_url": "https://api.github.com/repos/baxterthehacker/public-repo/branches{/branch}",
        "tags_url": "https://api.github.com/repos/baxterthehacker/public-repo/tags",
        "blobs_url": "https://api.github.com/repos/baxterthehacker/public-repo/git/blobs{/sha}",
        "git_tags_url": "https://api.github.com/repos/baxterthehacker/public-repo/git/tags{/sha}",
        "git_refs_url": "https://api.github.com/repos/baxterthehacker/public-repo/git/refs{/sha}",
        "trees_url": "https://api.github.com/repos/baxterthehacker/public-repo/git/trees{/sha}",
        "statuses_url": "https://api.github.com/repos/baxterthehacker/public-repo/statuses/{sha}",
        "languages_url": "https://api.github.com/repos/baxterthehacker/public-repo/languages",
        "stargazers_url": "https://api.github.com/repos/baxterthehacker/public-repo/stargazers",
        "contributors_url": "https://api.github.com/repos/baxterthehacker/public-repo/contributors",
        "subscribers_url": "https://api.github.com/repos/baxterthehacker/public-repo/subscribers",
        "subscription_url": "https://api.github.com/repos/baxterthehacker/public-repo/subscription",
        "commits_url": "https://api.github.com/repos/baxterthehacker/public-repo/commits{/sha}",
        "git_commits_url": "https://api.github.com/repos/baxterthehacker/public-repo/git/commits{/sha}",
        "comments_url": "https://api.github.com/repos/baxterthehacker/public-repo/comments{/number}",
        "issue_comment_url": "https://api.github.com/repos/baxterthehacker/public-repo/issues/comments{/number}",
        "contents_url": "https://api.github.com/repos/baxterthehacker/public-repo/contents/{+path}",
        "compare_url": "https://api.github.com/repos/baxterthehacker/public-repo/compare/{base}...{head}",
        "merges_url": "https://api.github.com/repos/baxterthehacker/public-repo/merges",
        "archive_url": "https://api.github.com/repos/baxterthehacker/public-repo/{archive_format}{/ref}",
        "downloads_url": "https://api.github.com/repos/baxterthehacker/public-repo/downloads",
        "issues_url": "https://api.github.com/repos/baxterthehacker/public-repo/issues{/number}",
        "pulls_url": "https://api.github.com/repos/baxterthehacker/public-repo/pulls{/number}",
        "milestones_url": "https://api.github.com/repos/baxterthehacker/public-repo/milestones{/number}",
        "notifications_url": "https://api.github.com/repos/baxterthehacker/public-repo/notifications{?since,all,participating}",
        "labels_url": "https://api.github.com/repos/baxterthehacker/public-repo/labels{/name}",
        "releases_url": "https://api.github.com/repos/baxterthehacker/public-repo/releases{/id}",
        "created_at": "2015-05-05T23:40:12Z",
        "updated_at": "2015-05-05T23:40:12Z",
        "pushed_at": "2015-05-05T23:40:26Z",
        "git_url": "git://github.com/baxterthehacker/public-repo.git",
        "ssh_url": "git@github.com:baxterthehacker/public-repo.git",
        "clone_url": "https://github.com/baxterthehacker/public-repo.git",
        "svn_url": "https://github.com/baxterthehacker/public-repo",
        "homepage": null,
        "size": 0,
        "stargazers_count": 0,
        "watchers_count": 0,
        "language": null,
        "has_issues": true,
        "has_downloads": true,
        "has_wiki": true,
        "has_pages": true,
        "forks_count": 0,
        "mirror_url": null,
        "open_issues_count": 1,
        "forks": 0,
        "open_issues": 1,
        "watchers": 0,
        "default_branch": "master"
      }
    },
    "base": {
      "label": "baxterthehacker:master",
      "ref": "master",
      "sha": "9049f1265b7d61be4a8904a9a27120d2064dab3b",
      "user": {
        "login": "baxterthehacker",
        "id": 6752317,
        "avatar_url": "https://avatars.githubusercontent.com/u/6752317?v=3",
        "gravatar_id": "",
        "url": "https://api.github.com/users/baxterthehacker",
        "html_url": "https://github.com/baxterthehacker",
        "followers_url": "https://api.github.com/users/baxterthehacker/followers",
        "following_url": "https://api.github.com/users/baxterthehacker/following{/other_user}",
        "gists_url": "https://api.github.com/users/baxterthehacker/gists{/gist_id}",
        "starred_url": "https://api.github.com/users/baxterthehacker/starred{/owner}{/repo}",
        "subscriptions_url": "https://api.github.com/users/baxterthehacker/subscriptions",
        "organizations_url": "https://api.github.com/users/baxterthehacker/orgs",
        "repos_url": "https://api.github.com/users/baxterthehacker/repos",
        "events_url": "https://api.github.com/users/baxterthehacker/events{/privacy}",
        "received_events_url": "https://api.github.com/users/baxterthehacker/received_events",
        "type": "User",
        "site_admin": false
      },
      "repo": {
        "id": 35129377,
        "name": "public-repo",
        "full_name": "baxterthehacker/public-repo",
        "owner": {
          "login": "baxterthehacker",
          "id": 6752317,
          "avatar_url": "https://avatars.githubusercontent.com/u/6752317?v=3",
          "gravatar_id": "",
          "url": "https://api.github.com/users/baxterthehacker",
          "html_url": "https://github.com/baxterthehacker",
          "followers_url": "https://api.github.com/users/baxterthehacker/followers",
          "following_url": "https://api.github.com/users/baxterthehacker/following{/other_user}",
          "gists_url": "https://api.github.com/users/baxterthehacker/gists{/gist_id}",
          "starred_url": "https://api.github.com/users/baxterthehacker/starred{/owner}{/repo}",
          "subscriptions_url": "https://api.github.com/users/baxterthehacker/subscriptions",
          "organizations_url": "https://api.github.com/users/baxterthehacker/orgs",
          "repos_url": "https://api.github.com/users/baxterthehacker/repos",
          "events_url": "https://api.github.com/users/baxterthehacker/events{/privacy}",
          "received_events_url": "https://api.github.com/users/baxterthehacker/received_events",
          "type": "User",
          "site_admin": false
        },
        "private": false,
        "html_url": "https://github.com/baxterthehacker/public-repo",
        "description": "",
        "fork": false,
        "url": "https://api.github.com/repos/baxterthehacker/public-repo",
        "forks_url": "https://api.github.com/repos/baxterthehacker/public-repo/forks",
        "keys_url": "https://api.github.com/repos/baxterthehacker/public-repo/keys{/key_id}",
        "collaborators_url": "https://api.github.com/repos/baxterthehacker/public-repo/collaborators{/collaborator}",
        "teams_url": "https://api.github.com/repos/baxterthehacker/public-repo/teams",
        "hooks_url": "https://api.github.com/repos/baxterthehacker/public-repo/hooks",
        "issue_events_url": "https://api.github.com/repos/baxterthehacker/public-repo/issues/events{/number}",
        "events_url": "https://api.github.com/repos/baxterthehacker/public-repo/events",
        "assignees_url": "https://api.github.com/repos/baxterthehacker/public-repo/assignees{/user}",
        "branches_url": "https://api.github.com/repos/baxterthehacker/public-repo/branches{/branch}",
        "tags_url": "https://api.github.com/repos/baxterthehacker/public-repo/tags",
        "blobs_url": "https://api.github.com/repos/baxterthehacker/public-repo/git/blobs{/sha}",
        "git_tags_url": "https://api.github.com/repos/baxterthehacker/public-repo/git/tags{/sha}",
        "git_refs_url": "https://api.github.com/repos/baxterthehacker/public-repo/git/refs{/sha}",
        "trees_url": "https://api.github.com/repos/baxterthehacker/public-repo/git/trees{/sha}",
        "statuses_url": "https://api.github.com/repos/baxterthehacker/public-repo/statuses/{sha}",
        "languages_url": "https://api.github.com/repos/baxterthehacker/public-repo/languages",
        "stargazers_url": "https://api.github.com/repos/baxterthehacker/public-repo/stargazers",
        "contributors_url": "https://api.github.com/repos/baxterthehacker/public-repo/contributors",
        "subscribers_url": "https://api.github.com/repos/baxterthehacker/public-repo/subscribers",
        "subscription_url": "https://api.github.com/repos/baxterthehacker/public-repo/subscription",
        "commits_url": "https://api.github.com/repos/baxterthehacker/public-repo/commits{/sha}",
        "git_commits_url": "https://api.github.com/repos/baxterthehacker/public-repo/git/commits{/sha}",
        "comments_url": "https://api.github.com/repos/baxterthehacker/public-repo/comments{/number}",
        "issue_comment_url": "https://api.github.com/repos/baxterthehacker/public-repo/issues/comments{/number}",
        "contents_url": "https://api.github.com/repos/baxterthehacker/public-repo/contents/{+path}",
        "compare_url": "https://api.github.com/repos/baxterthehacker/public-repo/compare/{base}...{head}",
        "merges_url": "https://api.github.com/repos/baxterthehacker/public-repo/merges",
        "archive_url": "https://api.github.com/repos/baxterthehacker/public-repo/{archive_format}{/ref}",
        "downloads_url": "https://api.github.com/repos/baxterthehacker/public-repo/downloads",
        "issues_url": "https://api.github.com/repos/baxterthehacker/public-repo/issues{/number}",
        "pulls_url": "https://api.github.com/repos/baxterthehacker/public-repo/pulls{/number}",
        "milestones_url": "https://api.github.com/repos/baxterthehacker/public-repo/milestones{/number}",
        "notifications_url": "https://api.github.com/repos/baxterthehacker/public-repo/notifications{?since,all,participating}",
        "labels_url": "https://api.github.com/repos/baxterthehacker/public-repo/labels{/name}",
        "releases_url": "https://api.github.com/repos/baxterthehacker/public-repo/releases{/id}",
        "created_at": "2015-05-05T23:40:12Z",
        "updated_at": "2015-05-05T23:40:12Z",
        "pushed_at": "2015-05-05T23:40:26Z",
        "git_url": "git://github.com/baxterthehacker/public-repo.git",
        "ssh_url": "git@github.com:baxterthehacker/public-repo.git",
        "clone_url": "https://github.com/baxterthehacker/public-repo.git",
        "svn_url": "https://github.com/baxterthehacker/public-repo",
        "homepage": null,
        "size": 0,
        "stargazers_count": 0,
        "watchers_count": 0,
        "language": null,
        "has_issues": true,
        "has_downloads": true,
        "has_wiki": true,
        "has_pages": true,
        "forks_count": 0,
        "mirror_url": null,
        "open_issues_count": 1,
        "forks": 0,
        "open_issues": 1,
        "watchers": 0,
        "default_branch": "master"
      }
    },
    "_links": {
      "self": {
        "href": "https://api.github.com/repos/baxterthehacker/public-repo/pulls/1"
      },
      "html": {
        "href": "https://github.com/baxterthehacker/public-repo/pull/1"
      },
      "issue": {
        "href": "https://api.github.com/repos/baxterthehacker/public-repo/issues/1"
      },
      "comments": {
        "href": "https://api.github.com/repos/baxterthehacker/public-repo/issues/1/comments"
      },
      "review_comments": {
        "href": "https://api.github.com/repos/baxterthehacker/public-repo/pulls/1/comments"
      },
      "review_comment": {
        "href": "https://api.github.com/repos/baxterthehacker/public-repo/pulls/comments{/number}"
      },
      "commits": {
        "href": "https://api.github.com/repos/baxterthehacker/public-repo/pulls/1/commits"
      },
      "statuses": {
        "href": "https://api.github.com/repos/baxterthehacker/public-repo/statuses/0d1a26e67d8f5eaf1f6ba5c57fc3c7d91ac0fd1c"
      }
    },
    "merged": false,
    "mergeable": null,
    "mergeable_state": "unknown",
    "merged_by": null,
    "comments": 0,
    "review_comments": 0,
    "commits": 1,
    "additions": 1,
    "deletions": 1,
    "changed_files": 1
  },
  "repository": {
    "id": 35129377,
    "name": "public-repo",
    "full_name": "baxterthehacker/public-repo",
    "owner": {
      "login": "baxterthehacker",
      "id": 6752317,
      "avatar_url": "https://avatars.githubusercontent.com/u/6752317?v=3",
      "gravatar_id": "",
      "url": "https://api.github.com/users/baxterthehacker",
      "html_url": "https://github.com/baxterthehacker",
      "followers_url": "https://api.github.com/users/baxterthehacker/followers",
      "following_url": "https://api.github.com/users/baxterthehacker/following{/other_user}",
      "gists_url": "https://api.github.com/users/baxterthehacker/gists{/gist_id}",
      "starred_url": "https://api.github.com/users/baxterthehacker/starred{/owner}{/repo}",
      "subscriptions_url": "https://api.github.com/users/baxterthehacker/subscriptions",
      "organizations_url": "https://api.github.com/users/baxterthehacker/orgs",
      "repos_url": "https://api.github.com/users/baxterthehacker/repos",
      "events_url": "https://api.github.com/users/baxterthehacker/events{/privacy}",
      "received_events_url": "https://api.github.com/users/baxterthehacker/received_events",
      "type": "User",
      "site_admin": false
    },
    "private": false,
    "html_url": "https://github.com/baxterthehacker/public-repo",
    "description": "",
    "fork": false,
    "url": "https://api.github.com/repos/baxterthehacker/public-repo",
    "forks_url": "https://api.github.com/repos/baxterthehacker/public-repo/forks",
    "keys_url": "https://api.github.com/repos/baxterthehacker/public-repo/keys{/key_id}",
    "collaborators_url": "https://api.github.com/repos/baxterthehacker/public-repo/collaborators{/collaborator}",
    "teams_url": "https://api.github.com/repos/baxterthehacker/public-repo/teams",
    "hooks_url": "https://api.github.com/repos/baxterthehacker/public-repo/hooks",
    "issue_events_url": "https://api.github.com/repos/baxterthehacker/public-repo/issues/events{/number}",
    "events_url": "https://api.github.com/repos/baxterthehacker/public-repo/events",
    "assignees_url": "https://api.github.com/repos/baxterthehacker/public-repo/assignees{/user}",
    "branches_url": "https://api.github.com/repos/baxterthehacker/public-repo/branches{/branch}",
    "tags_url": "https://api.github.com/repos/baxterthehacker/public-repo/tags",
    "blobs_url": "https://api.github.com/repos/baxterthehacker/public-repo/git/blobs{/sha}",
    "git_tags_url": "https://api.github.com/repos/baxterthehacker/public-repo/git/tags{/sha}",
    "git_refs_url": "https://api.github.com/repos/baxterthehacker/public-repo/git/refs{/sha}",
    "trees_url": "https://api.github.com/repos/baxterthehacker/public-repo/git/trees{/sha}",
    "statuses_url": "https://api.github.com/repos/baxterthehacker/public-repo/statuses/{sha}",
    "languages_url": "https://api.github.com/repos/baxterthehacker/public-repo/languages",
    "stargazers_url": "https://api.github.com/repos/baxterthehacker/public-repo/stargazers",
    "contributors_url": "https://api.github.com/repos/baxterthehacker/public-repo/contributors",
    "subscribers_url": "https://api.github.com/repos/baxterthehacker/public-repo/subscribers",
    "subscription_url": "https://api.github.com/repos/baxterthehacker/public-repo/subscription",
    "commits_url": "https://api.github.com/repos/baxterthehacker/public-repo/commits{/sha}",
    "git_commits_url": "https://api.github.com/repos/baxterthehacker/public-repo/git/commits{/sha}",
    "comments_url": "https://api.github.com/repos/baxterthehacker/public-repo/comments{/number}",
    "issue_comment_url": "https://api.github.com/repos/baxterthehacker/public-repo/issues/comments{/number}",
    "contents_url": "https://api.github.com/repos/baxterthehacker/public-repo/contents/{+path}",
    "compare_url": "https://api.github.com/repos/baxterthehacker/public-repo/compare/{base}...{head}",
    "merges_url": "https://api.github.com/repos/baxterthehacker/public-repo/merges",
    "archive_url": "https://api.github.com/repos/baxterthehacker/public-repo/{archive_format}{/ref}",
    "downloads_url": "https://api.github.com/repos/baxterthehacker/public-repo/downloads",
    "issues_url": "https://api.github.com/repos/baxterthehacker/public-repo/issues{/number}",
    "pulls_url": "https://api.github.com/repos/baxterthehacker/public-repo/pulls{/number}",
    "milestones_url": "https://api.github.com/repos/baxterthehacker/public-repo/milestones{/number}",
    "notifications_url": "https://api.github.com/repos/baxterthehacker/public-repo/notifications{?since,all,participating}",
    "labels_url": "https://api.github.com/repos/baxterthehacker/public-repo/labels{/name}",
    "releases_url": "https://api.github.com/repos/baxterthehacker/public-repo/releases{/id}",
    "created_at": "2015-05-05T23:40:12Z",
    "updated_at": "2015-05-05T23:40:12Z",
    "pushed_at": "2015-05-05T23:40:26Z",
    "git_url": "git://github.com/baxterthehacker/public-repo.git",
    "ssh_url": "git@github.com:baxterthehacker/public-repo.git",
    "clone_url": "https://github.com/baxterthehacker/public-repo.git",
    "svn_url": "https://github.com/baxterthehacker/public-repo",
    "homepage": null,
    "size": 0,
    "stargazers_count": 0,
    "watchers_count": 0,
    "language": null,
    "has_issues": true,
    "has_downloads": true,
    "has_wiki": true,
    "has_pages": true,
    "forks_count": 0,
    "mirror_url": null,
    "open_issues_count": 1,
    "forks": 0,
    "open_issues": 1,
    "watchers": 0,
    "default_branch": "master"
  },
  "sender": {
    "login": "baxterthehacker",
    "id": 6752317,
    "avatar_url": "https://avatars.githubusercontent.com/u/6752317?v=3",
    "gravatar_id": "",
    "url": "https://api.github.com/users/baxterthehacker",
    "html_url": "https://github.com/baxterthehacker",
    "followers_url": "https://api.github.com/users/baxterthehacker/followers",
    "following_url": "https://api.github.com/users/baxterthehacker/following{/other_user}",
    "gists_url": "https://api.github.com/users/baxterthehacker/gists{/gist_id}",
    "starred_url": "https://api.github.com/users/baxterthehacker/starred{/owner}{/repo}",
    "subscriptions_url": "https://api.github.com/users/baxterthehacker/subscriptions",
    "organizations_url": "https://api.github.com/users/baxterthehacker/orgs",
    "repos_url": "https://api.github.com/users/baxterthehacker/repos",
    "events_url": "https://api.github.com/users/baxterthehacker/events{/privacy}",
    "received_events_url": "https://api.github.com/users/baxterthehacker/received_events",
    "type": "User",
    "site_admin": false
  }
}






X-Github-Event=gollum
{
  "pages": [
    {
      "page_name": "Home",
      "title": "Home",
      "summary": null,
      "action": "created",
      "sha": "91ea1bd42aa2ba166b86e8aefe049e9837214e67",
      "html_url": "https://github.com/baxterthehacker/public-repo/wiki/Home"
    }
  ],
  "repository": {
    "id": 35129377,
    "name": "public-repo",
    "full_name": "baxterthehacker/public-repo",
    "owner": {
      "login": "baxterthehacker",
      "id": 6752317,
      "avatar_url": "https://avatars.githubusercontent.com/u/6752317?v=3",
      "gravatar_id": "",
      "url": "https://api.github.com/users/baxterthehacker",
      "html_url": "https://github.com/baxterthehacker",
      "followers_url": "https://api.github.com/users/baxterthehacker/followers",
      "following_url": "https://api.github.com/users/baxterthehacker/following{/other_user}",
      "gists_url": "https://api.github.com/users/baxterthehacker/gists{/gist_id}",
      "starred_url": "https://api.github.com/users/baxterthehacker/starred{/owner}{/repo}",
      "subscriptions_url": "https://api.github.com/users/baxterthehacker/subscriptions",
      "organizations_url": "https://api.github.com/users/baxterthehacker/orgs",
      "repos_url": "https://api.github.com/users/baxterthehacker/repos",
      "events_url": "https://api.github.com/users/baxterthehacker/events{/privacy}",
      "received_events_url": "https://api.github.com/users/baxterthehacker/received_events",
      "type": "User",
      "site_admin": false
    },
    "private": false,
    "html_url": "https://github.com/baxterthehacker/public-repo",
    "description": "",
    "fork": false,
    "url": "https://api.github.com/repos/baxterthehacker/public-repo",
    "forks_url": "https://api.github.com/repos/baxterthehacker/public-repo/forks",
    "keys_url": "https://api.github.com/repos/baxterthehacker/public-repo/keys{/key_id}",
    "collaborators_url": "https://api.github.com/repos/baxterthehacker/public-repo/collaborators{/collaborator}",
    "teams_url": "https://api.github.com/repos/baxterthehacker/public-repo/teams",
    "hooks_url": "https://api.github.com/repos/baxterthehacker/public-repo/hooks",
    "issue_events_url": "https://api.github.com/repos/baxterthehacker/public-repo/issues/events{/number}",
    "events_url": "https://api.github.com/repos/baxterthehacker/public-repo/events",
    "assignees_url": "https://api.github.com/repos/baxterthehacker/public-repo/assignees{/user}",
    "branches_url": "https://api.github.com/repos/baxterthehacker/public-repo/branches{/branch}",
    "tags_url": "https://api.github.com/repos/baxterthehacker/public-repo/tags",
    "blobs_url": "https://api.github.com/repos/baxterthehacker/public-repo/git/blobs{/sha}",
    "git_tags_url": "https://api.github.com/repos/baxterthehacker/public-repo/git/tags{/sha}",
    "git_refs_url": "https://api.github.com/repos/baxterthehacker/public-repo/git/refs{/sha}",
    "trees_url": "https://api.github.com/repos/baxterthehacker/public-repo/git/trees{/sha}",
    "statuses_url": "https://api.github.com/repos/baxterthehacker/public-repo/statuses/{sha}",
    "languages_url": "https://api.github.com/repos/baxterthehacker/public-repo/languages",
    "stargazers_url": "https://api.github.com/repos/baxterthehacker/public-repo/stargazers",
    "contributors_url": "https://api.github.com/repos/baxterthehacker/public-repo/contributors",
    "subscribers_url": "https://api.github.com/repos/baxterthehacker/public-repo/subscribers",
    "subscription_url": "https://api.github.com/repos/baxterthehacker/public-repo/subscription",
    "commits_url": "https://api.github.com/repos/baxterthehacker/public-repo/commits{/sha}",
    "git_commits_url": "https://api.github.com/repos/baxterthehacker/public-repo/git/commits{/sha}",
    "comments_url": "https://api.github.com/repos/baxterthehacker/public-repo/comments{/number}",
    "issue_comment_url": "https://api.github.com/repos/baxterthehacker/public-repo/issues/comments{/number}",
    "contents_url": "https://api.github.com/repos/baxterthehacker/public-repo/contents/{+path}",
    "compare_url": "https://api.github.com/repos/baxterthehacker/public-repo/compare/{base}...{head}",
    "merges_url": "https://api.github.com/repos/baxterthehacker/public-repo/merges",
    "archive_url": "https://api.github.com/repos/baxterthehacker/public-repo/{archive_format}{/ref}",
    "downloads_url": "https://api.github.com/repos/baxterthehacker/public-repo/downloads",
    "issues_url": "https://api.github.com/repos/baxterthehacker/public-repo/issues{/number}",
    "pulls_url": "https://api.github.com/repos/baxterthehacker/public-repo/pulls{/number}",
    "milestones_url": "https://api.github.com/repos/baxterthehacker/public-repo/milestones{/number}",
    "notifications_url": "https://api.github.com/repos/baxterthehacker/public-repo/notifications{?since,all,participating}",
    "labels_url": "https://api.github.com/repos/baxterthehacker/public-repo/labels{/name}",
    "releases_url": "https://api.github.com/repos/baxterthehacker/public-repo/releases{/id}",
    "created_at": "2015-05-05T23:40:12Z",
    "updated_at": "2015-05-05T23:40:12Z",
    "pushed_at": "2015-05-05T23:40:17Z",
    "git_url": "git://github.com/baxterthehacker/public-repo.git",
    "ssh_url": "git@github.com:baxterthehacker/public-repo.git",
    "clone_url": "https://github.com/baxterthehacker/public-repo.git",
    "svn_url": "https://github.com/baxterthehacker/public-repo",
    "homepage": null,
    "size": 0,
    "stargazers_count": 0,
    "watchers_count": 0,
    "language": null,
    "has_issues": true,
    "has_downloads": true,
    "has_wiki": true,
    "has_pages": true,
    "forks_count": 0,
    "mirror_url": null,
    "open_issues_count": 0,
    "forks": 0,
    "open_issues": 0,
    "watchers": 0,
    "default_branch": "master"
  },
  "sender": {
    "login": "jasonrudolph",
    "id": 2988,
    "avatar_url": "https://avatars.githubusercontent.com/u/2988?v=3",
    "gravatar_id": "",
    "url": "https://api.github.com/users/jasonrudolph",
    "html_url": "https://github.com/jasonrudolph",
    "followers_url": "https://api.github.com/users/jasonrudolph/followers",
    "following_url": "https://api.github.com/users/jasonrudolph/following{/other_user}",
    "gists_url": "https://api.github.com/users/jasonrudolph/gists{/gist_id}",
    "starred_url": "https://api.github.com/users/jasonrudolph/starred{/owner}{/repo}",
    "subscriptions_url": "https://api.github.com/users/jasonrudolph/subscriptions",
    "organizations_url": "https://api.github.com/users/jasonrudolph/orgs",
    "repos_url": "https://api.github.com/users/jasonrudolph/repos",
    "events_url": "https://api.github.com/users/jasonrudolph/events{/privacy}",
    "received_events_url": "https://api.github.com/users/jasonrudolph/received_events",
    "type": "User",
    "site_admin": true
  }
}





X-Github-Event=push
{
   "after":"1481a2de7b2a7d02428ad93446ab166be7793fbb",
   "before":"17c497ccc7cca9c2f735aa07e9e3813060ce9a6a",
   "commits":[
      {
         "added":[

         ],
         "author":{
            "email":"lolwut@noway.biz",
            "name":"Garen Torikian",
            "username":"octokitty"
         },
         "committer":{
            "email":"lolwut@noway.biz",
            "name":"Garen Torikian",
            "username":"octokitty"
         },
         "distinct":true,
         "id":"c441029cf673f84c8b7db52d0a5944ee5c52ff89",
         "message":"Test",
         "modified":[
            "README.md"
         ],
         "removed":[

         ],
         "timestamp":"2013-02-22T13:50:07-08:00",
         "url":"https://github.com/octokitty/testing/commit/c441029cf673f84c8b7db52d0a5944ee5c52ff89"
      },
      {
         "added":[

         ],
         "author":{
            "email":"lolwut@noway.biz",
            "name":"Garen Torikian",
            "username":"octokitty"
         },
         "committer":{
            "email":"lolwut@noway.biz",
            "name":"Garen Torikian",
            "username":"octokitty"
         },
         "distinct":true,
         "id":"36c5f2243ed24de58284a96f2a643bed8c028658",
         "message":"This is me testing the windows client.",
         "modified":[
            "README.md"
         ],
         "removed":[

         ],
         "timestamp":"2013-02-22T14:07:13-08:00",
         "url":"https://github.com/octokitty/testing/commit/36c5f2243ed24de58284a96f2a643bed8c028658"
      },
      {
         "added":[
            "words/madame-bovary.txt"
         ],
         "author":{
            "email":"lolwut@noway.biz",
            "name":"Garen Torikian",
            "username":"octokitty"
         },
         "committer":{
            "email":"lolwut@noway.biz",
            "name":"Garen Torikian",
            "username":"octokitty"
         },
         "distinct":true,
         "id":"1481a2de7b2a7d02428ad93446ab166be7793fbb",
         "message":"Rename madame-bovary.txt to words/madame-bovary.txt",
         "modified":[

         ],
         "removed":[
            "madame-bovary.txt"
         ],
         "timestamp":"2013-03-12T08:14:29-07:00",
         "url":"https://github.com/octokitty/testing/commit/1481a2de7b2a7d02428ad93446ab166be7793fbb"
      }
   ],
   "compare":"https://github.com/octokitty/testing/compare/17c497ccc7cc...1481a2de7b2a",
   "created":false,
   "deleted":false,
   "forced":false,
   "head_commit":{
      "added":[
         "words/madame-bovary.txt"
      ],
      "author":{
         "email":"lolwut@noway.biz",
         "name":"Garen Torikian",
         "username":"octokitty"
      },
      "committer":{
         "email":"lolwut@noway.biz",
         "name":"Garen Torikian",
         "username":"octokitty"
      },
      "distinct":true,
      "id":"1481a2de7b2a7d02428ad93446ab166be7793fbb",
      "message":"Rename madame-bovary.txt to words/madame-bovary.txt",
      "modified":[

      ],
      "removed":[
         "madame-bovary.txt"
      ],
      "timestamp":"2013-03-12T08:14:29-07:00",
      "url":"https://github.com/octokitty/testing/commit/1481a2de7b2a7d02428ad93446ab166be7793fbb"
   },
   "pusher":{
      "email":"lolwut@noway.biz",
      "name":"Garen Torikian"
   },
   "ref":"refs/heads/master",
   "repository":{
      "created_at":1332977768,
      "description":"",
      "fork":false,
      "forks":0,
      "has_downloads":true,
      "has_issues":true,
      "has_wiki":true,
      "homepage":"",
      "id":3860742,
      "language":"Ruby",
      "master_branch":"master",
      "name":"testing",
      "open_issues":2,
      "owner":{
         "email":"lolwut@noway.biz",
         "name":"octokitty"
      },
      "private":false,
      "pushed_at":1363295520,
      "size":2156,
      "stargazers":1,
      "url":"https://github.com/octokitty/testing",
      "watchers":1
   }
}
*/
