package main

import (
	"fmt"
	"os"
)

const version = "0.1.0"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("claudelens %s\n", version)
		os.Exit(0)
	}

	fmt.Println("ClaudeLens — session discovery TUI (coming soon)")
}
