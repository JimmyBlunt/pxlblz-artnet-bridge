// Package wsmini implements the deliberately small RFC6455 subset used by the
// localhost PXLBLZ pixel link. It supports one complete binary message per RGB
// frame plus ping/pong/close. No external runtime dependency is required.
package wsmini

import (
	"bufio"
	"context"
	"crypto/rand"
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

const websocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

func acceptForKey(key string) string {
	h := sha1.Sum([]byte(key + websocketGUID))
	return base64.StdEncoding.EncodeToString(h[:])
}

type ServerStats struct {
	Connections uint64
	Active      int64
	Binary      uint64
	ProtocolErr uint64
}

type Server struct {
	listen     string
	path       string
	maxPayload int
	onBinary   func([]byte)

	httpServer *http.Server
	listener   net.Listener
	mu         sync.Mutex
	conns      map[net.Conn]struct{}

	connections atomic.Uint64
	active      atomic.Int64
	binary      atomic.Uint64
	protocolErr atomic.Uint64
}

func NewServer(listen, path string, maxPayload int, onBinary func([]byte)) (*Server, error) {
	if listen == "" {
		listen = "127.0.0.1:9980"
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
		return nil, fmt.Errorf("onBinary callback is required")
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

func (s *Server) Stats() ServerStats {
	return ServerStats{Connections: s.connections.Load(), Active: s.active.Load(), Binary: s.binary.Load(), ProtocolErr: s.protocolErr.Load()}
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

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") || !headerHasToken(r.Header.Get("Connection"), "upgrade") {
		http.Error(w, "websocket upgrade required", http.StatusUpgradeRequired)
		return
	}
	key := strings.TrimSpace(r.Header.Get("Sec-WebSocket-Key"))
	if key == "" || r.Header.Get("Sec-WebSocket-Version") != "13" {
		http.Error(w, "invalid websocket handshake", http.StatusBadRequest)
		return
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking unsupported", http.StatusInternalServerError)
		return
	}
	conn, rw, err := hj.Hijack()
	if err != nil {
		return
	}
	if _, err := fmt.Fprintf(rw, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", acceptForKey(key)); err != nil {
		_ = conn.Close()
		return
	}
	if err := rw.Flush(); err != nil {
		_ = conn.Close()
		return
	}

	s.mu.Lock()
	s.conns[conn] = struct{}{}
	s.mu.Unlock()
	s.connections.Add(1)
	s.active.Add(1)
	defer func() { s.active.Add(-1); s.mu.Lock(); delete(s.conns, conn); s.mu.Unlock(); _ = conn.Close() }()

	br := rw.Reader
	for {
		opcode, payload, err := readFrame(br, true, s.maxPayload)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				s.protocolErr.Add(1)
			}
			return
		}
		switch opcode {
		case 0x2:
			s.binary.Add(1)
			s.onBinary(payload)
		case 0x8:
			_ = writeFrame(conn, 0x8, nil, false)
			return
		case 0x9:
			if err := writeFrame(conn, 0xA, payload, false); err != nil {
				return
			}
		case 0xA:
			// pong
		default:
			s.protocolErr.Add(1)
			_ = writeClose(conn, 1003, "binary frames only")
			return
		}
	}
}

func headerHasToken(v, token string) bool {
	for _, p := range strings.Split(v, ",") {
		if strings.EqualFold(strings.TrimSpace(p), token) {
			return true
		}
	}
	return false
}

