package main

import (
	"log"
	"os"

	"headlessdesk/internal/servercmd"
)

func main() {
	if err := servercmd.Execute(); err != nil {
		log.Printf("%v", err)
		os.Exit(1)
	}
}
