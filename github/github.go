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

const maxLines = 5

type gh struct {
	irc *irc.IConn
	tpl *template.Template
}

func Init(ctx context.Context) context.Context {
	t, _ := ctx.Value("maintemplate").(*template.Template)
	cfg := config.GetFromContext(ctx)
	gh := &gh{
		irc: irc.GetFromContext(ctx),
		tpl: template.Must(t.ParseFiles(cfg.Github.TplPath)),
	}

	http.HandleFunc(cfg.Github.HookPath, gh.handler)
	return ctx
}

func (s *gh) handler(w http.ResponseWriter, r *http.Request) {
	d.D("request", r)
	switch r.Header.Get("X-Github-Event") {
	case "push":
		s.pushHandler(r)
	case "gollum":
		s.wikiHandler(r)
	case "pull_request":
		s.prHandler(r)
	case "issues":
		s.issueHandler(r)
	}
}

func handlePayload(r *http.Request, data interface{}) error {
	if r.Header.Get("Content-Type") == "application/json" {
		dec := json.NewDecoder(r.Body)
		return dec.Decode(&data)
	} else {
		payload := r.FormValue("payload")
		return json.Unmarshal([]byte(payload), &data)
	}
}

func (s *gh) pushHandler(r *http.Request) {
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

	err := handlePayload(r, &data)
	if err != nil {
		d.P("Error unmarshaling json:", err)
		return
	}

	pos := strings.LastIndex(data.Ref, "/") + 1
	branch := data.Ref[pos:]
	lines := make([]string, 0, maxLines)
	repo := data.Repository.Name
	repourl := data.Repository.Url
	b := bytes.NewBuffer(nil)

	needSkip := len(data.Commits) > maxLines

	for k, v := range data.Commits {
		firstline := strings.TrimSpace(v.Message)
		pos = strings.Index(firstline, "\n")
		if pos > 0 {
			firstline = strings.TrimSpace(firstline[:pos])
		}

		b.Reset()
		if needSkip && k == len(data.Commits)-maxLines {
			_ = s.tpl.ExecuteTemplate(b, "pushSkipped", &struct {
				Author    string // commits[i].author.username
				FromID    string // commits[0].id
				ToID      string // commits[len -5].id
				SkipCount int
				Repo      string // repository.name
				RepoURL   string // repository.url
			}{
				Author:    v.Author.Username,
				FromID:    data.Commits[0].Id,
				ToID:      v.Id,
				SkipCount: len(data.Commits) - 4,
				Repo:      repo,
				RepoURL:   repourl,
			})
		} else if !needSkip || k > len(data.Commits)-maxLines {
			_ = s.tpl.ExecuteTemplate(b, "push", &struct {
				Author  string // commits[i].author.username
				Url     string // commits[i].url
				Message string // commits[i].message
				ID      string // commits[i].id
				Repo    string // repository.name
				RepoURL string // repository.url
				Branch  string // .ref the part after refs/heads/
			}{
				Author:  v.Author.Username,
				Url:     v.Url,
				Message: firstline,
				ID:      v.Id,
				Repo:    repo,
				RepoURL: repourl,
				Branch:  branch,
			})
		} else {
			continue
		}

		lines = append(lines, b.String())
	}

	for _, line := range lines {
		s.irc.WriteLine(line)
	}
}

func (s *gh) prHandler(r *http.Request) {
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

	err := handlePayload(r, &data)
	if err != nil {
		d.P("Error unmarshaling json:", err)
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

	err := handlePayload(r, &data)
	if err != nil {
		d.P("Error unmarshaling json:", err)
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

	if l := len(lines); l > maxLines {
		lines = lines[l-maxLines:]
	}

	for _, line := range lines {
		s.irc.WriteLine(line)
	}
}

func (s *gh) issueHandler(r *http.Request) {
	var data struct {
		Action string
		Issue  struct {
			Title    string
			Html_url string
			User     struct {
				Login string
			}
		}
	}

	err := handlePayload(r, &data)
	if err != nil {
		d.P("Error unmarshaling json:", err)
		return
	}

	if data.Action != "opened" {
		return
	}

	b := bytes.NewBuffer(nil)
	_ = s.tpl.ExecuteTemplate(b, "issues", &struct {
		Author string
		Title  string
		Url    string
	}{
		Author: data.Issue.User.Login,
		Title:  data.Issue.Title,
		Url:    data.Issue.Html_url,
	})

	s.irc.WriteLine(b.String())
}
