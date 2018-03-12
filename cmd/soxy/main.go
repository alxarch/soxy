package main

import (
	"flag"
	"log"
	"os"

	socks5 "github.com/alxarch/go-socks5"
	"github.com/alxarch/soxy"
	"github.com/xtaci/smux"
)

var (
	addr    = ""
	config  = smux.DefaultConfig()
	sconfig = new(socks5.Config)
)

func init() {
	flag.StringVar(&addr, "address", addr, "Dial address")
	flag.IntVar(&config.MaxFrameSize, "max-frame-size", config.MaxFrameSize, "Max frame size")
	flag.IntVar(&config.MaxReceiveBuffer, "max-recv-buffer", config.MaxReceiveBuffer, "Max receive buffer size")
	flag.DurationVar(&config.KeepAliveInterval, "keep-alive-interval", config.KeepAliveInterval, "Keep alive interval")
	flag.DurationVar(&config.KeepAliveTimeout, "keep-alive-timeout", config.KeepAliveTimeout, "Keep alive timeout")

}

func main() {
	flag.Parse()
	logger := log.New(os.Stdout, "[soxy]", log.LstdFlags)
	sconfig.Logger = log.New(os.Stdout, "[socks5]", log.LstdFlags)
	s, err := socks5.New(sconfig)
	if err != nil {
		log.Fatal(err)
	}
	c := soxy.NewClient(s, config, logger)
	logger.Fatal(c.DialAndListen(addr))
}
