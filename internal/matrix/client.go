package matrix

import (
	"bytes"
	"fmt"
	"net"
	"sync"
	"time"
)

// Matrix is the control surface the TUI depends on. *Client implements it;
// tests inject a fake so no commands hit a real device.
type Matrix interface {
	Status() (map[int]int, error)
	Route(in, out int) error
	AllTo(in int) error
	Mirror() error
	SavePreset(p int) error
	RecallPreset(p int) error
	Buzzer(on bool) error
	Connected() bool
}

// Client holds one persistent TCP connection, serialized with a mutex so the
// periodic status poll never interleaves on the socket with a user action.
type Client struct {
	Host        string
	Port        int
	ReadTimeout time.Duration

	mu        sync.Mutex
	conn      net.Conn
	connected bool
	lastErr   error
}

// New returns a Client for host:port with a sane default read timeout.
func New(host string, port int) *Client {
	return &Client{Host: host, Port: port, ReadTimeout: 2 * time.Second}
}

func (c *Client) addr() string { return net.JoinHostPort(c.Host, fmt.Sprint(c.Port)) }

// ensure dials if there is no live connection. Caller must hold c.mu.
func (c *Client) ensure() error {
	if c.conn != nil {
		return nil
	}
	conn, err := net.DialTimeout("tcp", c.addr(), c.ReadTimeout)
	if err != nil {
		c.connected = false
		c.lastErr = err
		return err
	}
	c.conn = conn
	c.connected = true
	c.lastErr = nil
	return nil
}

// drop tears down the connection and records why. Caller must hold c.mu.
func (c *Client) drop(err error) {
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
	c.connected = false
	c.lastErr = err
}

// write sends a command framed with CRLF. Caller must hold c.mu.
func (c *Client) write(cmd string) error {
	if err := c.ensure(); err != nil {
		return err
	}
	_ = c.conn.SetWriteDeadline(time.Now().Add(c.ReadTimeout))
	if _, err := c.conn.Write([]byte(cmd + term)); err != nil {
		c.drop(err)
		return err
	}
	return nil
}

// readUntilEnd accumulates chunks until "END" appears or the deadline elapses.
// The device dribbles its reply across several segments and is slow to send END.
// Caller must hold c.mu.
func (c *Client) readUntilEnd() (string, error) {
	_ = c.conn.SetReadDeadline(time.Now().Add(c.ReadTimeout))
	var buf bytes.Buffer
	chunk := make([]byte, 128)
	for !bytes.Contains(buf.Bytes(), []byte("END")) {
		n, err := c.conn.Read(chunk)
		if n > 0 {
			buf.Write(chunk[:n])
		}
		if err != nil {
			// A timeout after we already have data is fine; otherwise it's fatal.
			if ne, ok := err.(net.Error); ok && ne.Timeout() && buf.Len() > 0 {
				break
			}
			c.drop(err)
			return buf.String(), err
		}
	}
	return buf.String(), nil
}

// Status polls and parses the current routing.
func (c *Client) Status() (map[int]int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.write(CmdReadStatus()); err != nil {
		return nil, err
	}
	reply, err := c.readUntilEnd()
	if err != nil {
		return nil, err
	}
	c.connected = true
	c.lastErr = nil
	return ParseStatus(reply), nil
}

// fire sends a no-reply command.
func (c *Client) fire(cmd string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.write(cmd)
}

func (c *Client) Route(in, out int) error  { return c.fire(CmdSwitch(in, out)) }
func (c *Client) AllTo(in int) error       { return c.fire(CmdSwitch(in, 0)) }
func (c *Client) Mirror() error            { return c.fire(CmdMirror()) }
func (c *Client) SavePreset(p int) error   { return c.fire(CmdSavePreset(p)) }
func (c *Client) RecallPreset(p int) error { return c.fire(CmdRecallPreset(p)) }
func (c *Client) Buzzer(on bool) error     { return c.fire(CmdBuzzer(on)) }

// Connected reports whether the last operation held a live connection.
func (c *Client) Connected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

// LastError returns the most recent connection error, if any.
func (c *Client) LastError() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastErr
}

// Close releases the connection.
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.drop(nil)
}
