package main

import (
	"zerogravity-82/metrics/internal/agent"
	"zerogravity-82/metrics/internal/config"
)

func main() {
	cfg := config.ParseFlags()
	agent.Run(cfg)
}
