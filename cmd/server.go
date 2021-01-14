package main

import "flag"

var (
	ListenAddr string
)

func main() {
	flag.StringVar(&ListenAddr, "addr", ":39746", "addr to listen")
}
