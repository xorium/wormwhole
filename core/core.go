package core

import (
	"fmt"
	"sync"
	"time"
)

type CommandState string

const (
	CommandStateUndefined   CommandState = "undefined"
	CommandStateExecuting   CommandState = "executing"
	CommandStateSuccess     CommandState = "success"
	CommandStateFailed      CommandState = "failed"
	CommandStateInterrupted CommandState = "interrupted"
)

const (
	CommandResultCodeSuccess = "success"
	CommandResultCodeError   = "error"
)

type Command struct {
	*sync.RWMutex
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Args     []interface{} `json:"args"`
	state    CommandState
	targetId string
}

func NewCommand(name string, args ...interface{}) *Command {
	if len(args) == 0 {
		args = make([]interface{}, 0)
	}
	return &Command{
		RWMutex: new(sync.RWMutex),
		ID:      fmt.Sprintf("%d", time.Now().UnixNano()),
		Name:    name,
		Args:    args,
		state:   CommandStateUndefined,
	}
}

func (c *Command) String() string {
	return fmt.Sprintf("%s <%s>", c.Name, c.state)
}

func (c *Command) SetState(state CommandState) {
	c.Lock()
	c.state = state
	c.Unlock()
}

func (c *Command) State() CommandState {
	c.RLock()
	defer c.Unlock()
	return c.state
}

func (c *Command) SetTarget(botId string) {
	c.Lock()
	c.targetId = botId
	c.Unlock()
}

func (c *Command) Target() string {
	c.RLock()
	defer c.Unlock()
	return c.targetId
}
