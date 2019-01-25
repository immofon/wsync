package wsync

import "time"

const (
	// Time allowed to write the file to the client.
	WriteWait = 10 * time.Second

	// Time allowed to read the next pong message from the client.
	PongWait = 60 * time.Second

	// Send pings to client with this period. Must be less than pongWait.
	PingPeriod = (PongWait * 9) / 10
)
