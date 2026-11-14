package main

import (
	"log"

	"github.com/zeann3th/policy-service/internal/application"
)

func main() {
	if err := application.Run(); err != nil {
		log.Fatal(err)
	}
}
