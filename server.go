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

type AuthMethod int

const (
	AuthMethod_Auth AuthMethod = iota
	AuthMethod_Sub
	AuthMethod_Boardcast
)

func (am AuthMethod) String() string {
	switch am {
	case AuthMethod_Auth:
		return "[auth:auth]"
	case AuthMethod_Sub:
		return "[auth:sub]"
	case AuthMethod_Boardcast:
		return "[auth:boardcast]"
	default:
		return "[auth:unknown]"
	}
}

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
	Auth         func(token string, m AuthMethod, topic string) bool
}

func NewServer() *Server {
	return &Server{
		C:      make(chan ServerFunc, 100),
		Agents: make(map[*websocket.Conn]*Agent),

		MessageSent: 0,

		Upgrader:     DefaultUpgrader,
		MessageCache: DefaultMessageCache,
		Auth:         func(_ string, _ AuthMethod, _ string) bool { return true },
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
		pingTicker := time.NewTicker(PingPeriod)
		defer pingTicker.Stop()

		for {
			select {
			case topic, ok := <-updatedCh:
				if !ok {
					return
				}

				conn.SetWriteDeadline(time.Now().Add(WriteWait))
				err := conn.WriteMessage(websocket.TextMessage, []byte("T:"+topic))
				if err != nil {
					conn.Close()
				}
			case <-ticker.C:
				s.C <- func(s *Server) { s.GetUpdated(conn, updatedCh) }
			case <-pingTicker.C:
				conn.SetWriteDeadline(time.Now().Add(WriteWait))
				err := conn.WriteMessage(websocket.PingMessage, nil)
				if err != nil {
					conn.Close()
				}
			}
		}
	}()

	// read loop
	conn.SetReadDeadline(time.Now().Add(PongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(PongWait))
		return nil
	})

	var token string
	for {
		_, p, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if p[1] != ':' || len(p) < 2 {
			return
		}

		topic := string(p[2:])

		switch p[0] {
		case 'S': // subscibe
			s.C <- func(s *Server) {
				if !s.Auth(token, AuthMethod_Sub, topic) {
					conn.Close()
				}
				s.Sub(conn, topic)
			}
		case 'A': // auth
			s.C <- func(s *Server) {
				token = topic
				if !s.Auth(token, AuthMethod_Auth, "") {
					conn.Close()
				}
			}
		case 'U': // unsubscibe
			s.C <- func(s *Server) {
				if !s.Auth(token, AuthMethod_Sub, topic) {
					conn.Close()
				}
				s.Unsub(conn, topic)
			}
		case 'B': //boardcast
			s.C <- func(s *Server) {
				if !s.Auth(token, AuthMethod_Boardcast, topic) {
					conn.Close()
				}
				s.Boardcast(topic)
			}
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
