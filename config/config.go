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

package config

import (
	"flag"
	"io"
	"os"

	"code.google.com/p/gcfg"
	"golang.org/x/net/context"
)

type Website struct {
	Addr string
}

type Debug struct {
	Debug   bool
	Logfile string
}

type Github struct {
	HookPath string
	TplPath  string
}

type IRC struct {
	Addr     string
	Nick     string
	Password string
	Channel  string
}

type AppConfig struct {
	Website
	Debug
	Github
	IRC
}

var settingsFile = flag.String("config", "settings.cfg", `path to the config file, it it doesn't exist it will
		be created with default values`)

const sampleconf = `[website]
addr=:80

[debug]
debug=no
logfile=logs/debug.txt

[github]
hookpath=somethingrandom
tplpath=tpl/github.tpl

[irc]
addr=irc.freenode.net:6667
nick=sd-bot
password=
channel=systemd
`

func Init(ctx context.Context) context.Context {
	flag.Parse()
	f, err := os.OpenFile(*settingsFile, os.O_CREATE|os.O_RDWR, 0660)
	if err != nil {
		panic("Could not open " + *settingsFile + " err: " + err.Error())
	}
	defer f.Close()

	// empty? initialize it
	if info, err := f.Stat(); err == nil && info.Size() == 0 {
		io.WriteString(f, sampleconf)
		f.Seek(0, 0)
	}

	cfg := ReadConfig(f)
	return context.WithValue(ctx, "appconfig", cfg)
}

func ReadConfig(f *os.File) *AppConfig {
	ret := &AppConfig{}
	if err := gcfg.ReadInto(ret, f); err != nil {
		panic("Failed to parse config file, err: " + err.Error())
	}

	return ret
}

func GetFromContext(ctx context.Context) *AppConfig {
	cfg, _ := ctx.Value("appconfig").(*AppConfig)
	return cfg
}
