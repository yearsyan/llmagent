package daemon

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

func sendAndRead(sock string, req Request, handler func(Response) bool) error {
	conn, err := dial(sock)
	if err != nil {
		return fmt.Errorf("connect to daemon: %w (is the daemon running?)", err)
	}
	defer conn.Close()

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	reqBytes = append(reqBytes, '\n')

	if _, err := conn.Write(reqBytes); err != nil {
		return fmt.Errorf("send request: %w", err)
	}

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var resp Response
		if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
			fmt.Fprintf(os.Stderr, "invalid response: %v\n", err)
			continue
		}
		if !handler(resp) {
			return nil
		}
	}
	return nil
}

func CreateSession(sock, model, prompt string) error {
	return sendAndRead(sock, Request{Action: "create", Model: model, Prompt: prompt}, func(resp Response) bool {
		switch resp.Type {
		case "created":
			fmt.Printf("[session %s]\n", resp.ID)
		case "stdout":
			fmt.Println(resp.Data)
		case "stderr":
			fmt.Fprintln(os.Stderr, resp.Data)
		case "exit":
			if resp.Code != 0 {
				os.Exit(resp.Code)
			}
			return false
		}
		return true
	})
}

func CreateSessionAsync(sock, model, prompt string) (string, error) {
	var sessionID string
	err := sendAndRead(sock, Request{Action: "create", Model: model, Prompt: prompt, Async: true}, func(resp Response) bool {
		switch resp.Type {
		case "created":
			sessionID = resp.ID
		case "stderr":
			fmt.Fprintln(os.Stderr, resp.Data)
		case "exit":
			return false
		}
		return true
	})
	return sessionID, err
}

func AttachSession(sock, id string) error {
	return sendAndRead(sock, Request{Action: "attach", ID: id}, func(resp Response) bool {
		switch resp.Type {
		case "history":
			for _, line := range resp.DataArr {
				fmt.Println(line)
			}
		case "stdout":
			fmt.Println(resp.Data)
		case "stderr":
			fmt.Fprintln(os.Stderr, resp.Data)
		case "exit":
			if resp.Code != 0 {
				os.Exit(resp.Code)
			}
			return false
		}
		return true
	})
}

func TestSession(sock, model, prompt string) (int, error) {
	exitCode := -1
	err := sendAndRead(sock, Request{Action: "create", Model: model, Prompt: prompt}, func(resp Response) bool {
		if resp.Type == "stderr" {
			fmt.Fprintln(os.Stderr, resp.Data)
		}
		if resp.Type == "exit" {
			exitCode = resp.Code
			return false
		}
		return true
	})
	return exitCode, err
}

func ListSessions(sock string) ([]SessionInfo, error) {
	var sessions []SessionInfo
	err := sendAndRead(sock, Request{Action: "list"}, func(resp Response) bool {
		if resp.Type == "sessions" {
			sessions = resp.Sessions
		}
		return resp.Type != "exit"
	})
	return sessions, err
}

type SessionOutput struct {
	SessionID string
	Stdout    []string
	Stderr    []string
	ExitCode  int
}

// CaptureSession runs a session and collects all output without printing.
func CaptureSession(sock, model, prompt string) (*SessionOutput, error) {
	var out SessionOutput
	err := sendAndRead(sock, Request{Action: "create", Model: model, Prompt: prompt}, func(resp Response) bool {
		switch resp.Type {
		case "created":
			out.SessionID = resp.ID
		case "stdout":
			out.Stdout = append(out.Stdout, resp.Data)
		case "stderr":
			out.Stderr = append(out.Stderr, resp.Data)
		case "exit":
			out.ExitCode = resp.Code
			return false
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}