func readFrame(r io.Reader, requireMask bool, maxPayload int) (byte, []byte, error) {
	var h [2]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return 0, nil, err
	}
	fin := h[0]&0x80 != 0
	if !fin || h[0]&0x70 != 0 {
		return 0, nil, fmt.Errorf("fragmented/RSV websocket frames unsupported")
	}
	opcode := h[0] & 0x0f
	masked := h[1]&0x80 != 0
	if requireMask && !masked {
		return 0, nil, fmt.Errorf("client frame is not masked")
	}
	n64 := uint64(h[1] & 0x7f)
	switch n64 {
	case 126:
		var x [2]byte
		if _, err := io.ReadFull(r, x[:]); err != nil {
			return 0, nil, err
		}
		n64 = uint64(binary.BigEndian.Uint16(x[:]))
	case 127:
		var x [8]byte
		if _, err := io.ReadFull(r, x[:]); err != nil {
			return 0, nil, err
		}
		n64 = binary.BigEndian.Uint64(x[:])
	}
	if n64 > uint64(maxPayload) {
		return 0, nil, fmt.Errorf("websocket payload %d exceeds max %d", n64, maxPayload)
	}
	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(r, mask[:]); err != nil {
			return 0, nil, err
		}
	}
	payload := make([]byte, int(n64))
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i&3]
		}
	}
	return opcode, payload, nil
}

func writeClose(w io.Writer, code uint16, text string) error {
	p := make([]byte, 2+len(text))
	binary.BigEndian.PutUint16(p[:2], code)
	copy(p[2:], text)
	return writeFrame(w, 0x8, p, false)
}

func writeFrame(w io.Writer, opcode byte, payload []byte, mask bool) error {
	if len(payload) > 65535 {
		return fmt.Errorf("payload too large for wsmini frame writer")
	}
	header := make([]byte, 0, 8)
	header = append(header, 0x80|(opcode&0x0f))
	maskBit := byte(0)
	if mask {
		maskBit = 0x80
	}
	n := len(payload)
	if n <= 125 {
		header = append(header, maskBit|byte(n))
	} else {
		header = append(header, maskBit|126, byte(n>>8), byte(n))
	}
	if mask {
		var m [4]byte
		if _, err := rand.Read(m[:]); err != nil {
			return err
		}
		header = append(header, m[:]...)
		if _, err := w.Write(header); err != nil {
			return err
		}
		tmp := make([]byte, n)
		for i := range payload {
			tmp[i] = payload[i] ^ m[i&3]
		}
		_, err := w.Write(tmp)
		return err
	}
	if _, err := w.Write(header); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

// Client is a minimal browser-compatible websocket client used by the standalone
// frame generator and tests. Production PXLBLZ will use the browser WebSocket API.
type Client struct {
	conn net.Conn
	br   *bufio.Reader
}

func Dial(urlHost, path string) (*Client, error) {
	if path == "" {
		path = "/pixels"
	}
	conn, err := net.DialTimeout("tcp", urlHost, 3*time.Second)
	if err != nil {
		return nil, err
	}
	keyRaw := make([]byte, 16)
	if _, err := rand.Read(keyRaw); err != nil {
		conn.Close()
		return nil, err
	}
	key := base64.StdEncoding.EncodeToString(keyRaw)
	req := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n", path, urlHost, key)
	if _, err := io.WriteString(conn, req); err != nil {
		conn.Close()
		return nil, err
	}
	br := bufio.NewReader(conn)
	status, err := br.ReadString('\n')
	if err != nil {
		conn.Close()
		return nil, err
	}
	if !strings.Contains(status, " 101 ") {
		conn.Close()
		return nil, fmt.Errorf("websocket handshake failed: %s", strings.TrimSpace(status))
	}
	headers := map[string]string{}
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			conn.Close()
			return nil, err
		}
		if line == "\r\n" {
			break
		}
		p := strings.SplitN(line, ":", 2)
		if len(p) == 2 {
			headers[strings.ToLower(strings.TrimSpace(p[0]))] = strings.TrimSpace(p[1])
		}
	}
	if headers["sec-websocket-accept"] != acceptForKey(key) {
		conn.Close()
		return nil, fmt.Errorf("invalid Sec-WebSocket-Accept")
	}
	return &Client{conn: conn, br: br}, nil
}

func (c *Client) SendBinary(payload []byte) error { return writeFrame(c.conn, 0x2, payload, true) }
func (c *Client) Close() error                    { _ = writeFrame(c.conn, 0x8, nil, true); return c.conn.Close() }
