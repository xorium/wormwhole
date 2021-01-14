package core

import (
	"fmt"
	"time"
)

type Command struct {
	ID   string        `json:"id"`
	Name string        `json:"name"`
	Args []interface{} `json:"args"`
}

func NewCommand(name string, args ...interface{}) Command {
	if len(args) == 0 {
		args = make([]interface{}, 0)
	}
	return Command{
		ID:   fmt.Sprintf("%d", time.Now().UnixNano()),
		Name: name,
		Args: args,
	}
}
