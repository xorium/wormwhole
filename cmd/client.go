package main

import (
	"flag"
	"github.com/xorium/wormwhole/client"
)

func main() {
	var (
		inProto    = "ws"
		serverAddr = "127.0.0.1:39746"
		debug      = false
	)

	flag.StringVar(&serverAddr, "addr", "ws://127.0.0.1:39746", "server address")
	flag.StringVar(&inProto, "proto", "ws", "connection protocol")
	flag.BoolVar(&debug, "debug", false, "debug mode")
	flag.Parse()

	cli := client.NewClient(serverAddr, inProto)
	cli.Debug = debug
	cli.Run()
}
