package main

import (
	"regexp"
	"strings"
	"time"

	"github.com/sorcix/irc"
	"github.com/sztanpet/sd-bot/config"
	"github.com/sztanpet/sd-bot/debug"
	"github.com/sztanpet/sd-bot/factoids"
	"github.com/sztanpet/sd-bot/persist"
	"github.com/sztanpet/sd-bot/sirc"
	"golang.org/x/net/context"
)

var (
	adminState *persist.State
	admins     map[string]struct{}
	adminRE    = regexp.MustCompile(`^\.(addadmin|deladmin|raw)\s+(.*)$`)
)

func initIRC(ctx context.Context) context.Context {
	adminRE.Longest()

	var err error
	adminState, err = persist.New("admins.state", &map[string]struct{}{
		"sztanpet": struct{}{},
	})
	if err != nil {
		d.F(err.Error())
	}

	admins = *adminState.Get().(*map[string]struct{})
	ctx = sirc.Init(ctx, func(c *sirc.IConn, m *irc.Message) bool {
		return handleIRC(ctx, c, m)
	})

	return ctx
}

func handleIRC(ctx context.Context, c *sirc.IConn, m *irc.Message) bool {
	if m.Command == irc.RPL_WELCOME {
		cfg := config.FromContext(ctx)
		c.Write(&irc.Message{
			Command:  irc.PRIVMSG,
			Params:   []string{"NickServ"},
			Trailing: "identify " + cfg.Nickserv.Password,
		})
		c.Write(&irc.Message{
			Command: irc.MODE,
			Params:  []string{cfg.IRC.Nick, "+R"},
		})
		c.Write(&irc.Message{Command: irc.JOIN, Params: []string{"#systemd"}})
		return false
	}

	if handleFreenode(c, m) {
		return true
	}

	if m.Command != irc.PRIVMSG {
		return false
	}

	if factoids.Handle(c, m) {
		return true
	}

	if m.Trailing[0] == '.' {
		go checkAdmin(c, m)
		return true
	}

	return false
}

func checkAdmin(c *sirc.IConn, m *irc.Message) {
	ch := make(chan string, 1)
	if u, ok := ac.Get(m.Prefix.Name); ok {
		ch <- u
	} else {
		lookupUsername(c, m, ch)
	}

	go (func() {
		user, ok := <-ch
		if !ok { // channel got closed due to timeout
			return
		}

		adminState.Lock()
		_, admin := admins[user]
		adminState.Unlock()
		if !admin {
			return
		}

		if factoids.HandleAdmin(c, m) {
			return
		}

		handleAdmin(c, m)
	})()

	// make sure the channel is closed and the goroutine is cleaned up no matter what
	time.Sleep(time.Minute)
	close(ch)
}

func handleAdmin(c *sirc.IConn, m *irc.Message) bool {
	matches := adminRE.FindStringSubmatch(m.Trailing)
	if len(matches) == 0 {
		return false
	}
	adminState.Lock()
	// lifo defer order
	defer adminState.Save()
	defer adminState.Unlock()

	user := strings.TrimSpace(matches[2])
	switch matches[1] {
	case "addadmin":
		admins[user] = struct{}{}
		c.Notice(m, "Added user successfully")
	case "deladmin":
		delete(admins, user)
		c.Notice(m, "Removed user successfully")
	case "raw":
		nm := irc.ParseMessage(matches[2])
		if nm == nil {
			c.Notice(m, "Could not parse, are you sure you know the irc protocol?")
		} else {
			go c.Write(nm)
		}
	}

	return true
}
