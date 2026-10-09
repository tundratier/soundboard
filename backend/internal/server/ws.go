package server

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"

	"soundboard/internal/i18n"
)

type client struct {
	send chan []byte
}

type hub struct {
	mu      sync.Mutex
	clients map[*client]struct{}
}

func newHub() *hub { return &hub{clients: map[*client]struct{}{}} }

func (h *hub) add(c *client) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

func (h *hub) remove(c *client) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
}

func (h *hub) broadcast(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		select {
		case c.send <- b:
		default: // slow client: drop the message; it catches up on the next "hello"/refetch
		}
	}
}

func (h *hub) closeAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		close(c.send)
		delete(h.clients, c)
	}
}

type wsMsg struct {
	Type string `json:"type"`
	Tile string `json:"tile,omitempty"`
}

// ws: clients send {"type":"trigger"|"stop"|"stopAll","tile":"…"} and receive
// {"type":"hello","playing":[…]}, {"type":"started"|"stopped","tile":"…"}, {"type":"board"}.
func (s *Server) ws(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil) // checks Origin against Host
	if err != nil {
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(4096)

	c := &client{send: make(chan []byte, 64)}
	hello, _ := json.Marshal(map[string]any{"type": "hello", "playing": s.d.Engine.Playing()})
	c.send <- hello // before add(): otherwise races with closeAll
	s.hub.add(c)
	defer s.hub.remove(c)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go func() { // Writer
		defer cancel()
		ping := time.NewTicker(25 * time.Second)
		defer ping.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-c.send:
				if !ok {
					return
				}
				wctx, wcancel := context.WithTimeout(ctx, 5*time.Second)
				err := conn.Write(wctx, websocket.MessageText, msg)
				wcancel()
				if err != nil {
					return
				}
			case <-ping.C:
				pctx, pcancel := context.WithTimeout(ctx, 5*time.Second)
				err := conn.Ping(pctx)
				pcancel()
				if err != nil {
					return
				}
			}
		}
	}()

	for { // Reader
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		var m wsMsg
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		switch m.Type {
		case "trigger":
			if !s.trigger(m.Tile) {
				i18n.Logf(i18n.LogUnknownTile, m.Tile)
			}
		case "stop":
			s.d.Engine.Stop(m.Tile)
		case "stopAll":
			s.d.Engine.StopAll()
		}
	}
}
