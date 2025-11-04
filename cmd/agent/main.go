package main

import (
	"log"

	"zerogravity-82/metrics/internal/agent"
	"zerogravity-82/metrics/internal/config"
)

func main() {
	cfg, err := config.GetAgentConfig()
	if err != nil {
		log.Fatal(err)
	}
	agent.Run(cfg)
}
