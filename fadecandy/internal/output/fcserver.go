package output

import (
	"fmt"
	"net"
	"sync"
	"time"

	"pxlblz-fadecandy/internal/opc"
)

type Client struct {
	addr    string
	channel byte
	timeout time.Duration
	mu      sync.Mutex
	conn    *net.TCPConn
}

func New(addr string, channel byte) *Client {
	if addr == "" {
		addr = "127.0.0.1:7890"
	}
	return &Client{addr: addr, channel: channel, timeout: 2 * time.Second}
}

func (c *Client) connectLocked() error {
	if c.conn != nil {
		return nil
	}
	d := net.Dialer{Timeout: c.timeout}
	conn, err := d.Dial("tcp", c.addr)
	if err != nil {
		return fmt.Errorf("connect fcserver %s: %w", c.addr, err)
	}
	tcp, ok := conn.(*net.TCPConn)
	if !ok {
		_ = conn.Close()
		return fmt.Errorf("fcserver connection is not TCP")
	}
	_ = tcp.SetNoDelay(true)
	c.conn = tcp
	return nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

func (c *Client) SendRGB(rgb []byte) error {
	packet, err := opc.EncodeSetPixelColors(c.channel, rgb)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err = c.connectLocked(); err != nil {
		return err
	}
	_ = c.conn.SetWriteDeadline(time.Now().Add(c.timeout))
	if _, err = c.conn.Write(packet); err != nil {
		_ = c.conn.Close()
		c.conn = nil
		return fmt.Errorf("write OPC frame: %w", err)
	}
	return nil
}
