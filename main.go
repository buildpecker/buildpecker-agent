package main

import (
	"github.com/joho/godotenv"
	"github.com/pthsarmah/forge/cmd"
)

func main() {
	godotenv.Load()
	cmd.Execute()
}
