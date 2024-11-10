package main

import (
	"log"
)

func main() {
	store, err := OpenStorage("./storage.json")
	store.CardsUpToDate()

	if err != nil {
		log.Fatal(err)
	}

	server := NewAPISErver(":3000", store)
	server.Run()

}
