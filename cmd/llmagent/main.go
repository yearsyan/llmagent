package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/yearsyan/agentd/internal/config"
	"github.com/yearsyan/agentd/internal/daemon"
	"github.com/yearsyan/agentd/internal/skill"
	"github.com/yearsyan/agentd/internal/summary"
)

const version = "0.1.1"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	for _, a := range os.Args {
		if a == "-h" || a == "--help" {
			usage()
			os.Exit(0)
		}
		if a == "--version" || a == "-v" {
			fmt.Println("llmagent", version)
			os.Exit(0)
		}
	}

	switch os.Args[1] {
	case "daemon":
		daemonCmd(os.Args[2:])
	case "run":
		runCmd(os.Args[2:])
	case "models":
		modelsCmd()
	case "test":
		testCmd(os.Args[2:])
	case "test-summary":
		testSummaryCmd()
	case "install-skills":
		installSkills()
	default:
		runCmd(os.Args[1:])
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `llmagent — multi-backend LLM agent (`+version+`)

Usage:
  llmagent run -m <model> "prompt"   Execute prompt with model's backend
  llmagent -m <model> "prompt"       Short form of run
  llmagent -m <model> --summary "p"  Execute, then summarize via OpenAI-compatible API
  llmagent daemon start              Start background daemon
  llmagent daemon stop               Stop daemon
  llmagent daemon status             Show daemon status + sessions
  llmagent daemon attach <id>        Attach to a session
  llmagent models                    List configured models
  llmagent test ["prompt"]           Test all models with an optional custom prompt
  llmagent test-summary              Test the summary API configuration
  llmagent install-skills            Install llmagent as a Claude Code skill`)
}

func daemonCmd(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: llmagent daemon <start|stop|status|attach>")
		os.Exit(1)
	}

	switch args[0] {
	case "__serve__":
		serveDaemon()
	case "start":
		handleDaemonStart()
	case "stop":
		handleDaemonStop()
	case "status":
		handleDaemonStatus()
	case "attach":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: llmagent daemon attach <session-id>")
			os.Exit(1)
		}
		handleDaemonAttach(args[1])
	default:
		fmt.Fprintf(os.Stderr, "unknown daemon command: %s\n", args[0])
		os.Exit(1)
	}
}

func ensureDaemon(sock string) {
	if daemon.IsRunning(sock) {
		return
	}
	startDaemon(sock)
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		if daemon.IsRunning(sock) {
			return
		}
	}
	fmt.Fprintf(os.Stderr, "daemon failed to start within 5s\n")
	os.Exit(1)
}

func handleDaemonStart() {
	cfg := mustLoadConfig()
	startDaemon(cfg.Daemon.Socket)
}

func handleDaemonStop() {
	cfg := mustLoadConfig()
	stopDaemon(cfg.Daemon.Socket)
}

func handleDaemonStatus() {
	cfg := mustLoadConfig()
	sock := cfg.Daemon.Socket

	if !daemon.IsRunning(sock) {
		fmt.Println("daemon is not running")
		return
	}

	pid, _ := daemon.ReadPID(sock)
	fmt.Printf("daemon: running (pid=%d, socket=%s)\n", pid, sock)

	sessions, err := daemon.ListSessions(sock)
	if err != nil {
		fmt.Fprintf(os.Stderr, "list sessions: %v\n", err)
		return
	}

	if len(sessions) == 0 {
		fmt.Println("sessions: none")
		return
	}

	fmt.Println("sessions:")
	for _, s := range sessions {
		fmt.Printf("  %s  %-8s  %-10s  %s\n", s.ID, s.Status, s.Model, truncate(s.Prompt, 50))
	}
}

func handleDaemonAttach(id string) {
	cfg := mustLoadConfig()
	sock := cfg.Daemon.Socket
	ensureDaemon(sock)

	if err := daemon.AttachSession(sock, id); err != nil {
		fmt.Fprintf(os.Stderr, "attach error: %v\n", err)
		os.Exit(1)
	}
}

