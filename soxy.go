package soxy

import (
	"io/ioutil"
	"log"
	"net"

	"github.com/hashicorp/yamux"
)

// Handler is a connection handler
type Handler interface {
	HandleConn(conn net.Conn) error
}

// HandlerFunc is a closure Handler
type HandlerFunc func(conn net.Conn) error

// HandleConn implements Handler interface
func (fn HandlerFunc) HandleConn(conn net.Conn) error {
	return fn(conn)
}

// Client is a tunneling client to a socks server
type Client struct {
	config *yamux.Config
	logger *log.Logger
}

// NewClient creates a tunnel client
func NewClient(config *yamux.Config, logger *log.Logger) *Client {
	if config == nil {
		config = yamux.DefaultConfig()
	}
	if logger == nil {
		logger = log.New(ioutil.Discard, "[soxy]", log.LstdFlags)
	}
	c := Client{
		config: config,
		logger: logger,
	}
	return &c
}

// DialAndServe establishes a connection to a tunnel server and uses forwards incoming streams to a socks server
func (c *Client) DialAndServe(address string, handler Handler) (err error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		c.logger.Println("Failed to dial", address, err.Error())
		return
	}
	mux, err := yamux.Server(conn, c.config)
	if err != nil {
		c.logger.Println("Failed to open session", err.Error())
		return
	}
	for {
		stream, err := mux.AcceptStream()
		if err != nil {
			c.logger.Println("Failed to open stream", err.Error())
			return err
		}

		c.logger.Println("New stream", stream.StreamID())
		go handler.HandleConn(stream)
	}
}
