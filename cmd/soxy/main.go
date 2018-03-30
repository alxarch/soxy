package main

import (
	"flag"
	"log"
	"os"

	socks5 "github.com/alxarch/go-socks5"
	"github.com/alxarch/soxy"
	"github.com/hashicorp/yamux"
)

var (
	addr    = ""
	config  = yamux.DefaultConfig()
	sconfig = new(socks5.Config)
)

func init() {
	flag.StringVar(&addr, "address", addr, "Dial address")
	// flag.IntVar(&config.MaxFrameSize, "max-frame-size", config.MaxFrameSize, "Max frame size")
	// flag.IntVar(&config.MaxReceiveBuffer, "max-recv-buffer", config.MaxReceiveBuffer, "Max receive buffer size")
	// flag.DurationVar(&config.KeepAliveInterval, "keep-alive-interval", config.KeepAliveInterval, "Keep alive interval")
	// flag.DurationVar(&config.KeepAliveTimeout, "keep-alive-timeout", config.KeepAliveTimeout, "Keep alive timeout")
	flag.DurationVar(&config.KeepAliveInterval, "keep-alive-interval", config.KeepAliveInterval, "Keep alive interval")
	flag.DurationVar(&config.ConnectionWriteTimeout, "write-timeout", config.ConnectionWriteTimeout, "Connection write timeout")
	flag.IntVar(&config.AcceptBacklog, "accept-backlog", config.AcceptBacklog, "Stream backlog size")
}

func main() {
	flag.Parse()

	logger := log.New(os.Stdout, "[soxy]", log.LstdFlags)
	s, err := socks5.New(sconfig)
	if err != nil {
		log.Fatal(err)
	}
	c := soxy.NewClient(config, logger)
	logger.Fatal(c.DialAndServe(addr, soxy.HandlerFunc(s.ServeConn)))
}
