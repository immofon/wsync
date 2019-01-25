package main

import (
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/immofon/wsync"
)

func daemon() {
	s := wsync.NewServer()
	go s.Serve()

	go func() {
		for range time.Tick(time.Second) {
			s.C <- func(s *wsync.Server) {
				s.Boardcast("test")
				fmt.Println("message_sent:", s.MessageSent)
				fmt.Println("connected_count:", len(s.Agents))
				for conn, _ := range s.Agents {
					conn.Close()
					break
				}
			}
		}
	}()

	http.ListenAndServe(":8111", s)
}

func client() {
	wg := &sync.WaitGroup{}

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tclient()
		}()
	}

	wg.Wait()
}

func tclient() {
	c := wsync.NewClient("ws://localhost:8111")
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

func main() {
	cmd := os.Getenv("cmd")

	switch cmd {
	case "client", "":
		client()
	case "daemon":
		daemon()
	}
}
