package system

import (
	"encoding/json"
	"fmt"
	"os"

	ctypes "github.com/pthsarmah/forge/types"
)

func SaveNodeInfo(nodeToken string, userId string, nodeId string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error while getting home directory '%s'", err)
		return err
	}

	path := "/.forge/config.json"

	file, err := os.OpenFile(homeDir+path, os.O_RDWR|os.O_CREATE, 0664)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not create/open config file '%s'", err)
		return err
	}

	stat, err := file.Stat()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not stat file '%s'", err)
		return err
	}

	var cfg ctypes.Config

	if stat.Size() > 0 {
		//read file
		data, err := os.ReadFile(homeDir + path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not read file '%s'", err)
			return err
		}

		err = json.Unmarshal(data, &cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not unmarshal to config struct '%s'", err)
			return err
		}
	}

	cfg.Nodes[userId] = ctypes.NodeInfo{
		UserId:    userId,
		NodeId:    nodeId,
		NodeToken: nodeToken,
	}

	newData, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error in marshalling new data '%s'", err)
		return err
	}

	err = file.Truncate(0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error in truncating file '%s'", err)
		return err
	}

	_, err = file.Seek(0, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error in seeking file '%s'", err)
		return err
	}

	_, err = file.Write(newData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error in writing new data '%s'", err)
		return err
	}

	fmt.Println("Updated config!")

	return nil
}
