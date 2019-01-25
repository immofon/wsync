package wsync

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestServer(t *testing.T) {
	s := NewServer()
	go s.Serve()

	go func() {
		for range time.Tick(time.Second) {
			s.C <- func(s *Server) {
				s.Boardcast("test")
				fmt.Println("message_sent:", s.MessageSent)
				fmt.Println("connected_count:", len(s.Agents))
				for conn, _ := range s.Agents {
					conn.Close()
					break
				}
				go tclient()
			}
		}
	}()

	go http.ListenAndServe(":8111", s)

	for i := 0; i < 300; i++ {
		go tclient()
	}
	tclient()
}

func tclient() {
	c := NewClient("ws://localhost:8111")
	c.OnTopic = func(topic string) {
	}
	c.OnError = func(err error) {
		fmt.Println("error:", err)
	}
	c.AfterOpen = func() {
		c.Sub <- "test"
	}

	c.Serve()
}
