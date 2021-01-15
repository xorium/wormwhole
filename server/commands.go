package server

import "github.com/xorium/wormwhole/core"

func ExecCommand(cmd string) *core.Command {
	return core.NewCommand("exec", cmd)
}

func PingCommand() *core.Command {
	return core.NewCommand("ping")
}
