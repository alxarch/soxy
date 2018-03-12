package soxy

import (
	"io/ioutil"
	"log"
	"net"

	socks5 "github.com/alxarch/go-socks5"

	"github.com/xtaci/smux"
)

// Client is a tunneling client to a socks server
type Client struct {
	config *smux.Config
	socks  *socks5.Server
	logger *log.Logger
}

// NewClient creates a tunnel client to a socks server
func NewClient(s *socks5.Server, config *smux.Config, logger *log.Logger) *Client {
	if config == nil {
		config = smux.DefaultConfig()
	}
	if logger == nil {
		logger = log.New(ioutil.Discard, "[soxy]", log.LstdFlags)
	}
	c := Client{
		socks:  s,
		config: config,
		logger: logger,
	}
	return &c
}

// DialAndListen establishes a connection to a tunnel server and uses forwards incoming streams to a socks server
func (c *Client) DialAndListen(address string) (err error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		log.Println("Failed to dial", address, err.Error())
		return
	}
	mux, err := smux.Server(conn, c.config)
	if err != nil {
		log.Println("Failed to open session", err.Error())
		return
	}
	for {
		stream, err := mux.AcceptStream()
		if err != nil {
			log.Println("Failed to open stream", err.Error())
			return err
		}
		log.Println("New stream", stream.ID())
		go c.socks.ServeConn(stream)

	}
}
