package ws

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestDialAndReadText(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		br := bufio.NewReader(conn)
		req, err := http.ReadRequest(br)
		if err != nil {
			return
		}
		key := req.Header.Get("Sec-WebSocket-Key")
		resp := fmt.Sprintf("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", serverAccept(key))
		if _, err := io.WriteString(conn, resp); err != nil {
			return
		}
		writeServerFrame(conn, 0x1, false, []byte("hel"))
		writeServerFrame(conn, 0x0, true, []byte("lo"))
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := Dial(ctx, "ws://"+ln.Addr().String()+"/devtools/page/test")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	msg, err := client.ReadText()
	if err != nil {
		t.Fatal(err)
	}
	if string(msg) != "hello" {
		t.Fatalf("got %q", msg)
	}
}

func TestServerAccept(t *testing.T) {
	got := serverAccept(base64.StdEncoding.EncodeToString([]byte("0123456789abcdef")))
	if strings.TrimSpace(got) == "" {
		t.Fatal("empty accept header")
	}
}

func writeServerFrame(w io.Writer, opcode byte, fin bool, payload []byte) {
	first := opcode
	if fin {
		first |= 0x80
	}
	frame := []byte{first}
	if len(payload) < 126 {
		frame = append(frame, byte(len(payload)))
	}
	frame = append(frame, payload...)
	_, _ = w.Write(frame)
}
