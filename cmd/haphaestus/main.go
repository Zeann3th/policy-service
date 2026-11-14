package main

import (
	"log"

	"github.com/zeann3th/haphaestus/internal/application"
)

func main() {
	if err := application.Run(); err != nil {
		log.Fatal(err)
	}
}
