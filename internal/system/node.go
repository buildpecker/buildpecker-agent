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
		fmt.Fprintf(os.Stderr, "Error in reading home director: %v", err)
		return nil, err
	}

	configPath := filepath.Join(homeDir, ".forge/config.json")
	body, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error in reading config file: %v", err)
		return nil, err
	}

	var cfg ctypes.Config

	err = json.Unmarshal(body, &cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error in reading config file: %v", err)
		return nil, err
	}

	if cfg.Nodes == nil {
		err = fmt.Errorf("Config is empty")
		fmt.Fprintf(os.Stderr, "Error in reading config file: %v", err)
		return nil, err
	}

	return cfg.Nodes, nil
}

func IsNodeAlreadyConnectedToUser(userId string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error in reading home directory: %v", err)
		return "", err
	}

	configPath := filepath.Join(homeDir, ".forge/config.json")
	body, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "no_file", err
		}
		fmt.Fprintf(os.Stderr, "Error in reading config file: %v", err)
		return "", err
	}

	var cfg ctypes.Config

	err = json.Unmarshal(body, &cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error in reading config file: %v", err)
		return "", err
	}

	if cfg.Nodes == nil {
		err = fmt.Errorf("Config is empty")
		fmt.Fprintf(os.Stderr, "Error in reading config file: %v", err)
		return "", err
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
		return err
	}

	configDir := filepath.Join(homeDir, ".forge")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	configPath := filepath.Join(configDir, "config.json")

	file, err := os.OpenFile(configPath, os.O_RDWR|os.O_CREATE, 0664)
	if err != nil {
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
		data, err := os.ReadFile(configPath)
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
