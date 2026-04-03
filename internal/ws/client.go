package ws

import (
	"bufio"
	"context"
	"crypto/rand"
	// #nosec G505 -- RFC6455 requires SHA-1 for the websocket accept handshake.
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Client struct {
	conn net.Conn
	br   *bufio.Reader
	mu   sync.Mutex
}

func Dial(ctx context.Context, rawURL string) (*Client, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	host := u.Host
	if !strings.Contains(host, ":") {
		if u.Scheme == "wss" {
			host += ":443"
		} else {
			host += ":80"
		}
	}
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", host)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "wss" {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: u.Hostname()})
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			_ = conn.Close()
			return nil, err
		}
		conn = tlsConn
	}
	keyRaw := make([]byte, 16)
	if _, err := rand.Read(keyRaw); err != nil {
		_ = conn.Close()
		return nil, err
	}
	key := base64.StdEncoding.EncodeToString(keyRaw)
	path := u.RequestURI()
	if path == "" {
		path = "/"
	}
	req := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: %s\r\n\r\n", path, u.Host, key)
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	if _, err := io.WriteString(conn, req); err != nil {
		_ = conn.Close()
		return nil, err
	}
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusSwitchingProtocols {
		_ = conn.Close()
		return nil, fmt.Errorf("websocket handshake failed: %s", resp.Status)
	}
	accept := resp.Header.Get("Sec-WebSocket-Accept")
	want := serverAccept(key)
	if accept != want {
		_ = conn.Close()
		return nil, fmt.Errorf("websocket accept mismatch: got %q want %q", accept, want)
	}
	_ = conn.SetDeadline(time.Time{})
	return &Client{conn: conn, br: br}, nil
}

func serverAccept(key string) string {
	// #nosec G401 -- RFC6455 defines SHA-1 for Sec-WebSocket-Accept derivation.
	h := sha1.New()
	_, _ = io.WriteString(h, key)
	_, _ = io.WriteString(h, "258EAFA5-E914-47DA-95CA-C5AB0DC85B11")
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func (c *Client) Close() error {
	_ = c.WriteClose()
	return c.conn.Close()
}

func (c *Client) WriteText(payload []byte) error {
	return c.writeFrame(0x1, payload)
}

func (c *Client) WriteClose() error {
	return c.writeFrame(0x8, nil)
}

func (c *Client) WritePong(payload []byte) error {
	return c.writeFrame(0xA, payload)
}

func (c *Client) writeFrame(opcode byte, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	header := []byte{0x80 | opcode}
	n := len(payload)
	switch {
	case n < 126:
		header = append(header, 0x80|byte(n))
	case n <= 65535:
		buf := make([]byte, 2)
		binary.BigEndian.PutUint16(buf, uint16(n))
		header = append(header, 0x80|126)
		header = append(header, buf...)
	default:
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, uint64(n))
		header = append(header, 0x80|127)
		header = append(header, buf...)
	}
	mask := make([]byte, 4)
	if _, err := rand.Read(mask); err != nil {
		return err
	}
	header = append(header, mask...)
	masked := make([]byte, len(payload))
	for i := range payload {
		masked[i] = payload[i] ^ mask[i%4]
	}
	if _, err := c.conn.Write(header); err != nil {
		return err
	}
	if len(masked) > 0 {
		_, err := c.conn.Write(masked)
		return err
	}
	return nil
}

func (c *Client) ReadText() ([]byte, error) {
	var message []byte
	var started bool
	for {
		opcode, payload, fin, err := c.readFrame()
		if err != nil {
			return nil, err
		}
		switch opcode {
		case 0x1:
			message = append(message, payload...)
			started = true
			if fin {
				return message, nil
			}
		case 0x0:
			if !started {
				return nil, errors.New("unexpected continuation frame")
			}
			message = append(message, payload...)
			if fin {
				return message, nil
			}
		case 0x8:
			return nil, io.EOF
		case 0x9:
			if err := c.WritePong(payload); err != nil {
				return nil, err
			}
		case 0xA:
		default:
			return nil, fmt.Errorf("unsupported websocket opcode %d", opcode)
		}
	}
}

func (c *Client) readFrame() (byte, []byte, bool, error) {
	first, err := c.br.ReadByte()
	if err != nil {
		return 0, nil, false, err
	}
	second, err := c.br.ReadByte()
	if err != nil {
		return 0, nil, false, err
	}
	opcode := first & 0x0F
	fin := first&0x80 != 0
	masked := second&0x80 != 0
	size := int(second & 0x7F)
	switch size {
	case 126:
		buf := make([]byte, 2)
		if _, err := io.ReadFull(c.br, buf); err != nil {
			return 0, nil, false, err
		}
		size = int(binary.BigEndian.Uint16(buf))
	case 127:
		buf := make([]byte, 8)
		if _, err := io.ReadFull(c.br, buf); err != nil {
			return 0, nil, false, err
		}
		rawSize := binary.BigEndian.Uint64(buf)
		const maxInt = int(^uint(0) >> 1)
		if rawSize > uint64(maxInt) {
			return 0, nil, false, errors.New("websocket frame too large")
		}
		size = int(rawSize)
	}
	var mask []byte
	if masked {
		mask = make([]byte, 4)
		if _, err := io.ReadFull(c.br, mask); err != nil {
			return 0, nil, false, err
		}
	}
	payload := make([]byte, size)
	if _, err := io.ReadFull(c.br, payload); err != nil {
		return 0, nil, false, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	return opcode, payload, fin, nil
}
