package deploy

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	ctypes "github.com/pthsarmah/buildpecker-agent/types"
	"github.com/pthsarmah/buildpecker-agent/utils"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	// A cold nixpacks build (nix store fetch plus docker build) is legitimately
	// slow; past this it is wedged, not working.
	nixpacksBuildTimeout = 30 * time.Minute
	// Build output is near continuous, so a long silence means a stuck step
	// (unreachable registry, hung nix fetch) rather than slow progress.
	nixpacksBuildIdleTimeout = 8 * time.Minute

	// docker run -d on an image that already exists locally is quick.
	dockerRunTimeout     = 3 * time.Minute
	dockerRunIdleTimeout = 2 * time.Minute

	dockerRemoveTimeout = 60 * time.Second

	// Cadence of "still running" lines while a command produces no output, so a
	// tailing user always sees the pipeline is alive.
	progressInterval = 30 * time.Second

	// Grace between killing a timed out process group and abandoning its pipes.
	killGraceDelay = 10 * time.Second

	// Longest single log line kept intact; past this the scanner gives up and
	// the rest of the stream is drained instead of streamed.
	maxLogLine = 1024 * 1024
)

// cmdLimits bounds one streamed command. Any field left zero disables that
// particular guard.
type cmdLimits struct {
	total     time.Duration // hard cap on the whole command
	idle      time.Duration // abort after this long with no output at all
	heartbeat time.Duration // cadence of progress lines while output is silent
}

// runStreaming runs name/args under lim and fans every stdout/stderr line,
// live, into each non-nil sink. Replaces CombinedOutput so a long build is
// tailable per deployment instead of dumped once on exit, and bounds the
// command so a wedged build fails with a reason instead of hanging the
// deployment forever. label prefixes the timeout and progress lines.
func runStreaming(ctx context.Context, lim cmdLimits, label, name string, args []string, sinks ...*log.Logger) error {
	emit := func(format string, a ...any) {
		for _, s := range sinks {
			if s != nil {
				s.Printf(format, a...)
			}
		}
	}

	if lim.total > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, lim.total)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, name, args...)
	// nixpacks and docker both fork children; put the command in its own
	// process group so a timeout takes the whole tree down with it.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Signalling the group by negated pid is only safe until Wait reaps the
	// process: after that the kernel may hand the pid to somebody else and the
	// kill would land on an unrelated group. Both killers (os/exec's cancel
	// goroutine and our watchdog) can still be in flight then, so gate them on
	// the same reaped flag.
	var (
		procMu sync.Mutex
		reaped bool
	)
	killTree := func() error {
		procMu.Lock()
		defer procMu.Unlock()
		if reaped || cmd.Process == nil {
			return os.ErrProcessDone
		}
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
			return cmd.Process.Kill()
		}
		return nil
	}

	cmd.Cancel = killTree
	cmd.WaitDelay = killGraceDelay

	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout // fold stderr into the same pipe

	started := time.Now()
	if err := cmd.Start(); err != nil {
		return err
	}

	var lastOutput atomic.Int64
	lastOutput.Store(started.UnixNano())

	var stalled atomic.Bool
	watchDone := make(chan struct{})
	if tick := watchInterval(lim); tick > 0 {
		go func() {
			t := time.NewTicker(tick)
			defer t.Stop()
			for {
				select {
				case <-watchDone:
					return
				case now := <-t.C:
					silent := now.Sub(time.Unix(0, lastOutput.Load()))
					if lim.idle > 0 && silent >= lim.idle {
						stalled.Store(true)
						emit("%s: no output for %s, aborting", label, lim.idle)
						_ = killTree()
						return
					}
					if lim.heartbeat > 0 && silent >= lim.heartbeat {
						emit("%s: still running, elapsed %s (silent for %s)",
							label,
							now.Sub(started).Round(time.Second),
							silent.Round(time.Second),
						)
					}
				}
			}
		}()
	}

	sc := bufio.NewScanner(pipe)
	sc.Buffer(make([]byte, 0, 64*1024), maxLogLine)
	for sc.Scan() {
		lastOutput.Store(time.Now().UnixNano())
		line := sc.Text()
		for _, s := range sinks {
			if s != nil {
				s.Print(line)
			}
		}
	}
	if scanErr := sc.Err(); scanErr != nil {
		// Nobody is reading the pipe any more, so the command would block on a
		// full buffer and the watchdog would report a stall that never
		// happened. Say what actually went wrong, then keep the pipe moving.
		emit("%s: log stream stopped (%v); remaining output dropped", label, scanErr)
		drain(pipe, &lastOutput)
	}

	err = cmd.Wait()
	procMu.Lock()
	reaped = true
	procMu.Unlock()
	close(watchDone)

	switch {
	case stalled.Load():
		return fmt.Errorf("%s produced no output for %s and was killed after %s",
			label, lim.idle, time.Since(started).Round(time.Second))
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return fmt.Errorf("%s timed out after %s", label, lim.total)
	case err != nil:
		return err
	}
	return nil
}

