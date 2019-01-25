package wsync

import (
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var DefaultUpgrader = websocket.Upgrader{
	ReadBufferSize:  128,
	WriteBufferSize: 128,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

const DefaultMessageCache = 1

type Agent struct {
	Sub map[string]bool
}

func NewAgent() *Agent {
	return &Agent{
		Sub: make(map[string]bool),
	}
}

type ServerFunc func(*Server)

type Server struct {
	C chan ServerFunc // never closed

	Agents map[*websocket.Conn]*Agent

	// status
	MessageSent int

	// readonly
	Upgrader     websocket.Upgrader
	MessageCache int
}

func NewServer() *Server {
	return &Server{
		C:      make(chan ServerFunc, 100),
		Agents: make(map[*websocket.Conn]*Agent),

		MessageSent: 0,

		Upgrader:     DefaultUpgrader,
		MessageCache: DefaultMessageCache,
	}
}

func (s *Server) Serve() {
	for fn := range s.C {
		fn(s)
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := s.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	msgCache := s.MessageCache
	if msgCache < 1 {
		msgCache = 1
	}
	updatedCh := make(chan string, msgCache)

	defer func() {
		s.C <- func(s *Server) {
			close(updatedCh) // close after leave conn
		}
	}()

	s.C <- func(s *Server) { s.Join(conn) }
	defer func() {
		s.C <- func(s *Server) { s.Leave(conn) }
	}()

	go func() {
		ticker := time.NewTicker(time.Millisecond * 10)
		defer ticker.Stop()

		for {
			select {
			case topic, ok := <-updatedCh:
				if !ok {
					return
				}

				err := conn.WriteMessage(websocket.TextMessage, []byte("T:"+topic))
				if err != nil {
					conn.Close()
				}
			case <-ticker.C:
				s.C <- func(s *Server) { s.GetUpdated(conn, updatedCh) }
			}
		}
	}()

	// read loop
	for {
		_, p, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}
		if p[1] != ':' || len(p) < 2 {
			log.Println("error format")
			return
		}

		text := string(p[2:])

		switch p[0] {
		case 'S': // subscibe
			s.C <- func(s *Server) { s.Sub(conn, text) }
		case 'A': // auth
		// TODO
		case 'U': // unsubscibe
			s.C <- func(s *Server) { s.Unsub(conn, text) }
		case 'P': // ping
			// TODO
		}
	}
}

func (s *Server) Join(conn *websocket.Conn) {
	s.Agents[conn] = NewAgent()
	log.Println("join", conn.RemoteAddr())
}

func (s *Server) Leave(conn *websocket.Conn) {
	delete(s.Agents, conn)
	log.Println("leave", conn.RemoteAddr())
}

func (s *Server) Sub(conn *websocket.Conn, topic string) {
	a := s.Agents[conn]
	if a == nil {
		return
	}

	if _, ok := a.Sub[topic]; !ok {
		a.Sub[topic] = false
		log.Println("sub", conn.RemoteAddr(), topic)
	}
}

func (s *Server) Unsub(conn *websocket.Conn, topic string) {
	a := s.Agents[conn]
	if a == nil {
		return
	}

	delete(a.Sub, topic)
	log.Println("unsub", conn.RemoteAddr(), topic)
}

func (s *Server) Boardcast(topic string) {
	for _, a := range s.Agents {
		if _, ok := a.Sub[topic]; ok {
			a.Sub[topic] = true
		}
	}
	log.Println("boardcast", topic)
}

func (s *Server) GetUpdated(conn *websocket.Conn, ch chan<- string) {
	a := s.Agents[conn]
	if a == nil {
		return
	}

	for topic, updated := range a.Sub {
		if updated {
			select {
			case ch <- topic:
				a.Sub[topic] = false
				s.MessageSent++
			default:
				return
			}
		}
	}
}
