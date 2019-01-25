package wsync

import (
	"log"

	"github.com/gorilla/websocket"
)

var DefaultDialer = websocket.DefaultDialer

type Client struct {
	URL       string
	Dialer    *websocket.Dialer
	OnTopic   func(string)
	AfterOpen func()
	OnError   func(error)

	Unsub chan string
	Sub   chan string
}

func NewClient(url string) *Client {
	c := &Client{
		URL:       url,
		Dialer:    DefaultDialer,
		OnTopic:   func(_ string) {},
		AfterOpen: func() {},
		OnError:   func(_ error) {},

		Unsub: make(chan string),
		Sub:   make(chan string),
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

	go c.AfterOpen()

	// read loop
	go func(conn *websocket.Conn) {
		defer conn.Close()
		for {
			_, p, err := conn.ReadMessage()
			if err != nil {
				c.OnError(err)
				return
			}
			if p[1] != ':' || len(p) < 2 {
				log.Println("error format")
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
		}
	}
}
