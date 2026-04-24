package daemon

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"
)

type SessionStatus string

const (
	StatusRunning SessionStatus = "running"
	StatusDone    SessionStatus = "done"
	StatusError   SessionStatus = "error"
)

type SessionInfo struct {
	ID        string        `json:"id"`
	Model     string        `json:"model"`
	Prompt    string        `json:"prompt"`
	Status    SessionStatus `json:"status"`
	StartTime time.Time     `json:"start_time"`
}

type Session struct {
	ID        string
	Model     string
	Prompt    string
	Status    SessionStatus
	StartTime time.Time
	EndTime   time.Time
	ExitCode  int
	Timeout   time.Duration

	mu          sync.Mutex
	subscribers map[chan Response]struct{}
	history     []Response
	done        chan struct{}
	cancel      context.CancelFunc
}

const (
	maxHistoryLines = 10000
)

type SessionManager struct {
	mu       sync.Mutex
	sessions map[string]*Session
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*Session),
	}
}

func genID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return "sess-" + hex.EncodeToString(b)
}

func (sm *SessionManager) Create(model, prompt string) *Session {
	s := &Session{
		ID:          genID(),
		Model:       model,
		Prompt:      prompt,
		Status:      StatusRunning,
		StartTime:   time.Now(),
		Timeout:     120 * time.Minute,
		subscribers: make(map[chan Response]struct{}),
		done:        make(chan struct{}),
	}
	sm.mu.Lock()
	sm.sessions[s.ID] = s
	sm.mu.Unlock()
	return s
}

func (sm *SessionManager) Cleanup(maxDoneAge, maxRunAge time.Duration) {
	now := time.Now()
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for id, s := range sm.sessions {
		switch s.Status {
		case StatusRunning:
			if now.Sub(s.StartTime) > maxRunAge {
				log.Printf("[session %s] timeout after %v, killing", id, maxRunAge)
				s.kill()
				sm.removeLocked(id)
			}
		case StatusDone, StatusError:
			if s.EndTime.IsZero() {
				continue
			}
			if now.Sub(s.EndTime) > maxDoneAge {
				log.Printf("[session %s] expired, removing", id)
				sm.removeLocked(id)
			}
		}
	}
}

func (sm *SessionManager) removeLocked(id string) {
	if s, ok := sm.sessions[id]; ok {
		s.kill()
		s.Close()
	}
	delete(sm.sessions, id)
}

func (sm *SessionManager) Remove(id string) {
	sm.mu.Lock()
	if s, ok := sm.sessions[id]; ok {
		s.kill()
		s.Close()
	}
	delete(sm.sessions, id)
	sm.mu.Unlock()
}

func (s *Session) SetCancel(cancel context.CancelFunc) {
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()
}

func (s *Session) kill() {
	if s.cancel != nil {
		s.cancel()
	}
}

func (sm *SessionManager) Get(id string) *Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.sessions[id]
}

func (sm *SessionManager) List() []SessionInfo {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	list := make([]SessionInfo, 0, len(sm.sessions))
	for _, s := range sm.sessions {
		list = append(list, SessionInfo{
			ID:        s.ID,
			Model:     s.Model,
			Prompt:    s.Prompt,
			Status:    s.Status,
			StartTime: s.StartTime,
		})
	}
	return list
}

func (sm *SessionManager) All() []*Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sessions := make([]*Session, 0, len(sm.sessions))
	for _, s := range sm.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}

func (s *Session) Broadcast(resp Response) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.history) >= maxHistoryLines {
		// Drop oldest 10% to prevent unbounded growth
		drop := maxHistoryLines / 10
		s.history = s.history[drop:]
	}
	s.history = append(s.history, resp)

	for ch := range s.subscribers {
		select {
		case ch <- resp:
		default:
		}
	}
}

func (s *Session) Subscribe() (chan Response, []Response) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := make(chan Response, 256)
	s.subscribers[ch] = struct{}{}
	history := make([]Response, len(s.history))
	copy(history, s.history)
	return ch, history
}

func (s *Session) Unsubscribe(ch chan Response) {
	s.mu.Lock()
	delete(s.subscribers, ch)
	s.mu.Unlock()
}

func (s *Session) MarkDone(code int) {
	s.mu.Lock()
	s.ExitCode = code
	s.EndTime = time.Now()
	if code == 0 {
		s.Status = StatusDone
	} else {
		s.Status = StatusError
	}
	s.mu.Unlock()
	close(s.done)
}

func (s *Session) Done() <-chan struct{} {
	return s.done
}

func (s *Session) Close() {
	s.mu.Lock()
	for ch := range s.subscribers {
		close(ch)
	}
	s.subscribers = make(map[chan Response]struct{})
	s.mu.Unlock()
}

func (s *Session) String() string {
	return fmt.Sprintf("%s | %s | %s | %s", s.ID, s.Model, s.Status, s.Prompt)
}
