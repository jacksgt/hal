package irc

import (
	"crypto/tls"
	"fmt"
	"github.com/danryan/env"
	"github.com/danryan/hal"
	"github.com/mattn/go-xmpp"
	"strings"
)

func init() {
	hal.RegisterAdapter("xmpp", New)
}

// adapter struct
type adapter struct {
	hal.BasicAdapter
	username string
	nick     string
	password string
	status   string
	server   string
	port     int
	mode     string
	channels []string //[]string
	noTLS    bool
	conn     *xmpp.Client
}

type config struct {
	User     string `env:"required key=HAL_XMPP_USER"`
	Nick     string `env:"required key=HAL_XMPP_NICK"`
	Password string `env:"key=HAL_XMPP_PASSWORD"`
	Server   string `env:"required key=HAL_XMPP_SERVER"`
	Port     int    `env:"key=HAL_XMPP_PORT default=5222"`
	Channels string `env:"required key=HAL_XMPP_CHANNELS"`
	NoTLS    bool   `env:"key=HAL_XMPP_NO_TLS default=false"`
}

// New returns an initialized adapter
func New(robot *hal.Robot) (hal.Adapter, error) {
	c := &config{}
	env.MustProcess(c)

	a := &adapter{
		username: c.User,
		nick:     c.Nick,
		password: c.Password,
		server:   c.Server,
		port:     c.Port,
		channels: func() []string { return strings.Split(c.Channels, ",") }(),
		noTLS:    c.NoTLS,
	}
	// Set the robot name to the IRC nick so respond commands will work
	a.SetRobot(robot)
	a.Robot.SetName(a.nick)
	return a, nil
}

// Send sends a regular response
func (a *adapter) Send(res *hal.Response, strings ...string) error {
	hal.Logger.Debug("xmpp - sending XMPP response")
	for _, str := range strings {
		a.conn.Send(
			xmpp.Chat{
				Remote: res.Message.Room,
				Type:   "chat",
				Text:   str,
			},
		)
	}
	hal.Logger.Debug("xmpp - sent XMPP response")
	return nil
}

// Reply sends a direct response
func (a *adapter) Reply(res *hal.Response, strings ...string) error {
	newStrings := make([]string, len(strings))
	for _, str := range strings {
		newStrings = append(newStrings, res.UserID()+`: `+str)
	}

	a.Send(res, newStrings...)

	return nil
}

// Emote is not implemented.
func (a *adapter) Emote(res *hal.Response, strings ...string) error {
	return nil
}

// Topic sets the topic
func (a *adapter) Topic(res *hal.Response, strings ...string) error {
	for _, str := range strings {
		a.conn.SendTopic(
			xmpp.Chat{
				Remote: res.Room(),
				Text:   str,
			},
		)
	}
	return nil
}

// Play is not implemented.
func (a *adapter) Play(res *hal.Response, strings ...string) error {
	return nil
}

// Receive forwards a message to the robot
func (a *adapter) Receive(msg *hal.Message) error {
	hal.Logger.Debug("xmpp - adapter received message")
	a.Robot.Receive(msg)
	hal.Logger.Debug("xmpp - adapter sent message to robot")
	return nil
}

// Run starts the adapter
func (a *adapter) Run() error {
	hal.Logger.Debug("xmpp - starting XMPP connection")
	a.startXMPPConnection()
	hal.Logger.Debug("xmpp - started XMPP connection")

	return nil
}

// Stop shuts down the adapter
func (a *adapter) Stop() error {
	hal.Logger.Debug("xmpp - stopping XMPP connection")
	a.stopXMPPConnection()
	hal.Logger.Debug("xmpp - stopped XMPP connection")

	return nil
}

func (a *adapter) newMessage(req *irc.Event) *hal.Message {
	return &hal.Message{
		User: hal.User{
			ID:   req.Nick,
			Name: req.Nick,
		},
		Room: req.Arguments[0],
		Text: req.Message(),
	}
}

type ircPayload struct {
	Channel  string
	Username string
	Text     string
}

func (a *adapter) startIRCConnection() {
	if a.nick == "" {
		a.nick = a.user
	}

	conn := irc.IRC(a.nick, a.user)
	if a.useTLS {
		conn.UseTLS = true
		conn.TLSConfig = &tls.Config{ServerName: a.server}
	}
	conn.Password = a.password
	conn.Debug = (hal.Logger.Level() == 10)

	err := conn.Connect(a.connectionString())
	if err != nil {
		panic("failed to connect to" + err.Error())
	}

	conn.AddCallback("001", func(e *irc.Event) {
		for _, channel := range a.channels {
			conn.Join(channel)
			hal.Logger.Debug("irc - joined " + channel)
		}
	})

	conn.AddCallback("PRIVMSG", func(e *irc.Event) {
		message := a.newMessage(e)
		a.Receive(message)
	})

	a.conn = conn
	hal.Logger.Debug("irc - waiting for server acknowledgement")
	conn.Loop()
}

func (a *adapter) stopIRCConnection() {
	hal.Logger.Debug("xmpp - Stopping connection")
	a.conn.Close()
	hal.Logger.Debug("xmpp - Stopped connection")
}

func (a *adapter) connectionString() string {
	return fmt.Sprintf("%s:%d", a.server, a.port)
}
