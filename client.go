package wsync

import (
	"github.com/gorilla/websocket"
)

var DefaultDialer = websocket.DefaultDialer

type Client struct {
	URL       string
	Dialer    *websocket.Dialer
	OnTopic   func(string)
	AfterOpen func(conn *websocket.Conn)
	OnError   func(error)

	Unsub     chan string
	Sub       chan string
	Boardcast chan string
}

func NewClient(url string) *Client {
	c := &Client{
		URL:       url,
		Dialer:    DefaultDialer,
		OnTopic:   func(_ string) {},
		AfterOpen: func(_ *websocket.Conn) {},
		OnError:   func(_ error) {},

		Unsub:     make(chan string),
		Sub:       make(chan string),
		Boardcast: make(chan string),
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
			messageType, p, err := conn.ReadMessage()
			if err != nil {
				c.OnError(err)
				return
			}

			if messageType == websocket.PingMessage {
				err := conn.WriteMessage(websocket.PongMessage, nil)
				if err != nil {
					c.OnError(err)
					return
				}
				continue
			}

			if p[1] != ':' || len(p) < 2 {
				return
			}

			text := string(p[2:])

			switch p[0] {
			case 'T': // topic
				c.OnTopic(text)
			}
		}
	}(conn)

	// write loop
	for {
		select {
		case topic := <-c.Sub:
			err := conn.WriteMessage(websocket.TextMessage, []byte("S:"+topic))
			if err != nil {
				c.OnError(err)
				return
			}
		case topic := <-c.Unsub:
			err := conn.WriteMessage(websocket.TextMessage, []byte("U:"+topic))
			if err != nil {
				c.OnError(err)
				return
			}
		case topic := <-c.Boardcast:
			err := conn.WriteMessage(websocket.TextMessage, []byte("B:"+topic))
			if err != nil {
				c.OnError(err)
				return
			}
		}
	}
}
