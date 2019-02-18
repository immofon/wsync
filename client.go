package wsync

import (
	"time"

	"github.com/gorilla/websocket"
)

var DefaultDialer = websocket.DefaultDialer

var DefaultReadWait = PongWait
var DefaultWriteWait = WriteWait

type Client struct {
	URL       string
	Token     string // for auth
	Dialer    *websocket.Dialer
	OnTopic   func(topic string, metas ...string)
	AfterOpen func(conn *websocket.Conn)
	OnError   func(error)

	ReadWait  time.Duration
	WriteWait time.Duration

	S chan string     // subscribe
	U chan string     // unsubscrbe
	B chan TopicEvent // boardcast
}

func NewClient(url string, token string) *Client {
	c := &Client{
		URL:       url,
		Token:     token,
		Dialer:    DefaultDialer,
		OnTopic:   func(_ string, _ ...string) {},
		AfterOpen: func(_ *websocket.Conn) {},
		OnError:   func(_ error) {},

		ReadWait:  DefaultReadWait,
		WriteWait: DefaultWriteWait,

		U: make(chan string),
		S: make(chan string),
		B: make(chan TopicEvent),
	}

	return c
}

func (c *Client) Serve() {
	conn, _, err := c.Dialer.Dial(c.URL, nil)
	if err != nil {
		c.OnError(err)
		return
	}
	defer conn.Close()

	c.AfterOpen(conn)

	// read loop
	go func(conn *websocket.Conn) {
		defer conn.Close()
		for {
			conn.SetReadDeadline(time.Now().Add(c.ReadWait))
			_, p, err := conn.ReadMessage()
			if err != nil {
				c.OnError(err)
				return
			}

			method, topic, metas := Decode(string(p))

			switch method {
			case "t": // topic
				c.OnTopic(topic, metas...)
			case "p":
				conn.SetWriteDeadline(time.Now().Add(c.WriteWait))
				err = conn.WriteMessage(websocket.TextMessage, []byte("P"))
				if err != nil {
					c.OnError(err)
					return
				}
			}
		}
	}(conn)

	// write loop
	conn.SetWriteDeadline(time.Now().Add(c.WriteWait))
	err = conn.WriteMessage(websocket.TextMessage, Encode("A", c.Token))
	if err != nil {
		c.OnError(err)
		return
	}

	for {
		select {
		case topic := <-c.S:
			conn.SetWriteDeadline(time.Now().Add(c.WriteWait))
			err := conn.WriteMessage(websocket.TextMessage, Encode("S", topic))
			if err != nil {
				c.OnError(err)
				return
			}
		case topic := <-c.U:
			conn.SetWriteDeadline(time.Now().Add(c.WriteWait))
			err := conn.WriteMessage(websocket.TextMessage, Encode("U", topic))
			if err != nil {
				c.OnError(err)
				return
			}
		case topic := <-c.B:
			conn.SetWriteDeadline(time.Now().Add(c.WriteWait))
			err := conn.WriteMessage(websocket.TextMessage, Encode("B", topic.Topic, topic.Meta...))
			if err != nil {
				c.OnError(err)
				return
			}
		}
	}
}

func (c *Client) Sub(topics ...string) {
	for _, topic := range topics {
		c.S <- topic
	}
}
func (c *Client) Unsub(topics ...string) {
	for _, topic := range topics {
		c.U <- topic
	}
}
func (c *Client) Boardcast(topic string, metas ...string) {
	c.B <- TopicEvent{
		Topic: topic,
		Meta:  metas,
	}
}
