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
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/naoina/toml"
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
	HookPath     string `toml:"hookpath"`
	TplPath      string `toml:"tplpath"`
	AnnounceChan string `toml:"announcechan"`
}

type Factoids struct {
	HookPath string `toml:"hookpath"`
	TplPath  string `toml:"tplpath"`
}

type IRC struct {
	Addr     string
	Nick     string
	Password string
	Channel  []string
}

type AppConfig struct {
	Website
	Debug
	Github
	Factoids
	IRC `toml:"irc"`
}

const sampleconf = `[website]
addr=:80

[debug]
debug=false
logfile=logs/debug.txt

[github]
hookpath=somethingrandom
tplpath=tpl/github.tpl
announcechan="#systemd"

[factoids]
hookpath=/
tplpath=tpl/factoids.tpl

[irc]
addr=irc.freenode.net:6667
nick=sd-bot
password=
channels=["#systemd"]
`

var (
	contextKey   *int
	settingsFile *string
)

func init() {
	contextKey = new(int)
}

func Init(ctx context.Context) context.Context {
	settingsFile = flag.String("config", "settings.cfg", `path to the config file, it it doesn't exist it will
			be created with default values`)
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

	cfg := &AppConfig{}
	if err := ReadConfig(f, cfg); err != nil {
		panic("Failed to parse config file, err: " + err.Error())
	}

	return context.WithValue(ctx, contextKey, cfg)
}

func ReadConfig(r io.Reader, d interface{}) error {
	dec := toml.NewDecoder(r)
	return dec.Decode(d)
}

func WriteConfig(w io.Writer, d interface{}) error {
	enc := toml.NewEncoder(w)
	return enc.Encode(d)
}

func Save(ctx context.Context) error {
	return SafeSave(*settingsFile, *FromContext(ctx))
}

func SafeSave(file string, data interface{}) error {
	dir, err := filepath.Abs(filepath.Dir(file))
	if err != nil {
		return err
	}

	f, err := ioutil.TempFile(dir, "tmpconf-")
	if err != nil {
		return err
	}

	err = WriteConfig(f, data)
	if err != nil {
		return err
	}
	_ = f.Close()

	return os.Rename(f.Name(), file)
}

func FromContext(ctx context.Context) *AppConfig {
	cfg, _ := ctx.Value(contextKey).(*AppConfig)
	return cfg
}
