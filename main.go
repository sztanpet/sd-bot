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

package main

import (
	"html"
	"net/http"
	"strings"
	"text/template"
	"time"

	"github.com/sztanpet/sd-bot/config"
	"github.com/sztanpet/sd-bot/debug"
	"github.com/sztanpet/sd-bot/factoids"
	"github.com/sztanpet/sd-bot/github"
	"golang.org/x/net/context"
)

func main() {
	time.Local = time.UTC
	ctx := context.Background()
	ctx = config.Init(ctx)
	ctx = d.Init(ctx)
	ctx = initRootTemplate(ctx)
	ctx = initIRC(ctx)
	ctx = github.Init(ctx)
	ctx = factoids.Init(ctx)

	cfg := config.FromContext(ctx)
	if err := http.ListenAndServe(cfg.Website.Addr, http.DefaultServeMux); err != nil {
		d.F("ListenAndServe:", err)
	}
}

func initRootTemplate(ctx context.Context) context.Context {
	t := template.New("main")
	t.Funcs(template.FuncMap{
		"truncate": func(s string, l int, endstring string) (ret string) {
			if len(s) > l {
				ret = s[0:l-len(endstring)] + endstring
			} else {
				ret = s
			}
			return
		},
		"trim":     strings.TrimSpace,
		"unescape": html.UnescapeString,
	})

	return context.WithValue(ctx, "maintemplate", t)
}