// watchInterval is how often the watchdog wakes: fine enough to honour the
// tighter of the two guards, zero when neither is set.
func watchInterval(lim cmdLimits) time.Duration {
	tick := lim.heartbeat
	if lim.idle > 0 && (tick <= 0 || lim.idle < tick) {
		tick = lim.idle
	}
	return tick
}

// drain discards whatever the command still writes after the line scanner gave
// up, so the child never blocks on a full pipe. It keeps the last-output clock
// fresh: a command in this state is still making progress, just unloggable.
func drain(r io.Reader, lastOutput *atomic.Int64) {
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			lastOutput.Store(time.Now().UnixNano())
		}
		if err != nil {
			return
		}
	}
}

func filterPublicEnvs(envs []ctypes.EnvVar, prefixes []string) []ctypes.EnvVar {
	if len(prefixes) == 0 {
		return nil
	}
	var out []ctypes.EnvVar
	for _, e := range envs {
		for _, p := range prefixes {
			if strings.HasPrefix(e.Key, p) {
				out = append(out, e)
				break
			}
		}
	}
	return out
}

func freeHostPort() (int, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func updateIgnoreFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	var lines []string
	hasUnignore := false

	for line := range strings.SplitSeq(string(data), "\n") {
		trimmed := strings.TrimSpace(line)

		switch trimmed {
		case ".nixpacks", ".nixpacks/", "/.nixpacks", "/.nixpacks/":
			// remove ignore rule
			continue
		case "!.nixpacks", "!/.nixpacks":
			hasUnignore = true
		}

		lines = append(lines, line)
	}

	if !hasUnignore {
		lines = append(lines,
			"!.nixpacks",
			"!.nixpacks/**",
		)
	}

	content := strings.Join(lines, "\n")
	return os.WriteFile(path, []byte(content), 0644)
}

func writeNixpacksConfig(cfg string) (string, error) {
	dir, err := os.MkdirTemp("", "nixpacks-*")
	if err != nil {
		return "", err
	}

	cfgPath := filepath.Join(dir, "nixpacks.toml")

	err = os.WriteFile(cfgPath, []byte(cfg), 0644)
	if err != nil {
		return "", err
	}

	return cfgPath, nil
}