func runCmd(args []string) {
	var modelID string
	var prompt string
	var async bool
	var summaryMode bool
	var summaryPrompt string

	idx := 0
	if len(args) > 0 && args[0] == "run" {
		idx = 1
	}

	for idx < len(args) {
		if args[idx] == "-m" || args[idx] == "--model" {
			if idx+1 < len(args) {
				modelID = args[idx+1]
				idx += 2
			} else {
				fmt.Fprintln(os.Stderr, "-m requires a model ID")
				os.Exit(1)
			}
		} else if args[idx] == "--async" {
			async = true
			idx++
		} else if args[idx] == "--summary" {
			summaryMode = true
			idx++
		} else if args[idx] == "--summary-prompt" {
			if idx+1 < len(args) {
				summaryPrompt = args[idx+1]
				idx += 2
			} else {
				fmt.Fprintln(os.Stderr, "--summary-prompt requires a string argument")
				os.Exit(1)
			}
		} else {
			prompt = args[idx]
			idx++
		}
	}

	if modelID == "" {
		fmt.Fprintln(os.Stderr, "model ID is required (-m <model>)")
		usage()
		os.Exit(1)
	}
	if prompt == "" {
		fmt.Fprintln(os.Stderr, "prompt is required")
		usage()
		os.Exit(1)
	}

	cfg := mustLoadConfig()
	sock := cfg.Daemon.Socket

	if _, err := cfg.GetModelConfig(modelID); err != nil {
		fmt.Fprintf(os.Stderr, "model error: %v\n", err)
		os.Exit(1)
	}

	if summaryMode {
		runWithSummary(cfg, sock, modelID, prompt, summaryPrompt)
		return
	}

	if daemon.IsRunning(sock) {
		if err := daemon.CreateSession(sock, modelID, prompt); err != nil {
			fmt.Fprintf(os.Stderr, "daemon error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	startDaemon(sock)
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		if daemon.IsRunning(sock) {
			break
		}
	}

	if !daemon.IsRunning(sock) {
		fmt.Fprintf(os.Stderr, "daemon failed to start\n")
		os.Exit(1)
	}

	if async {
		sid, err := daemon.CreateSessionAsync(sock, modelID, prompt)
		if err != nil {
			fmt.Fprintf(os.Stderr, "daemon error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("[session %s]\n", sid)
		return
	}

	if err := daemon.CreateSession(sock, modelID, prompt); err != nil {
		fmt.Fprintf(os.Stderr, "daemon error: %v\n", err)
		os.Exit(1)
	}
}

func startDaemon(sock string) {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot find self: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command(exe, "daemon", "__serve__")
	cmd.Env = append(os.Environ(), "LLMAGENT_SERVE_SOCK="+sock)
	cmd.Stdout = nil
	cmd.Stderr = nil
	daemon.SetDaemonAttr(cmd)

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "start daemon: %v\n", err)
		os.Exit(1)
	}

	daemon.WritePID(sock, cmd.Process.Pid)
	fmt.Printf("daemon started (pid=%d, socket=%s)\n", cmd.Process.Pid, sock)
}

func serveDaemon() {
	sock := os.Getenv("LLMAGENT_SERVE_SOCK")
	if sock == "" {
		fmt.Fprintln(os.Stderr, "LLMAGENT_SERVE_SOCK not set")
		os.Exit(1)
	}

	sigCh := make(chan os.Signal, 1)
	daemon.NotifyShutdown(sigCh)
	go func() {
		<-sigCh
		daemon.Cleanup(sock)
		os.Exit(0)
	}()

	if err := daemon.Serve(sock); err != nil {
		fmt.Fprintf(os.Stderr, "daemon error: %v\n", err)
		os.Exit(1)
	}
}

func stopDaemon(sock string) {
	pid, err := daemon.ReadPID(sock)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot read pid: %v (is daemon running?)\n", err)
		os.Exit(1)
	}

	if err := daemon.TerminateProcess(pid); err != nil {
		fmt.Fprintf(os.Stderr, "stop daemon: %v\n", err)
		os.Exit(1)
	}

	daemon.Cleanup(sock)
	fmt.Println("daemon stopped")
}

func mustLoadConfig() *config.Config {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "using config: %s\n", cfg.LoadedPath)
	return cfg
}

func installSkills() {
	paths, err := skill.Install()
	if err != nil {
		fmt.Fprintf(os.Stderr, "install skill: %v\n", err)
		os.Exit(1)
	}
	for _, p := range paths {
		fmt.Println(p)
	}
}

func modelsCmd() {
	cfg := mustLoadConfig()
	for id, mc := range cfg.Models {
		fmt.Printf("%-24s %-12s %s\n", id, mc.Backend, mc.Description)
	}
}

func testCmd(args []string) {
	prompt := "reply with exactly 'ok' and nothing else"
	if len(args) > 0 {
		prompt = args[0]
	}

	cfg := mustLoadConfig()
	sock := cfg.Daemon.Socket
	ensureDaemon(sock)

	for id := range cfg.Models {
		fmt.Printf("testing %s ... ", id)
		code, err := daemon.TestSession(sock, id, prompt)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			continue
		}
		if code == 0 {
			fmt.Println("PASS")
		} else {
			fmt.Printf("FAIL (exit=%d)\n", code)
		}
	}
}

func runWithSummary(cfg *config.Config, sock, modelID, prompt, customPrompt string) {
	if !cfg.HasSummaryConfig() {
		fmt.Fprintln(os.Stderr, "summary mode requires summary.base_url and summary.api_key in config")
		os.Exit(1)
	}

	ensureDaemon(sock)

	sessOut, err := daemon.CaptureSession(sock, modelID, prompt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "daemon error: %v\n", err)
		os.Exit(1)
	}

	var allLines []string
	allLines = append(allLines, sessOut.Stdout...)
	allLines = append(allLines, sessOut.Stderr...)
	content := strings.Join(allLines, "\n")

	sysPrompt := customPrompt
	if sysPrompt == "" {
		sysPrompt = cfg.GetSummaryPrompt()
	}

	fmt.Fprintf(os.Stderr, "[session %s] summarizing with %s...\n", sessOut.SessionID, cfg.Summary.Model)
	result, err := summary.Summarize(cfg.Summary, sysPrompt, content)
	if err != nil {
		fmt.Fprintf(os.Stderr, "summary error: %v\n", err)
		// Print raw output as fallback
		fmt.Println(content)
		os.Exit(1)
	}

	fmt.Println(result)
	if sessOut.ExitCode != 0 {
		os.Exit(sessOut.ExitCode)
	}
}

func testSummaryCmd() {
	cfg := mustLoadConfig()

	if !cfg.HasSummaryConfig() {
		fmt.Fprintln(os.Stderr, "summary not configured: set summary.base_url and summary.api_key in config")
		os.Exit(1)
	}

	const testContent = "This is a connectivity test for the llmagent summary API. Reply with exactly: SUMMARY_OK"

	fmt.Printf("testing summary API at %s (model=%s)...\n", cfg.Summary.BaseURL, cfg.Summary.Model)
	result, err := summary.Summarize(cfg.Summary, "You are a test validator. If you receive a message, respond with exactly 'SUMMARY_OK' and nothing else.", testContent)
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("PASS (response: %s)\n", truncate(result, 100))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
