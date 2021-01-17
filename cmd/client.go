package main

import (
	"flag"
	"github.com/xorium/wormwhole/client"
)

func main() {
	var (
		serverAddr = "127.0.0.1:39746"
		debug      = false
	)

	flag.StringVar(&serverAddr, "addr", "127.0.0.1:39746", "server address")
	flag.BoolVar(&debug, "debug", false, "debug mode")
	flag.Parse()

	cli := client.NewClient(serverAddr)
	cli.Debug = debug
	cli.Run()
}
