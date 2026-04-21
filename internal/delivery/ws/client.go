package ws

import (
	"encoding/json"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10

	sendBufferSize = 256
)

type Client struct {
	hub     *Hub
	conn    *websocket.Conn
	userID  uuid.UUID
	chatIDs []uuid.UUID
	cfg     Config
	send    chan []byte
}

func NewClient(hub *Hub, conn *websocket.Conn, userID uuid.UUID, chatIDs []uuid.UUID, cfg Config) *Client {
	c := &Client{
		hub:     hub,
		conn:    conn,
		userID:  userID,
		chatIDs: chatIDs,
		cfg:     cfg,
		send:    make(chan []byte, sendBufferSize),
	}
	hub.Subscribe(c, chatIDs)
	return c
}

func (c *Client) isMember(chatID uuid.UUID) bool {
	return slices.Contains(c.chatIDs, chatID)
}

func (c *Client) Run() {
	go c.writePump()
	c.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.hub.Unsubscribe(c, c.chatIDs)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(c.cfg.MaxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure,
			) {
				log.Warn().Err(err).Str("user", c.userID.String()).Msg("ws: unexpected close")
			}
			return
		}

		var ce clientEvent
		if err := json.Unmarshal(data, &ce); err != nil {
			log.Warn().Err(err).Str("user", c.userID.String()).Msg("ws: malformed event")
			continue
		}
		if !c.isMember(ce.ChatID) {
			log.Warn().
				Str("user", c.userID.String()).
				Str("chat", ce.ChatID.String()).
				Msg("ws: event for unsubscribed chat, ignoring")
			continue
		}

		c.hub.BroadcastToChat(ce.ChatID, Event{
			Type:     ce.Type,
			Payload:  ce.Payload,
			SenderID: c.userID,
			ChatID:   ce.ChatID,
		}, c.userID)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				log.Warn().Err(err).Str("user", c.userID.String()).Msg("ws: write error")
				return
			}

			n := len(c.send)
			for range n {
				if err := c.conn.WriteMessage(websocket.TextMessage, <-c.send); err != nil {
					return
				}
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Warn().Err(err).Str("user", c.userID.String()).Msg("ws: ping error")
				return
			}
		}
	}
}
