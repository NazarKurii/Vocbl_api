package main

import (
	"log"
	"os"
)

func main() {
	// Get the port from environment variables, default to 3000 if not set
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	store, err := OpenStorage("./storage.json")
	store.CardsUpToDate()

	if err != nil {
		log.Fatal(err)
	}

	// Use the port from the environment variable
	server := NewAPISErver(":"+port, store)
	server.Run()
}
