package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime/pprof"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/webasis/wsync"
)

var ServerURL = getenv("WSYNC_SERVER_URL", "ws://localhost:8111/")
var ServeAddr = getenv("WSYNC_SERVE_ADDR", "localhost:8111")

func getenv(key, deft string) string {
	v := os.Getenv(key)
	if v == "" {
		return deft
	}
	return v

}

func daemon() {
	f, err := os.Create("cpu.prof")
	if err != nil {
		log.Fatal(err)
	}
	pprof.StartCPUProfile(f)
	go func() {
		defer f.Close()
		defer pprof.StopCPUProfile()
		time.Sleep(time.Second * 10)
	}()

	s := wsync.NewServer()
	s.Auth = func(token string, m wsync.AuthMethod, topic string) bool {
		// fmt.Println("auth:", token, m, topic)
		return true
	}

	go func() {
		lastSent := 0
		for range time.Tick(time.Second) {
			s.C <- func(s *wsync.Server) {
				if lastSent == 0 {
					lastSent = s.MessageSent
				}

				fmt.Println("message_sent:", s.MessageSent)
				fmt.Println("connected_count:", len(s.Agents))
				fmt.Println("message_sent_per_second:", s.MessageSent-lastSent)
				lastSent = s.MessageSent
			}
		}
	}()

	http.ListenAndServe(ServeAddr, s)
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
	c := wsync.NewClient(ServerURL, "mofon")
	c.OnTopic = func(topic string, metas ...string) {
	}
	c.OnError = func(err error) {
		//fmt.Println("error:", err)
	}
	c.AfterOpen = func(conn *websocket.Conn) {
		go func() {
			c.Sub("test", "testclient")

			for {
				c.Boardcast("testclient")
			}
		}()
	}

	for {
		c.Serve()
	}
}

func client() {
	c := wsync.NewClient(ServerURL, "mofon")
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
