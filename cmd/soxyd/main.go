package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/alxarch/soxy"
	"github.com/hashicorp/yamux"
)

var (
	addr     = ":0"
	httpAddr = ""
	config   = yamux.DefaultConfig()
)

func init() {
	flag.StringVar(&addr, "address", addr, "Listen address")
	flag.StringVar(&httpAddr, "http", httpAddr, "HTTP listen address")
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
	logger := log.New(os.Stdout, "[soxyd]", log.LstdFlags)
	s := soxy.NewServer(config, logger)
	if httpAddr != "" {
		go func() {
			http.ListenAndServe(httpAddr, s)
		}()
	}
	logger.Fatal(s.ListenAndServe(addr))
}
