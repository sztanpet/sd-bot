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

// Package sirc is a thin utility library ontop of github.com/sorcix/irc
// it handles the connection to the server and provides a single callback
// for code to interact with
package sirc

import (
	"fmt"
	"math"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/sorcix/irc"
	"github.com/sztanpet/sd-bot/config"
	"github.com/sztanpet/sd-bot/debug"
	"golang.org/x/net/context"
)

var contextKey *int

func init() {
	contextKey = new(int)
}

// Callback is function that is called for every irc command except:
// PING and ERR_NICKNAMEINUSE which are handled by the package
type Callback func(*IConn, *irc.Message) bool

// IConn represents the IRC connection to twitch,
// it is purely "single-threaded", methods are not safe to call concurrently
type IConn struct {
	Callback Callback

	conn net.Conn
	quit chan struct{}
	w    chan *irc.Message

	wg sync.WaitGroup
	*irc.Decoder
	*irc.Encoder
	cfg config.IRC

	mu       sync.Mutex
	Loggedin bool
	// exponentially increase the time we sleep based on the number of tries
	// only resets when successfully connected to the server
	tries float64
	// the number of pings that were sent but not yet answered, should never go
	// beyond 2
	pendingPings int

	// for ratelimiting purposes
	badness  time.Duration
	lastsent time.Time
}

// Init creates a client and assigns it to the context
func Init(ctx context.Context, cb Callback) context.Context {
	c := &IConn{
		Callback: cb,
		cfg:      config.FromContext(ctx).IRC,
		w:        make(chan *irc.Message),
		quit:     make(chan struct{}),
	}
	ctx = context.WithValue(ctx, contextKey, c)
	c.Reconnect("init")

	return ctx
}

// FromContext returns the client from the context
func FromContext(ctx context.Context) *IConn {
	c, _ := ctx.Value(contextKey).(*IConn)
	return c
}

// Reconnect does exactly that and takes a message to be printed as arguments
func (c *IConn) Reconnect(format string, args ...interface{}) {
	c.mu.Lock()

	close(c.quit)
	if c.conn != nil {
		_ = c.conn.Close()
	}
	c.wg.Wait()
	if c.tries > 0 {
		d := time.Duration(math.Pow(2.0, c.tries)*300) * time.Millisecond
		c.logWithDuration(format, d, args...)
		time.Sleep(d)
	}

	c.quit = make(chan struct{})
	conn, err := net.DialTimeout("tcp", c.cfg.Addr, 5*time.Second)
	if err != nil {
		c.mu.Unlock()
		c.addDelay()
		c.Reconnect("conn error: %+v", err)
		return
	}
	defer c.mu.Unlock()

	c.Loggedin = false
	c.pendingPings = 0
	c.badness = 0
	c.conn = conn
	c.Decoder = irc.NewDecoder(conn)
	c.Encoder = irc.NewEncoder(conn)

	c.wg.Add(2)
	go c.write()
	go c.read()

	c.w <- &irc.Message{
		Command: irc.USER,
		Params: []string{
			c.cfg.Nick,
			"0",
			"*",
		},
		Trailing: "github.com/sztanpet/sd-bot",
	}
	c.w <- &irc.Message{Command: irc.NICK, Params: []string{c.cfg.Nick}}
	if len(c.cfg.Password) > 0 {
		c.w <- &irc.Message{Command: irc.PASS, Params: []string{c.cfg.Password}}
	}

}

func (c *IConn) addDelay() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// clamp tries, so that the maximum amount of time we wait is ~5 minutes
	if c.tries > 10.0 {
		c.tries = 10.0
	}

	c.tries++
}

func (c *IConn) logWithDuration(format string, dur time.Duration, args ...interface{}) {
	newargs := make([]interface{}, 0, len(args)+1)
	newargs = append(newargs, args...)
	newargs = append(newargs, dur)
	d.PF(2, format+", reconnecting in %s", newargs...)
}

func (c *IConn) write() {
	defer c.wg.Done()

	for {
		select {
		case <-c.quit:
			return
		case m := <-c.w:
			d.DF(1, "\t> %v", m.String())
			_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := c.Encode(m); err != nil {
				c.addDelay()
				go c.Reconnect("write error: %+v", err)
				return
			}
		}
	}
}

