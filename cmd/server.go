package main

import (
	"flag"
	"github.com/xorium/wormwhole/server"
	"log"
)

var (
	ListenAddr string
)

func main() {
	flag.StringVar(&ListenAddr, "addr", ":39746", "addr to listen")
	flag.Parse()

	srv := server.NewCommandServer(ListenAddr)
	log.Fatal(srv.Run())
}
