package smoxy

import (
	"log"
	"os"
	"time"

	socks5 "github.com/alxarch/go-socks5"
	"github.com/alxarch/soxy"
	"github.com/xtaci/smux"
)

type Options struct {
	MaxReceiveBuffer  int
	MaxFrameSize      int
	KeepAliveInterval string
	KeepAliveTimeout  string
	Logging           bool
}

func (options *Options) Config() *smux.Config {
	config := smux.DefaultConfig()
	if options == nil {
		return config
	}
	if options.MaxReceiveBuffer > 0 {
		config.MaxReceiveBuffer = options.MaxReceiveBuffer
	}
	if options.MaxFrameSize > 0 {
		config.MaxFrameSize = options.MaxFrameSize
	}
	if d, err := time.ParseDuration(options.KeepAliveInterval); err != nil {
		config.KeepAliveInterval = d
	}
	if d, err := time.ParseDuration(options.KeepAliveTimeout); err != nil {
		config.KeepAliveTimeout = d
	}
	return config
}

func Dial(address string, options *Options) error {
	config := options.Config()
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
