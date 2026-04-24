package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/yearsyan/agentd/internal/config"
)

var manager = NewSessionManager()

type Request struct {
	Action string `json:"action"`
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	ID     string `json:"id"`
	Async  bool   `json:"async"`
}

type Response struct {
	Type     string        `json:"type"`
	Data     string        `json:"data,omitempty"`
	DataArr  []string      `json:"data_arr,omitempty"`
	Code     int           `json:"code,omitempty"`
	ID       string        `json:"id,omitempty"`
	Sessions []SessionInfo `json:"sessions,omitempty"`
}

func writeResponse(w io.Writer, resp Response) error {
	b, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = w.Write(b)
	return err
}

func Serve(socketPath string) error {
	go runCleanup()
	ln, err := listen(socketPath)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			return fmt.Errorf("accept: %w", err)
		}
		go handle(conn)
	}
}

func runCleanup() {
	for {
		time.Sleep(5 * time.Minute)
		manager.Cleanup(30*time.Minute, 120*time.Minute)
	}
}

func handle(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		return
	}

	var req Request
	if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
		writeResponse(conn, Response{Type: "stderr", Data: fmt.Sprintf("invalid request: %v", err)})
		writeResponse(conn, Response{Type: "exit", Code: 1})
		return
	}

	switch req.Action {
	case "attach":
		handleAttach(conn, req.ID)
	case "list":
		handleList(conn)
	default:
		handleCreate(conn, req.Model, req.Prompt, req.Async)
	}
}

func handleCreate(conn net.Conn, model, prompt string, async bool) {
	if model == "" || prompt == "" {
		writeResponse(conn, Response{Type: "stderr", Data: "model and prompt are required"})
		writeResponse(conn, Response{Type: "exit", Code: 1})
		return
	}

	cfg, err := config.Load()
	if err != nil {
		writeResponse(conn, Response{Type: "stderr", Data: err.Error()})
		writeResponse(conn, Response{Type: "exit", Code: 1})
		return
	}

	mc, err := cfg.GetModelConfig(model)
	if err != nil {
		writeResponse(conn, Response{Type: "stderr", Data: err.Error()})
		writeResponse(conn, Response{Type: "exit", Code: 1})
		return
	}

	cmd := buildCommand(prompt, mc)
	if cmd == nil {
		writeResponse(conn, Response{Type: "stderr", Data: fmt.Sprintf("no command for backend: %s", mc.Backend)})
		writeResponse(conn, Response{Type: "exit", Code: 1})
		return
	}

	session := manager.Create(model, prompt)
	writeResponse(conn, Response{Type: "created", ID: session.ID})

	// Apply session timeout via context
	ctx, cancel := context.WithTimeout(context.Background(), session.Timeout)
	session.SetCancel(cancel)
	go func() {
		<-ctx.Done()
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}()

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		writeResponse(conn, Response{Type: "stderr", Data: err.Error()})
		writeResponse(conn, Response{Type: "exit", Code: 1})
		manager.Remove(session.ID)
		return
	}

	// Fan-in lines from stdout/stderr into the session broadcaster
	go func() {
		var wg sync.WaitGroup
		wg.Add(2)
		go broadcastLines(stdout, "stdout", session, &wg)
		go broadcastLines(stderr, "stderr", session, &wg)
		wg.Wait()

		err := cmd.Wait()
		code := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				code = exitErr.ExitCode()
			} else {
				code = 1
			}
		}
		session.Broadcast(Response{Type: "exit", Code: code})
		session.MarkDone(code)
		session.Close()
	}()

	// Async: return session ID immediately, don't stream
	if async {
		writeResponse(conn, Response{Type: "exit", Code: 0})
		return
	}

	// Subscribe the requesting client and relay output
	subCh, _ := session.Subscribe()
	for {
		select {
		case resp, ok := <-subCh:
			if !ok {
				writeResponse(conn, Response{Type: "exit", Code: session.ExitCode})
				return
			}
			writeResponse(conn, resp)
		case <-session.Done():
			writeResponse(conn, Response{Type: "exit", Code: session.ExitCode})
			return
		}
	}
}

func handleAttach(conn net.Conn, id string) {
	session := manager.Get(id)
	if session == nil {
		writeResponse(conn, Response{Type: "stderr", Data: fmt.Sprintf("session %q not found", id)})
		writeResponse(conn, Response{Type: "exit", Code: 1})
		return
	}

	// Send history
	session.mu.Lock()
	if len(session.history) > 0 {
		lines := make([]string, len(session.history))
		for i, r := range session.history {
			lines[i] = r.Data
		}
		writeResponse(conn, Response{Type: "history", DataArr: lines})
	}
	session.mu.Unlock()

	// If session already done, send exit and return
	select {
	case <-session.Done():
		writeResponse(conn, Response{Type: "exit", Code: session.ExitCode})
		return
	default:
	}

	// Subscribe for live updates
	subCh, _ := session.Subscribe()
	defer session.Unsubscribe(subCh)

	for {
		select {
		case resp, ok := <-subCh:
			if !ok {
				writeResponse(conn, Response{Type: "exit", Code: session.ExitCode})
				return
			}
			writeResponse(conn, resp)
		case <-session.Done():
			writeResponse(conn, Response{Type: "exit", Code: session.ExitCode})
			return
		}
	}
}

func handleList(conn net.Conn) {
	sessions := manager.List()
	writeResponse(conn, Response{Type: "sessions", Sessions: sessions})
	writeResponse(conn, Response{Type: "exit", Code: 0})
}

func broadcastLines(r io.Reader, typ string, session *Session, wg *sync.WaitGroup) {
	defer wg.Done()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		session.Broadcast(Response{Type: typ, Data: scanner.Text()})
	}
}

func buildCommand(prompt string, mc config.ModelConfig) *exec.Cmd {
	switch mc.Backend {
	case config.BackendClaudeCode:
		cmd := exec.Command("claude", "-p", prompt, "--dangerously-skip-permissions")
		cmd.Env = os.Environ()
		if !mc.Official {
			cmd.Env = append(cmd.Env,
				"ANTHROPIC_BASE_URL="+mc.BaseURL,
				"ANTHROPIC_AUTH_TOKEN="+mc.AuthToken,
				"ANTHROPIC_MODEL="+mc.Model,
			)
			if mc.SmallFastModel != "" {
				cmd.Env = append(cmd.Env, "ANTHROPIC_SMALL_FAST_MODEL="+mc.SmallFastModel)
			}
		}
		return cmd
	case config.BackendOpenCode:
		args := []string{"run", prompt}
		if mc.Model != "" {
			args = append(args, "--model", mc.Model)
		}
		return exec.Command("opencode", args...)
	case config.BackendCodex:
		args := []string{"exec", prompt, "--skip-git-repo-check"}
		if mc.Model != "" {
			args = append(args, "--model", mc.Model)
		}
		return exec.Command("codex", args...)
	default:
		return nil
	}
}

func PIDPath(socketPath string) string {
	return socketPath + ".pid"
}

func WritePID(socketPath string, pid int) error {
	return os.WriteFile(PIDPath(socketPath), []byte(strconv.Itoa(pid)), 0644)
}

func ReadPID(socketPath string) (int, error) {
	data, err := os.ReadFile(PIDPath(socketPath))
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(string(data))
}

func RemovePID(socketPath string) {
	os.Remove(PIDPath(socketPath))
}

func IsRunning(socketPath string) bool {
	conn, err := dial(socketPath)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
