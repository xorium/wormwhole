package console

import (
	"bufio"
	"github.com/fatih/color"
	"github.com/steveyen/gkvlite"
	"github.com/xorium/wormwhole/server"
	"log"
	"os"
)

const (
	stateExecutingCommand = iota
	stateClear            = iota
)

type Console struct {
	srv        *server.CommandServer
	reader     *bufio.Reader
	currentBot *server.Bot
	bots       *gkvlite.Collection
}

func NewConsole(srv *server.CommandServer) *Console {
	f, err := os.OpenFile("wormwhole.gkvlite", os.O_CREATE|os.O_RDWR, os.ModeType)
	if err != nil {
		log.Fatal(err)
	}
	s, err := gkvlite.NewStore(f)
	if err != nil {
		log.Fatal(err)
	}
	bots := s.GetCollection("bots")

	return &Console{
		srv:    srv,
		reader: bufio.NewReader(os.Stdin),
		bots:   bots,
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
	if c.currentBot != nil {
		slug, err := c.bots.Get([]byte("" + c.currentBot.ID))
	}
	color.Green("")
}

func (c *Console) Run() {
	for {
		c.printCommandInvitation()
		text, err := c.getInput()
		if err != nil {
			log.Println("error while getting command input: ", err)
			continue
		}
	}
}
