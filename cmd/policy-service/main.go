package main

import (
	"log"

	"github.com/zeann3th/policy-service/internal/application"
)

// @title			Policy Service API
// @version		1.0
// @description	Policy configuration, custom policy, unit, and cache administration API.
// @BasePath		/policy-service/api
// @schemes		http https
func main() {
	if err := application.Run(); err != nil {
		log.Fatal(err)
	}
}
