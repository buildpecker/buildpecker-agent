package utils

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

type Logger struct {
	ApiLogger    *log.Logger
	SystemLogger *log.Logger
	DeployLogger *log.Logger

	apiFile    *os.File
	sysFile    *os.File
	deployFile *os.File

	deployments map[string]*DeploymentLogger
	mu          sync.Mutex

	baseDir string
}

type DeploymentLogger struct {
	Logger *log.Logger
	File   *os.File
}

var (
	instance *Logger
	once     sync.Once
	initErr  error
)

func newLogger(path string, prefix string) (*log.Logger, *os.File, error) {
	f, err := os.OpenFile(
		path,
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0o644,
	)
	if err != nil {
		return nil, nil, err
	}

	logger := log.New(
		f,
		prefix+" ",
		log.LstdFlags|log.Lmicroseconds|log.LUTC,
	)

	return logger, f, nil
}

func GetLoggerInstance() (*Logger, error) {
	once.Do(func() {
		home, err := os.UserHomeDir()
		if err != nil {
			initErr = err
			return
		}

		baseDir := filepath.Join(home, ".buildpecker", "logs")

		if err := os.MkdirAll(baseDir, 0o755); err != nil {
			initErr = err
			return
		}

		deploymentsDir := filepath.Join(baseDir, "deployments")

		if err := os.MkdirAll(deploymentsDir, 0o755); err != nil {
			initErr = err
			return
		}

		apiLogger, apiFile, err := newLogger(
			filepath.Join(baseDir, "api.log"),
			"[API]",
		)
		if err != nil {
			initErr = err
			return
		}

		systemLogger, sysFile, err := newLogger(
			filepath.Join(baseDir, "system.log"),
			"[SYSTEM]",
		)
		if err != nil {
			_ = apiFile.Close()
			initErr = err
			return
		}

		deployLogger, deployFile, err := newLogger(
			filepath.Join(baseDir, "deploy.log"),
			"[DEPLOY]",
		)
		if err != nil {
			_ = apiFile.Close()
			_ = sysFile.Close()
			initErr = err
			return
		}

		instance = &Logger{
			ApiLogger:    apiLogger,
			SystemLogger: systemLogger,
			DeployLogger: deployLogger,
			apiFile:      apiFile,
			sysFile:      sysFile,
			deployFile:   deployFile,
			deployments:  make(map[string]*DeploymentLogger),
			baseDir:      baseDir,
		}
	})

	return instance, initErr
}

func (l *Logger) GetDeploymentLogger(deploymentID string) (*log.Logger, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if existing, ok := l.deployments[deploymentID]; ok {
		return existing.Logger, nil
	}

	logPath := filepath.Join(
		l.baseDir,
		"deployments",
		fmt.Sprintf("%s.log", deploymentID),
	)

	logger, file, err := newLogger(
		logPath,
		fmt.Sprintf("[DEPLOYMENT:%s]", deploymentID),
	)
	if err != nil {
		return nil, err
	}

	l.deployments[deploymentID] = &DeploymentLogger{
		Logger: logger,
		File:   file,
	}

	return logger, nil
}

func (l *Logger) Close() error {
	if l.apiFile != nil {
		_ = l.apiFile.Close()
	}

	if l.sysFile != nil {
		_ = l.sysFile.Close()
	}

	if l.deployFile != nil {
		_ = l.deployFile.Close()
	}

	for _, deployment := range l.deployments {
		if deployment.File != nil {
			_ = deployment.File.Close()
		}
	}

	return nil
}
