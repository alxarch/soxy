package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/alxarch/soxy"
	"github.com/xtaci/smux"
)

var (
	addr     = ":0"
	httpAddr = ""
	config   = smux.DefaultConfig()
)

func init() {
	flag.StringVar(&addr, "address", addr, "Listen address")
	flag.StringVar(&httpAddr, "http", httpAddr, "HTTP listen address")
	flag.IntVar(&config.MaxFrameSize, "max-frame-size", config.MaxFrameSize, "Max frame size")
	flag.IntVar(&config.MaxReceiveBuffer, "max-recv-buffer", config.MaxReceiveBuffer, "Max receive buffer size")
	flag.DurationVar(&config.KeepAliveInterval, "keep-alive-interval", config.KeepAliveInterval, "Keep alive interval")
	flag.DurationVar(&config.KeepAliveTimeout, "keep-alive-timeout", config.KeepAliveTimeout, "Keep alive timeout")

}

func main() {
	flag.Parse()
	logger := log.New(os.Stdout, "[soxyd]", log.LstdFlags)
	s := soxy.NewServer(config, logger)
	if httpAddr != "" {
		go func() {
			http.ListenAndServe(httpAddr, s)
		}()
	}
	logger.Fatal(s.ListenAndServe(addr))
}
