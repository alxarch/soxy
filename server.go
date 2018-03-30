package soxy

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/yamux"
)

// Server is a server for reverse socks tunneling
type Server struct {
	nextID  uint32
	config  *yamux.Config
	logger  *log.Logger
	tunnels map[uint32]*Tunnel
	mu      sync.RWMutex
}

// NewServer creates a new tunnel server
func NewServer(config *yamux.Config, logger *log.Logger) *Server {
	if config == nil {
		config = yamux.DefaultConfig()
	}
	if logger == nil {
		logger = log.New(ioutil.Discard, "[soxy]", log.LstdFlags)
	}
	s := Server{
		config:  config,
		logger:  logger,
		tunnels: make(map[uint32]*Tunnel),
	}
	return &s
}

// Tunnel represents a tcp tunnel
type Tunnel struct {
	id                   uint32
	conn                 net.Conn
	ln                   net.Listener
	mux                  *yamux.Session
	sendBytes, recvBytes int64
	bufferSize           int
}

// Close closes a tunnel
func (t *Tunnel) Close() error {
	return t.mux.Close()
}

// MarshalJSON implents the json.Marshaller interface
func (t *Tunnel) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID            uint32 `json:"id"`
		RemoteAddress string `json:"remote_address"`
		LocalAddress  string `json:"local_address"`
		NumStreams    int    `json:"num_streams"`
		SendBytes     int64  `json:"send_bytes"`
		RecvBytes     int64  `json:"recv_bytes"`
	}{
		t.id,
		t.conn.RemoteAddr().String(),
		t.ln.Addr().String(),
		t.mux.NumStreams(),
		atomic.LoadInt64(&t.sendBytes),
		atomic.LoadInt64(&t.recvBytes),
	})
}

// String implements the Stringer interface
func (t *Tunnel) String() string {
	return fmt.Sprintf("tunnel %d [%s -> %s] (%d)", t.id, t.ln.Addr(), t.conn.RemoteAddr(), t.mux.NumStreams())
}

// Tunnels retutns a snapshot of currently active tunnels
func (s *Server) Tunnels() (ts []*Tunnel) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, t := range s.tunnels {
		ts = append(ts, t)
	}
	return ts
}

// Serve starts serving incoming tunnel connections
func (t *Tunnel) Serve() error {
	// Close the mux in case of listener error
	defer t.mux.Close()
	for {
		conn, err := t.ln.Accept()
		if err != nil {
			return err
		}
		go t.HandleConn(conn)
	}
}

// HandleConn opens a stream ands pipes an incoming connection to it
func (t *Tunnel) HandleConn(conn net.Conn) error {
	defer conn.Close()
	stream, err := t.mux.OpenStream()
	if err != nil {
		return err
	}
	defer stream.Close()
	sendErr, recvErr := make(chan error), make(chan error)
	go pipe(conn, stream, t.bufferSize, &t.sendBytes, sendErr)
	go pipe(stream, conn, t.bufferSize, &t.recvBytes, recvErr)
	select {
	case err = <-recvErr:
	case err = <-sendErr:
	case <-t.mux.CloseChan():
	}
	return err
}

// NewTunnel creates a new Tunnel to conn and starts listening for connections
func (s *Server) NewTunnel(conn net.Conn) (*Tunnel, error) {
	mux, err := yamux.Client(conn, s.config)
	if err != nil {
		return nil, err
	}
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, err
	}
	id := atomic.AddUint32(&s.nextID, 1)

	tun := Tunnel{
		id:         id,
		conn:       conn,
		mux:        mux,
		ln:         ln,
		bufferSize: int(s.config.MaxStreamWindowSize),
	}
	return &tun, nil
}

