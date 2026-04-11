package main

import (
	"os"

	"github.com/ashcastle/mipconfig/internal/ipconfig"
)

func main() {
	os.Exit(ipconfig.Run(os.Stdout, os.Stderr, os.Args[1:]))
}
