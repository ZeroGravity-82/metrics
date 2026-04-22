package main

import . "os"

func main() {
	Exit(3) // want `os.Exit called inside main\(\)`
}
