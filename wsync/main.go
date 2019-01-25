package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/immofon/wsync"
)

func daemon() {
	s := wsync.NewServer()
	s.Auth = func(token string, m wsync.AuthMethod, topic string) bool {
		fmt.Println("auth:", token, m, topic)
		return true
	}

	go s.Serve()

	go func() {
		for range time.Tick(time.Second) {
			s.C <- func(s *wsync.Server) {
				fmt.Println("message_sent:", s.MessageSent)
				fmt.Println("connected_count:", len(s.Agents))
			}
		}
	}()

	http.ListenAndServe(":8111", s)
}

func test() {
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
	c := wsync.NewClient("ws://localhost:8111", "mofon")
	c.OnTopic = func(topic string, metas ...string) {
	}
	c.OnError = func(err error) {
		fmt.Println("error:", err)
	}
	c.AfterOpen = func(conn *websocket.Conn) {
		conn.WriteMessage(websocket.TextMessage, []byte("A:mofon"))
		go func() {
			c.Sub("test", "testclient")

			time.Sleep(time.Second)
			c.Boardcast("testclient")
		}()
	}

	c.Serve()
}

func client() {
	c := wsync.NewClient("ws://localhost:8111", "mofon")
	c.OnTopic = func(topic string, metas ...string) {
		fmt.Println("t:", topic, metas)
	}
	c.OnError = func(err error) {
		fmt.Println("error:", err)
	}
	c.AfterOpen = func(conn *websocket.Conn) {
		go func() {
			c.Sub("test", "testclient")

			time.Sleep(time.Second)

			c.Boardcast("testclient")

			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				raw := scanner.Text()
				data := strings.Split(raw, " ")
				method, topic, metas := wsync.DecodeData(data...)
				fmt.Println(method, topic, metas)
				fmt.Println(method)
				switch method {
				case "S":
					c.Sub(topic)
				case "U":
					c.Unsub(topic)
				case "B":
					c.Boardcast(topic, metas...)
				default:
					help()
				}
			}
			if err := scanner.Err(); err != nil {
				fmt.Fprintln(os.Stderr, "reading standard input:", err)
			}

		}()
	}

	for {
		c.Serve()
	}

}

func help() {
	fmt.Println("help: (S|U|B) topic {meta}")
}

func main() {
	cmd := os.Getenv("cmd")

	switch cmd {
	case "test":
		test()
	case "client", "":
		client()
	case "daemon":
		daemon()
	}
}
