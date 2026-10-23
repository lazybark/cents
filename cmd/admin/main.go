package main

import (
	"log"
	"os"

	"github.com/lazybark/cents/adminapp"
)

func main() {
	if err := adminapp.Run(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
