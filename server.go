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

	"github.com/xtaci/smux"
)

// Server is a server for reverse socks tunneling
type Server struct {
	nextID  uint32
	config  *smux.Config
	logger  *log.Logger
	tunnels map[uint32]*Tunnel
	mu      sync.RWMutex
}

// NewServer creates a new tunnel server
func NewServer(config *smux.Config, logger *log.Logger) *Server {
	if config == nil {
		config = smux.DefaultConfig()
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
	id         uint32
	conn       net.Conn
	mux        *smux.Session
	ln         net.Listener
	bufferSize int
}

// Close closes a tunnel
func (t *Tunnel) Close() {
	t.ln.Close()
	t.mux.Close()
	t.conn.Close()
}

// MarshalJSON implents the json.Marshaller interface
func (t *Tunnel) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID            uint32 `json:"id"`
		RemoteAddress string `json:"remote_address"`
		LocalAddress  string `json:"local_address"`
		NumStreams    int    `json:"num_streams"`
	}{
		t.id,
		t.conn.RemoteAddr().String(),
		t.ln.Addr().String(),
		t.mux.NumStreams(),
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
	for {
		conn, err := t.ln.Accept()
		if err != nil {
			return err
		}
		stream, err := t.mux.OpenStream()
		if err != nil {
			return err
		}
		go pipe(conn, stream, t.bufferSize)
		go pipe(stream, conn, t.bufferSize)
	}
}

// NewTunnel creates a new Tunnel to conn and starts listening for connections
func (s *Server) NewTunnel(conn net.Conn) (*Tunnel, error) {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, err
	}
	mux, err := smux.Server(conn, s.config)
	if err != nil {
		log.Println("Failed to open session", err.Error())
		return nil, err
	}
	tun := Tunnel{
		id:         atomic.AddUint32(&s.nextID, 1),
		conn:       conn,
		mux:        mux,
		ln:         ln,
		bufferSize: int(s.config.MaxFrameSize),
	}
	s.mu.Lock()
	s.tunnels[tun.id] = &tun
	s.mu.Unlock()
	log.Println("New tunnel", tun.String())
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

func pipe(r io.Reader, w io.WriteCloser, size int) (int64, error) {
	defer w.Close()
	buffer := make([]byte, size)
	return io.CopyBuffer(w, r, buffer)
}

// CloseTunnel closes a tunnel
func (s *Server) CloseTunnel(id uint32) {
	s.mu.Lock()
	t, ok := s.tunnels[id]
	if ok {
		defer t.Close()
		delete(s.tunnels, id)
	}
	s.mu.Unlock()
}

// Tunnel gets a tunnel by id
func (s *Server) Tunnel(id uint32) (t *Tunnel) {
	s.mu.RLock()
	t = s.tunnels[id]
	s.mu.RUnlock()
	return
}

// HandleConn creates a new tunnel for an incoming connection
func (s *Server) HandleConn(c net.Conn) error {
	tun, err := s.NewTunnel(c)
	if err != nil {
		s.logger.Println("Failed to open tunnel", err.Error())
		return err
	}
	defer s.CloseTunnel(tun.id)
	err = tun.Serve()
	s.logger.Println("Tunnel closed", tun.String(), err)
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
				s.CloseTunnel(uint32(id))
			}
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
