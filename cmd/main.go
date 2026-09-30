package main

import (
	"log"
	"os"

	"github.com/lazybark/cents/cli"
)

func main() {
	if err := cli.NewRootCommand().Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
