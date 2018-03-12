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
	if options.MaxStreamWindowSize > 0 && options.MaxStreamWindowSize <= math.MaxUint32 {
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

// Dial is the go mobile interface
func Dial(address string, options *Options) error {
	config := options.config()
	sconfig := new(socks5.Config)
	var logger *log.Logger
	if options != nil && options.Logging {
		logger = log.New(os.Stdout, "[soxy]", log.LstdFlags)
		sconfig.Logger = log.New(os.Stdout, "[socks]", log.LstdFlags)
	}
	s, err := socks5.New(sconfig)
	if err != nil {
		log.Println("Failed to start socks server", err)

		return err
	}
	sx := soxy.NewClient(s, config, logger)
	return sx.DialAndListen(address)
}
