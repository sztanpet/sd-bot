package main

import (
	"regexp"
	"sync"
	"time"

	"github.com/sorcix/irc"
	"github.com/sztanpet/sirc"
)

type adminCache struct {
	mu sync.Mutex
	m  map[string]string
}

func (ac *adminCache) init() {
	ac.mu.Lock()
	ac.m = map[string]string{"sztanpet": "sztanpet"}
	ac.mu.Unlock()
}
func (ac *adminCache) Add(nick, user string) {
	ac.mu.Lock()
	ac.m[nick] = user
	ac.mu.Unlock()
}
func (ac *adminCache) Del(nick string) {
	ac.mu.Lock()
	delete(ac.m, nick)
	ac.mu.Unlock()
}
func (ac *adminCache) Get(nick string) (string, bool) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	user, ok := ac.m[nick]
	return user, ok
}

type outstandingAdminRequest struct {
	mu sync.Mutex
	m  map[string]struct {
		ch chan string
		t  time.Time
	}
}

func (o *outstandingAdminRequest) Add(nick string, ch chan string) {
	o.mu.Lock()
	o.m[nick] = struct {
		ch chan string
		t  time.Time
	}{
		ch: ch,
		t:  time.Now(),
	}
	o.mu.Unlock()
}
func (o *outstandingAdminRequest) Get(nick string) (struct {
	ch chan string
	t  time.Time
}, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	ch, ok := o.m[nick]
	return ch, ok
}

func (o *outstandingAdminRequest) Del(nick string) {
	o.mu.Lock()
	delete(o.m, nick)
	o.mu.Unlock()
}

// cleans up outstanding requests where the channel has been closed
func (o *outstandingAdminRequest) cleanup() {
	t := time.NewTicker(time.Minute)
	for {
		o.mu.Lock()
		for n, v := range o.m {
			if v.t.Before(time.Now().Add(-time.Minute)) {
				close(v.ch)
				delete(o.m, n)
			}
		}
		o.mu.Unlock()

		<-t.C
	}
}

var (
	ac     = &adminCache{}
	oar    = &outstandingAdminRequest{}
	infoRE = regexp.MustCompile(`^Information on ([^ ]+) \(account \x02(.*)\x02\)`)
)

func init() {
	ac.init()
	oar.m = map[string]struct {
		ch chan string
		t  time.Time
	}{}
	go oar.cleanup()
}

func handleFreenode(c *sirc.IConn, m *irc.Message) bool {
	if m.Command != irc.QUIT &&
		m.Command != irc.PART &&
		m.Command != irc.NICK &&
		m.Command != "DISCONNECT" &&
		m.Command != "330" &&
		m.Command != "263" &&
		m.Command != irc.NOTICE {
		return false
	}

	switch m.Command {
	case "330": // whois success
		// << whois armin armin
		// >> :wilhelm.freenode.net 330 sztanpet ariscop Phase4 :is logged in as
		v, ok := oar.Get(m.Params[1])
		if ok { // maybe there was a timeout, be sure
			v.ch <- m.Params[2]
		}
	case "263": // whois failed, ask for info from nickserv instead
		// << whois wolfe wolfe
		// >> :wolfe.freenode.net 263 sztanpet WHOIS :This command could not be completed because it has been used recently, and is rate-limited.
		c.Write(&irc.Message{
			Command:  irc.PRIVMSG,
			Params:   []string{"NickServ"},
			Trailing: "info " + m.Prefix.Name,
		})
	case irc.NOTICE: // asked for info from nickserv, it arrived
		// << PRIVMSG NickServer :info armin
		// >> :NickServ!NickServ@services. NOTICE sztanpet :Information on armin (account armin):
		if m.Prefix.Name != "NickServ" {
			return false
		}
		matches := infoRE.FindStringSubmatch(m.Trailing)
		if len(matches) == 0 {
			return false
		}

		v, ok := oar.Get(matches[1])
		if ok {
			v.ch <- matches[2]
		}
	case irc.PART: // invalidate cache
		fallthrough
	case irc.QUIT:
		ac.Del(m.Prefix.Name)
	case "DISCONNECT":
		ac.init() // clear the entire cache
	case irc.NICK:
		user, admin := ac.Get(m.Prefix.Name)
		if admin {
			ac.Del(m.Prefix.Name)
			ac.Add(m.Trailing, user)
		}
	}

	return true
}

func lookupUsername(c *sirc.IConn, m *irc.Message, ch chan string) {
	oar.Add(m.Prefix.Name, ch)
	c.Write(&irc.Message{
		Command: irc.WHOIS,
		Params:  []string{m.Prefix.Name, m.Prefix.Name},
	})
}