// ListenAndServe binds the server to an address and serves incoming requests
func (s *Server) ListenAndServe(address string) error {
	ln, err := net.Listen("tcp", address)
	if err != nil {
		s.logger.Println("Failed to listen", err)
		return err
	}
	s.logger.Println("Listening on", ln.Addr())
	for {
		conn, err := ln.Accept()
		if err != nil {
			s.logger.Println("Accept failed", err)
			return err
		}
		go s.HandleConn(conn)

	}
}

// Tunnel gets a tunnel by id
func (s *Server) Tunnel(id uint32) (t *Tunnel) {
	s.mu.RLock()
	t = s.tunnels[id]
	s.mu.RUnlock()
	return
}

// HandleConn creates a new tunnel for an incoming connection
func (s *Server) HandleConn(conn net.Conn) error {
	t, err := s.NewTunnel(conn)
	if err != nil {
		s.logger.Println("Failed to open tunnel", err.Error())
		return err
	}
	go func() {
		// Clean up after tunnel closed
		<-t.mux.CloseChan()
		s.logger.Println("Tunnel closed", t.String())
		s.mu.Lock()
		delete(s.tunnels, t.id)
		s.mu.Unlock()
		t.ln.Close()
		conn.Close()
	}()
	s.mu.Lock()
	s.tunnels[t.id] = t
	s.mu.Unlock()
	s.logger.Println("New tunnel", t.String())
	err = t.Serve()
	return err
}

// QueryParamID is the url query param for tunnel id
const QueryParamID = "id"

// ServeHTTP implements the http.Handler interface
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		tunnels := s.Tunnels()
		data, err := json.Marshal(tunnels)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	case http.MethodDelete:
		q := r.URL.Query()
		for _, v := range q[QueryParamID] {
			if id, err := strconv.ParseUint(v, 10, 32); err == nil {
				if t := s.Tunnel(uint32(id)); t != nil {
					t.Close()
				}
			}
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

var bufPool = new(sync.Pool)

const minBufferSize = 256

func getBuffer(size int) []byte {
	if size < minBufferSize {
		size = minBufferSize
	}
	if buffer, ok := bufPool.Get().([]byte); ok && cap(buffer) >= size {
		return buffer[:size]
	}
	return make([]byte, size)
}

func putBuffer(buffer []byte) {
	bufPool.Put(buffer)
}

func pipe(src io.Reader, dst io.Writer, size int, written *int64, err chan<- error) {
	buf := getBuffer(size)
	defer putBuffer(buf)
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				atomic.AddInt64(written, int64(nw))
			}
			if ew != nil {
				err <- ew
				break
			}
			if nr != nw {
				err <- io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err <- er
			}
			break
		}
	}
	return
}

// TunnelOptions is the config for a Tunnel
type TunnelOptions struct {
	// PipeBufferSize sets the size of the send/recv buffers for piping
	// tunnel connections to a stream
	PipeBufferSize int
	// Mux is the config for stream muxer
	// KeepAliveInterval is how often to perform the keep alive
	KeepAliveInterval time.Duration
	// MaxStreamWindowSize is used to control the maximum
	// window size that we allow for a stream.
	MaxStreamWindowSize uint32
	// AcceptBacklog is used to limit how many streams may be
	// waiting an accept.
	AcceptBacklog int
	// WriteTimeout is meant to be a "safety valve" timeout after
	// we which will suspect a problem with the underlying connection and
	// close it. This is only applied to writes, where's there's generally
	// an expectation that things will move along quickly.
	WriteTimeout time.Duration
}

func (options *TunnelOptions) yamuxConfig() *yamux.Config {
	config := yamux.DefaultConfig()
	if options.WriteTimeout > 0 {
		config.ConnectionWriteTimeout = options.WriteTimeout
	}
	if options.MaxStreamWindowSize > 0 {
		config.MaxStreamWindowSize = options.MaxStreamWindowSize
	}
	if options.AcceptBacklog != 0 {
		config.AcceptBacklog = options.AcceptBacklog
	}
	if options.KeepAliveInterval > 0 {
		config.KeepAliveInterval = options.KeepAliveInterval
	}
	return config
}
