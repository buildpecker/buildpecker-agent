package main

import (
	"github.com/joho/godotenv"
	"github.com/pthsarmah/forge-agent/cmd"
	"github.com/pthsarmah/forge-agent/utils"
)

func main() {
	godotenv.Load()

	logger, err := utils.GetLoggerInstance()
	if err != nil {
		panic(err)
	}

	defer logger.Close()

	logger.ApiLogger.Println("API started")
	logger.SystemLogger.Println("System initialized")
	logger.DeployLogger.Println("Deploy subsystem initialized")

	cmd.Execute()
}
