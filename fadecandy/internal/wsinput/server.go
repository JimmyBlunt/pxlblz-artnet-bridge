package wsinput

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const guid = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

func acceptForKey(key string) string {
	h := sha1.Sum([]byte(key + guid))
	return base64.StdEncoding.EncodeToString(h[:])
}

type Stats struct {
	Connections uint64
	Active      int64
	Binary      uint64
	ProtocolErr uint64
}
type Server struct {
	listen, path string
	maxPayload   int
	onBinary     func([]byte)
	httpServer   *http.Server
	listener     net.Listener
	mu           sync.Mutex
	conns        map[net.Conn]struct{}
	connections  atomic.Uint64
	active       atomic.Int64
	binary       atomic.Uint64
	protocolErr  atomic.Uint64
}

func New(listen, path string, maxPayload int, onBinary func([]byte)) (*Server, error) {
	if listen == "" {
		listen = "127.0.0.1:9981"
	}
	if path == "" {
		path = "/pixels"
	}
	if !strings.HasPrefix(path, "/") {
		return nil, fmt.Errorf("websocket path must begin with /")
	}
	if maxPayload <= 0 {
		return nil, fmt.Errorf("max payload must be > 0")
	}
	if onBinary == nil {
		return nil, fmt.Errorf("onBinary required")
	}
	return &Server{listen: listen, path: path, maxPayload: maxPayload, onBinary: onBinary, conns: map[net.Conn]struct{}{}}, nil
}
func (s *Server) Start() (string, error) {
	ln, err := net.Listen("tcp", s.listen)
	if err != nil {
		return "", err
	}
	s.listener = ln
	mux := http.NewServeMux()
	mux.HandleFunc(s.path, s.handle)
	s.httpServer = &http.Server{Handler: mux}
	go func() { _ = s.httpServer.Serve(ln) }()
	return ln.Addr().String(), nil
}
func (s *Server) Close() error {
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		_ = s.httpServer.Shutdown(ctx)
		cancel()
	}
	s.mu.Lock()
	for c := range s.conns {
		_ = c.Close()
	}
	s.conns = map[net.Conn]struct{}{}
	s.mu.Unlock()
	return nil
}
func (s *Server) Stats() Stats {
	return Stats{Connections: s.connections.Load(), Active: s.active.Load(), Binary: s.binary.Load(), ProtocolErr: s.protocolErr.Load()}
}
func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") || !hasToken(r.Header.Get("Connection"), "upgrade") {
		http.Error(w, "websocket upgrade required", 426)
		return
	}
	key := strings.TrimSpace(r.Header.Get("Sec-WebSocket-Key"))
	if key == "" || r.Header.Get("Sec-WebSocket-Version") != "13" {
		http.Error(w, "bad websocket handshake", 400)
		return
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking unsupported", 500)
		return
	}
	conn, rw, err := hj.Hijack()
	if err != nil {
		return
	}
	if _, err = fmt.Fprintf(rw, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", acceptForKey(key)); err != nil {
		_ = conn.Close()
		return
	}
	if err = rw.Flush(); err != nil {
		_ = conn.Close()
		return
	}
	s.mu.Lock()
	s.conns[conn] = struct{}{}
	s.mu.Unlock()
	s.connections.Add(1)
	s.active.Add(1)
	defer func() { s.active.Add(-1); s.mu.Lock(); delete(s.conns, conn); s.mu.Unlock(); _ = conn.Close() }()
	for {
		op, p, err := readFrame(rw.Reader, true, s.maxPayload)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				s.protocolErr.Add(1)
			}
			return
		}
		switch op {
		case 0x2:
			s.binary.Add(1)
			s.onBinary(p)
		case 0x8:
			_ = writeFrame(conn, 0x8, nil)
			return
		case 0x9:
			_ = writeFrame(conn, 0xA, p)
		default:
			s.protocolErr.Add(1)
			return
		}
	}
}
func hasToken(v, t string) bool {
	for _, p := range strings.Split(v, ",") {
		if strings.EqualFold(strings.TrimSpace(p), t) {
			return true
		}
	}
	return false
}
func readFrame(r io.Reader, requireMask bool, max int) (byte, []byte, error) {
	var h [2]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return 0, nil, err
	}
	if h[0]&0x80 == 0 || h[0]&0x70 != 0 {
		return 0, nil, fmt.Errorf("fragmented/RSV websocket frame unsupported")
	}
	op := h[0] & 0x0f
	masked := h[1]&0x80 != 0
	if requireMask && !masked {
		return 0, nil, fmt.Errorf("client frame is not masked")
	}
	n := uint64(h[1] & 0x7f)
	if n == 126 {
		var x [2]byte
		if _, err := io.ReadFull(r, x[:]); err != nil {
			return 0, nil, err
		}
		n = uint64(binary.BigEndian.Uint16(x[:]))
	} else if n == 127 {
		var x [8]byte
		if _, err := io.ReadFull(r, x[:]); err != nil {
			return 0, nil, err
		}
		n = binary.BigEndian.Uint64(x[:])
	}
	if n > uint64(max) {
		return 0, nil, fmt.Errorf("payload %d exceeds max %d", n, max)
	}
	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(r, mask[:]); err != nil {
			return 0, nil, err
		}
	}
	p := make([]byte, int(n))
	if _, err := io.ReadFull(r, p); err != nil {
		return 0, nil, err
	}
	if masked {
		for i := range p {
			p[i] ^= mask[i&3]
		}
	}
	return op, p, nil
}
func writeFrame(w io.Writer, op byte, p []byte) error {
	n := len(p)
	h := []byte{0x80 | (op & 0x0f)}
	if n <= 125 {
		h = append(h, byte(n))
	} else if n <= 65535 {
		h = append(h, 126, byte(n>>8), byte(n))
	} else {
		return fmt.Errorf("payload too large")
	}
	if _, err := w.Write(h); err != nil {
		return err
	}
	_, err := w.Write(p)
	return err
}
