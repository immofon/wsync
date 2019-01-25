package wsync

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
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

	wg := &sync.WaitGroup{}
	for i := 0; i < 300; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tclient()
		}()
	}
	wg.Wait()
}

func tclient() {
	c := NewClient("ws://localhost:8111")
	c.OnTopic = func(topic string) {
	}
	c.OnError = func(err error) {
		fmt.Println("error:", err)
	}
	c.AfterOpen = func(_ *websocket.Conn) {
		go func() { c.Sub <- "test" }()
	}

	c.Serve()
}
