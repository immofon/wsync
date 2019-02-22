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

	go func() {
		time.Sleep(time.Millisecond * 30)
		s.C <- func(s *Server) {
			s.Boardcast("test", "ok")
		}
	}()

	go http.ListenAndServe(":8111", s)

	wg := &sync.WaitGroup{}
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go tclient(wg)
	}
	wg.Wait()
}

func tclient(wg *sync.WaitGroup) {
	c := NewClient("ws://localhost:8111", "mofon")
	c.OnTopic = func(topic string, metas ...string) {
		if topic == "test" && len(metas) == 1 && metas[0] == "ok" {
			wg.Done()
		}
	}
	c.OnError = func(err error) {
		fmt.Println("error:", err)
	}
	c.AfterOpen = func(_ *websocket.Conn) {
		go c.Sub("test")
	}

	c.Serve()
}
