package system

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	ctypes "github.com/pthsarmah/forge-agent/types"
	"github.com/pthsarmah/forge-agent/utils"
)

func SetupCloudflared(tunnelToken string) error {
	logger, _ := utils.GetLoggerInstance()

	if strings.TrimSpace(tunnelToken) == "" {
		return fmt.Errorf("empty cloudflare tunnel token")
	}

	bin, err := exec.LookPath("cloudflared")
	if err != nil {
		return fmt.Errorf("cloudflared binary not on PATH (install.sh installs it): %v", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not resolve home directory: %v", err)
	}

	forgeDir := filepath.Join(homeDir, ".forge")
	if err := os.MkdirAll(forgeDir, 0755); err != nil {
		return fmt.Errorf("could not create %s: %v", forgeDir, err)
	}

	tokenPath := filepath.Join(forgeDir, "cloudflared.token")
	prev, _ := os.ReadFile(tokenPath)
	running := false
	if out, err := exec.Command("pgrep", "-f", "cloudflared tunnel.*run").Output(); err == nil && strings.TrimSpace(string(out)) != "" {
		running = true
	}

	if running && strings.TrimSpace(string(prev)) == strings.TrimSpace(tunnelToken) {
		logger.SystemLogger.Println("cloudflared tunnel already running with current token")
		return nil
	}

	if err := os.WriteFile(tokenPath, []byte(tunnelToken), 0600); err != nil {
		return fmt.Errorf("could not persist cloudflared token: %v", err)
	}
	if err := os.Chmod(tokenPath, 0600); err != nil {
		return fmt.Errorf("could not secure cloudflared token: %v", err)
	}

	if running {
		_ = exec.Command("pkill", "-f", "cloudflared tunnel.*run").Run()
		logger.SystemLogger.Println("stopped stale cloudflared (tunnel token changed)")
	}

	logFile, err := os.OpenFile(filepath.Join(forgeDir, "cloudflared.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("could not open cloudflared log: %v", err)
	}
	defer logFile.Close()

	cmd := exec.Command(bin, "tunnel", "--no-autoupdate", "run", "--token", tunnelToken)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not start cloudflared: %v", err)
	}

	if err := cmd.Process.Release(); err != nil {
		logger.SystemLogger.Printf("cloudflared process release warning: %v", err)
	}

	logger.SystemLogger.Println("cloudflared tunnel started")
	return nil
}

func GetAllNodes() (map[string]ctypes.NodeInfo, error) {
	logger, _ := utils.GetLoggerInstance()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		logger.SystemLogger.Printf("Read home dir failed: %v", err)
		return nil, fmt.Errorf("Error in reading home director: %v", err)
	}

	configPath := filepath.Join(homeDir, ".forge/config.json")
	body, err := os.ReadFile(configPath)
	if err != nil {
		logger.SystemLogger.Printf("Read config %s failed: %v", configPath, err)
		return nil, fmt.Errorf("Error in reading config file: %v", err)
	}

	var cfg ctypes.Config

	err = json.Unmarshal(body, &cfg)
	if err != nil {
		logger.SystemLogger.Printf("Unmarshal config failed: %v", err)
		return nil, fmt.Errorf("Error in reading config file: %v", err)
	}

	if cfg.Nodes == nil {
		logger.SystemLogger.Println("Config is empty")
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
		return fmt.Errorf("[SaveNodeInfo] Could not load home directory: %v", err)
	}

	configDir := filepath.Join(homeDir, ".forge")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("[SaveNodeInfo] Could not create config directory: %v", err)
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

	logger, _ := utils.GetLoggerInstance()
	logger.SystemLogger.Printf("Saved node info userId=%s nodeId=%s", userId, nodeId)

	return nil
}
