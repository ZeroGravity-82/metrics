package main

import o "os"

func main() {
	o.Exit(2) // want `os.Exit called inside main\(\)`
}
