package main

import (
	"flag"
	"github.com/xorium/wormwhole/console"
	"github.com/xorium/wormwhole/server"
)

var (
	ListenAddr string
	Debug      bool
)

func main() {
	flag.StringVar(&ListenAddr, "addr", ":39746", "addr to listen")
	flag.BoolVar(&Debug, "debug", false, "debug mode")
	flag.Parse()

	srv := server.NewCommandServer(ListenAddr)
	srv.Debug = Debug
	go srv.Run()

	c := console.NewConsole(srv)
	c.Debug = Debug
	c.Run()
}
