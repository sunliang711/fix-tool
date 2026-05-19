package shell

import "sync"

// SessionState 让 shell 中的多个 service 共享同一个 FIX 会话登录状态。
type SessionState struct {
	mu       sync.RWMutex
	loggedOn bool
}

func NewSessionState() *SessionState {
	return &SessionState{}
}

func (s *SessionState) LoggedOn() bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.loggedOn
}

func (s *SessionState) SetLoggedOn(value bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loggedOn = value
}
