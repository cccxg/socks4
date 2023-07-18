package main

import (
	"os"

	"github.com/cccxg/socks4"
	"github.com/sirupsen/logrus"
)

func main() {
	srv := socks4.NewServer(socks4.WithLogger(&logrus.Logger{
		Out: os.Stdout,
	}))
	srv.Run(":1080")
}