// Write handles sending messages, it reconnects if there are problems
func (c *IConn) Write(m *irc.Message) {
	if t := c.rateLimit(m.Len()); t != 0 {
		<-time.After(t)
	}

	c.w <- m
}

// read handles parsing messages from IRC and reconnects if there are problems
// returns nil on error
func (c *IConn) read() {
	defer c.wg.Done()

	for {
		// if there are pending pings, lower the timeout duration to speed up
		// the disconnection
		if c.pendingPings > 0 {
			_ = c.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		} else {
			_ = c.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		}

		m, err := c.Decode()

		select {
		case <-c.quit:
			return
		default:
		}

		if err == nil {
			// we do not actually care about the type of the message the server sends us,
			// as long as it sends something it signals that its alive
			if c.pendingPings > 0 {
				c.pendingPings--
			}

			switch m.Command {
			case irc.PING:
				c.w <- &irc.Message{Command: irc.PONG, Params: m.Params, Trailing: m.Trailing}
			case irc.ERR_NICKNAMEINUSE:
				c.w <- &irc.Message{
					Command: irc.NICK,
					Params: []string{
						fmt.Sprintf(c.cfg.Nick+"%d", rand.Intn(10)),
					},
				}
			case irc.RPL_WELCOME: // successfully connected
				d.PF(1, "Successfully connected to IRC")
				c.mu.Lock()
				c.Loggedin = true
				c.tries = 0
				c.mu.Unlock()
				fallthrough
			default:
				d.DF(1, "\t< %v", m.String())
				if c.Callback != nil {
					c.Callback(c, m)
				}
			}

			continue
		}

		// if we hit the timeout and there are no outstanding pings, send one
		if e, ok := err.(net.Error); ok && e.Timeout() && c.pendingPings < 1 {
			c.pendingPings++
			c.w <- &irc.Message{
				Command: "PING",
				Params:  []string{"sd-bot"},
			}

			continue
		}

		// otherwise there either was an error or we did not get a reply for our ping
		c.addDelay()
		go c.Reconnect("read error: %+v", err)
		return
	}
}

// rateLimit implements Hybrid's flood control algorithm for outgoing lines.
// Copyright (c) 2009+ Alex Bramley, github.com/fluffle/goirc
func (c *IConn) rateLimit(chars int) time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Hybrid's algorithm allows for 2 seconds per line and an additional
	// 1/120 of a second per character on that line.
	linetime := 2*time.Second + time.Duration(chars)*time.Second/120
	elapsed := time.Now().Sub(c.lastsent)
	if c.badness += linetime - elapsed; c.badness < 0 {
		// negative badness times are badness...
		c.badness = 0
	}
	c.lastsent = time.Now()
	// If we've sent more than 10 second's worth of lines according to the
	// calculation above, then we're at risk of "Excess Flood".
	if c.badness > 10*time.Second {
		return linetime
	}
	return 0
}

// Target returns the appropriate target of an operation based on the message
// if it's a private message, it returns the nick of the person messaging,
// if its a channel message, it returns the channel
func (c *IConn) Target(m *irc.Message) string {
	if len(m.Params) == 0 || len(m.Params[0]) == 0 {
		return ""
	}

	target := m.Params[0]
	// FIXME way too naive of a way to see if we got a private message or not
	if target[0] == '#' {
		return target
	}

	// not a channel message, so its a private message, so return the sender
	return m.Prefix.Name
}

// PrivMsg sends a PRIVMSG to the "appropriate" target as decided by Target
// with the following trailing arguments
func (c *IConn) PrivMsg(m *irc.Message, args ...string) {
	c.Write(&irc.Message{
		Command:  irc.PRIVMSG,
		Params:   []string{c.Target(m)},
		Trailing: strings.Join(args, ""),
	})
}

// Notice sends a NOTICE to the target with the following trailing arguments
func (c *IConn) Notice(m *irc.Message, args ...string) {
	c.Write(&irc.Message{
		Command:  irc.NOTICE,
		Params:   []string{m.Prefix.Name},
		Trailing: strings.Join(args, ""),
	})
}
