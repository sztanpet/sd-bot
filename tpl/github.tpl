{{define "push"}}[{{.Repo}}|{{.Author}}] {{truncate .Message 200 "..."}} {{.Repourl}}/commit/{{truncate .ID 7 ""}}{{end}}
{{define "pr"}}[GH PR|{{.Author}}] {{.Title | unescape}} {{.Url | unescape}}{{end}}
{{define "wiki"}}[GH Wiki|{{.Author}}] {{.Page | unescape}} {{.Action}} {{.Url | unescape}}{{if ne .Action "created"}}/_compare/{{truncate .Sha 7 ""}}%5E...{{truncate .Sha 7 ""}}{{end}}{{end}}
{{define "issues"}}[GH Issue|{{.Author}}] {{.Title | unescape}} {{.Url | unescape}}{{end}}
