package main

import (
	"github.com/joho/godotenv"
	"github.com/pthsarmah/forge-agent/cmd"
)

func main() {
	godotenv.Load()
	cmd.Execute()
}
