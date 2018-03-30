// Package smoxy exports soxy.Client to go mobile
package smoxy

import (
	"log"
	"math"
	"os"
	"time"

	socks5 "github.com/alxarch/go-socks5"
	"github.com/alxarch/soxy"
	"github.com/hashicorp/yamux"
)

// Options maps yamux.Config to go mobile compatible values
type Options struct {
	MaxStreamWindowSize    int
	KeepAliveInterval      string
	ConnectionWriteTimeout string
	Logging                bool
}

func (options *Options) config() *yamux.Config {
	config := yamux.DefaultConfig()
	if options == nil {
		return config
	}
	if options.Logging {
		config.LogOutput = os.Stderr
	}
	// if options.MaxReceiveBuffer > 0 {
	// 	config.MaxReceiveBuffer = options.MaxReceiveBuffer
	// }
	if options.MaxStreamWindowSize > 0 && uint32(options.MaxStreamWindowSize) <= math.MaxUint32 {
		config.MaxStreamWindowSize = uint32(options.MaxStreamWindowSize)
	}
	if d, err := time.ParseDuration(options.KeepAliveInterval); err != nil {
		config.KeepAliveInterval = d
	}
	if d, err := time.ParseDuration(options.ConnectionWriteTimeout); err != nil {
		config.ConnectionWriteTimeout = d
	}
	return config
}

// DialSocks is the go mobile interface for setting up a socks reverse tunnel
func DialSocks(address string, options *Options) error {
	config := options.config()
	var logger *log.Logger
	if options != nil && options.Logging {
		logger = log.New(os.Stdout, "[soxy]", log.LstdFlags)
	}
	s, err := socks5.New(nil)
	if err != nil {
		log.Println("Failed to start socks server", err)

		return err
	}
	sx := soxy.NewClient(config, logger)
	return sx.DialAndServe(address, soxy.HandlerFunc(s.ServeConn))
}