func NixpackDeploy(dep ctypes.Deployment, envs []ctypes.EnvVar, projectPath string, framework ctypes.FrameworkInfo) (int, error) {

	var nixpackEnvs = map[string]string{
		"NIXPACKS_NODE_VERSION":    "22",
		"NIXPACKS_NIXPKGS_ARCHIVE": "51ad838b03a05b1de6f9f2a0fffecee64a9788ee",
	}

	//set .gitignore or .dockerignore flags to allow .nixpacks
	noGitPath := strings.TrimSuffix(projectPath, ".git")
	for _, name := range []string{".gitignore", ".dockerignore"} {
		path := filepath.Join(noGitPath, name)

		if err := updateIgnoreFile(path); err != nil {
			return 0, fmt.Errorf("could not update %s: %w", name, err)
		}
	}

	logger, _ := utils.GetLoggerInstance()
	depLog, _ := logger.GetDeploymentLogger(dep.Id)

	ctx := context.Background()

	imageName := strings.TrimSuffix(path.Base(projectPath), ".git")
	versionNo := "v1"

	logger.DeployLogger.Printf("Nixpack build start dep=%s image=%s:%s framework=%s", dep.Id, imageName, versionNo, framework.DisplayName)
	if depLog != nil {
		depLog.Printf("Nixpack build start image=%s:%s framework=%s", imageName, versionNo, framework.DisplayName)
	}

	//nixpack build
	nixargs := []string{
		"build", projectPath,
		"--name", fmt.Sprintf("%s:%s", imageName, versionNo),
	}

	//custom toml for static builds if needed
	if framework.NixpacksToml != "" {
		cfgPath, err := writeNixpacksConfig(framework.NixpacksToml)
		if err != nil {
			return 0, err
		}

		nixargs = append(nixargs, "--config", cfgPath)
		nixpackEnvs["NIXPACKS_CONFIG_FILE"] = cfgPath
	}

	runtimePkgs := []string{"curl"}
	if pkgs := DetectNativePkgs(projectPath); len(pkgs) > 0 {
		runtimePkgs = append(runtimePkgs, pkgs...)
	}
	logger.DeployLogger.Printf("Injecting build/runtime pkgs dep=%s pkgs=%v", dep.Id, runtimePkgs)
	if depLog != nil {
		depLog.Printf("Injecting build/runtime pkgs: %v", runtimePkgs)
	}
	nixargs = append(nixargs, "--pkgs", strings.Join(runtimePkgs, " "))

	for k, v := range nixpackEnvs {
		nixargs = append(nixargs,
			"--env",
			k+"="+v,
		)
	}

	buildEnvs := filterPublicEnvs(envs, framework.PublicEnvPrefixes)
	for _, e := range buildEnvs {
		nixargs = append(nixargs,
			"--env",
			e.Key+"="+e.Value,
		)
	}
	if len(buildEnvs) > 0 {
		logger.DeployLogger.Printf("Injecting %d public build envs dep=%s framework=%s", len(buildEnvs), dep.Id, framework.Id)
		if depLog != nil {
			depLog.Printf("Injecting %d public build envs", len(buildEnvs))
		}
	}

	buildLimits := cmdLimits{
		total:     nixpacksBuildTimeout,
		idle:      nixpacksBuildIdleTimeout,
		heartbeat: progressInterval,
	}

	if err := runStreaming(ctx, buildLimits, "nixpacks build", "nixpacks", nixargs, logger.DeployLogger, depLog); err != nil {
		logger.DeployLogger.Printf("Nixpack build failed dep=%s: %v", dep.Id, err)
		if depLog != nil {
			depLog.Printf("Nixpack build failed: %v", err)
		}
		return 0, fmt.Errorf("could not run command: %w", err)
	}

	hostPort, err := freeHostPort()
	if err != nil {
		logger.DeployLogger.Printf("Allocate host port failed dep=%s: %v", dep.Id, err)
		if depLog != nil {
			depLog.Printf("Allocate host port failed: %v", err)
		}
		return 0, fmt.Errorf("could not allocate host port: %w", err)
	}

	rmCtx, cancelRm := context.WithTimeout(ctx, dockerRemoveTimeout)
	rmCmd := exec.CommandContext(rmCtx, "docker", "rm", "-f", imageName)
	// Without a WaitDelay the deadline kills docker but Wait still blocks on
	// the output pipe for as long as any inherited child holds it open.
	rmCmd.WaitDelay = killGraceDelay
	out, rmErr := rmCmd.CombinedOutput()
	cancelRm()
	if rmErr != nil {
		// A deadline here means the docker daemon itself is unresponsive, so
		// the run below would hang too: fail now with a reason.
		if errors.Is(rmCtx.Err(), context.DeadlineExceeded) {
			logger.DeployLogger.Printf("Remove previous container timed out dep=%s name=%s after %s",
				dep.Id, imageName, dockerRemoveTimeout)
			if depLog != nil {
				depLog.Printf("Removing previous container timed out after %s; docker may be unresponsive", dockerRemoveTimeout)
			}
			return 0, fmt.Errorf("docker rm timed out after %s", dockerRemoveTimeout)
		}
		logger.DeployLogger.Printf("No previous container to remove dep=%s name=%s: %s",
			dep.Id, imageName, strings.TrimSpace(string(out)))
	} else {
		logger.DeployLogger.Printf("Removed previous container dep=%s name=%s", dep.Id, imageName)
		if depLog != nil {
			depLog.Printf("Removed previous container name=%s", imageName)
		}
	}

	baseURL := strings.TrimRight(os.Getenv("CONVEX_SITE_URL"), "/")
	if baseURL == "" {
		return 0, fmt.Errorf("CONVEX_SITE_URL is empty")
	}

	healthURL := fmt.Sprintf("%s/deployments/health/%s?token=%s", baseURL, dep.Id, dep.HealthToken)

	args := []string{
		"run",
		"-d",
		"--name", imageName,
		"--restart", "unless-stopped",
		"--health-cmd", fmt.Sprintf("wget --no-verbose --tries=1 --spider '%s' || exit 1", healthURL),
		"--network", "buildpecker",
		"--health-interval", "30s",
		"--health-timeout", "5s",
		"--health-retries", "3",
		"--health-start-period", "10s",
		"-p", fmt.Sprintf("127.0.0.1:%d:%d", hostPort, framework.DefaultPort),
	}

	for _, e := range envs {
		args = append(args,
			"--env",
			e.Key+"="+e.Value,
		)
	}

	args = append(args,
		fmt.Sprintf("%s:%s", imageName, versionNo),
	)

	runLimits := cmdLimits{
		total:     dockerRunTimeout,
		idle:      dockerRunIdleTimeout,
		heartbeat: progressInterval,
	}

	if err := runStreaming(ctx, runLimits, "docker run", "docker", args, logger.DeployLogger, depLog); err != nil {
		logger.DeployLogger.Printf("Docker run failed dep=%s: %v", dep.Id, err)
		if depLog != nil {
			depLog.Printf("Docker run failed: %v", err)
		}
		return 0, fmt.Errorf("could not run command: %w", err)
	}

	logger.DeployLogger.Printf("Container running dep=%s image=%s port=%d", dep.Id, imageName, hostPort)
	if depLog != nil {
		depLog.Printf("Container running image=%s port=%d", imageName, hostPort)
	}

	return hostPort, nil
}
