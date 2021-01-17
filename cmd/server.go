package main

import (
	"flag"
	"github.com/xorium/wormwhole/console"
	"github.com/xorium/wormwhole/server"
)

func main() {
	var (
		listenAddr string
		debug      bool
	)

	flag.StringVar(&listenAddr, "addr", ":39746", "addr to listen")
	flag.BoolVar(&debug, "debug", false, "debug mode")
	flag.Parse()

	srv := server.NewCommandServer(listenAddr)
	srv.Debug = debug
	go srv.Run()

	c := console.NewConsole(srv)
	c.Debug = debug
	c.Run()
}
