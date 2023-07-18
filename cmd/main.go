package main

import (
	"github.com/cccxg/socks4"
)

func main() {
	srv := socks4.NewServer()
	srv.Run(":1080")
}
