package system

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	ctypes "github.com/pthsarmah/forge-agent/types"
)

func GetAllNodes() (map[string]ctypes.NodeInfo, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("Error in reading home director: %v", err)
	}

	configPath := filepath.Join(homeDir, ".forge/config.json")
	body, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("Error in reading config file: %v", err)
	}

	var cfg ctypes.Config

	err = json.Unmarshal(body, &cfg)
	if err != nil {
		return nil, fmt.Errorf("Error in reading config file: %v", err)
	}

	if cfg.Nodes == nil {
		err = fmt.Errorf("Config is empty")
		return nil, fmt.Errorf("Error in reading config file: %v", err)
	}

	return cfg.Nodes, nil
}

func IsNodeAlreadyConnectedToUser(userId string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("Error in reading home directory: %v", err)
	}

	configPath := filepath.Join(homeDir, ".forge/config.json")
	body, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "no_file", nil
		}
		return "", fmt.Errorf("Error in reading config file: %v", err)
	}

	var cfg ctypes.Config

	err = json.Unmarshal(body, &cfg)
	if err != nil {
		return "", fmt.Errorf("Error in reading config file: %v", err)
	}

	if cfg.Nodes == nil {
		err = fmt.Errorf("Config is empty")
		return "", fmt.Errorf("Error in reading config file: %v", err)
	}

	_, ok := cfg.Nodes[userId]
	if ok {
		return "not_connected", nil
	}

	return "connected", nil
}

func SaveNodeInfo(nodeToken string, userId string, nodeId string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {

	}

	configDir := filepath.Join(homeDir, ".forge")

	if err := os.MkdirAll(configDir, 0755); err != nil {

	}

	configPath := filepath.Join(configDir, "config.json")

	file, err := os.OpenFile(configPath, os.O_RDWR|os.O_CREATE, 0664)
	if err != nil {

	}

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("Could not stat file '%s'", err)

	}

	var cfg ctypes.Config

	if stat.Size() > 0 {
		//read file
		data, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("Could not read file '%s'", err)
		}

		err = json.Unmarshal(data, &cfg)
		if err != nil {
			return fmt.Errorf("Could not unmarshal to config struct '%s'", err)
		}
	}

	if cfg.Nodes == nil {
		cfg = ctypes.Config{
			Nodes: make(map[string]ctypes.NodeInfo),
		}
	}

	cfg.Nodes[userId] = ctypes.NodeInfo{
		NodeId:    nodeId,
		NodeToken: nodeToken,
	}

	newData, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("Error in marshalling new data '%s'", err)
	}

	err = file.Truncate(0)
	if err != nil {
		return fmt.Errorf("Error in truncating file '%s'", err)
	}

	_, err = file.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("Error in seeking file '%s'", err)
	}

	_, err = file.Write(newData)
	if err != nil {
		return fmt.Errorf("Error in writing new data '%s'", err)
	}

	fmt.Println("Updated config!")

	return nil
}
