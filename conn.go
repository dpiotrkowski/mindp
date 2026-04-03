package mindp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"sync/atomic"

	"mindp/internal/ws"
)

type cdpRequest struct {
	ID        int64  `json:"id,omitempty"`
	SessionID string `json:"sessionId,omitempty"`
	Method    string `json:"method"`
	Params    any    `json:"params,omitempty"`
}

type cdpResponse struct {
	ID        int64           `json:"id"`
	SessionID string          `json:"sessionId,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     *CDPError       `json:"error,omitempty"`
	Method    string          `json:"method,omitempty"`
	Params    json.RawMessage `json:"params,omitempty"`
}

type eventHandler func(json.RawMessage)

type subscription struct {
	sessionID string
	method    string
	handler   eventHandler
}

type conn struct {
	ws   *ws.Client
	seq  atomic.Int64
	done chan struct{}
	once sync.Once

	mu      sync.Mutex
	pending map[int64]chan cdpResponse
	subs    map[int64]subscription
	subSeq  int64
}

func newConn(ctx context.Context, wsURL string) (*conn, error) {
	client, err := ws.Dial(ctx, wsURL)
	if err != nil {
		return nil, err
	}
	c := &conn{
		ws:      client,
		done:    make(chan struct{}),
		pending: make(map[int64]chan cdpResponse),
		subs:    make(map[int64]subscription),
	}
	go c.readLoop()
	return c, nil
}

func (c *conn) close() error {
	c.once.Do(func() { close(c.done) })
	return c.ws.Close()
}

func (c *conn) call(ctx context.Context, sessionID, method string, params any, result any) error {
	id := c.seq.Add(1)
	respCh := make(chan cdpResponse, 1)
	c.mu.Lock()
	c.pending[id] = respCh
	c.mu.Unlock()

	req := cdpRequest{ID: id, SessionID: sessionID, Method: method, Params: params}
	payload, err := json.Marshal(req)
	if err != nil {
		return err
	}
	if err := c.ws.WriteText(payload); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return ctx.Err()
	case <-c.done:
		return io.EOF
	case resp := <-respCh:
		if resp.Error != nil {
			return resp.Error
		}
		if result != nil && len(resp.Result) > 0 {
			if err := json.Unmarshal(resp.Result, result); err != nil {
				return err
			}
		}
		return nil
	}
}

func (c *conn) subscribe(sessionID, method string, handler eventHandler) func() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subSeq++
	id := c.subSeq
	c.subs[id] = subscription{sessionID: sessionID, method: method, handler: handler}
	return func() {
		c.mu.Lock()
		delete(c.subs, id)
		c.mu.Unlock()
	}
}

func (c *conn) readLoop() {
	defer c.once.Do(func() { close(c.done) })
	for {
		msg, err := c.ws.ReadText()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			return
		}
		var resp cdpResponse
		if err := json.Unmarshal(msg, &resp); err != nil {
			continue
		}
		if resp.ID != 0 {
			c.mu.Lock()
			ch := c.pending[resp.ID]
			delete(c.pending, resp.ID)
			c.mu.Unlock()
			if ch != nil {
				ch <- resp
			}
			continue
		}
		c.dispatchEvent(resp.SessionID, resp.Method, resp.Params)
	}
}

func (c *conn) dispatchEvent(sessionID, method string, params json.RawMessage) {
	c.mu.Lock()
	subs := make([]subscription, 0, len(c.subs))
	for _, sub := range c.subs {
		if sub.method == method && sub.sessionID == sessionID {
			subs = append(subs, sub)
		}
	}
	c.mu.Unlock()
	for _, sub := range subs {
		sub.handler(params)
	}
}
