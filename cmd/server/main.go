package main

import (
	"zerogravity-82/metrics/internal/application"
)

func main() {
	app := application.NewApplication()
	defer app.Close()
	app.Run()
}
