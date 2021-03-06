{{define "push"}}[{{.Repo}}|{{.Author}}] {{truncate .Message 200 "..."}} {{.RepoURL}}/commit/{{truncate .ID 7 ""}}{{end}}
{{define "pushSkipped"}}[{{.Repo}}|{{.Author}}] Skipping announcement of {{.SkipCount}} commits: {{.RepoURL}}/compare/{{truncate .FromID 7 ""}}...{{truncate .ToID 7 ""}}{{end}}
{{define "pr"}}[GH PR|{{.Author}}] {{.Title | unescape}} {{.URL | unescape}}{{end}}
{{define "wiki"}}[GH Wiki|{{.Author}}] {{.Page | unescape}} {{.Action}} {{.URL | unescape}}{{if ne .Action "created"}}/_compare/{{truncate .Sha 7 ""}}%5E...{{truncate .Sha 7 ""}}{{end}}{{end}}
{{define "issues"}}[GH Issue|{{.Author}}] {{.Title | unescape}} {{.URL | unescape}}{{end}}
