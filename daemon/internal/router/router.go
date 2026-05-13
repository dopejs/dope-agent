package router

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/profiles"
)

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrChannelRequired = errors.New("channel is required")
	ErrPeerRequired    = errors.New("peer is required")
	ErrThreadRequired  = errors.New("thread is required for group sessions")
)

type SessionKind string

const (
	SessionKindDirect SessionKind = "direct"
	SessionKindGroup  SessionKind = "group"
)

type SessionStatus string

const (
	SessionStatusActive SessionStatus = "active"
)

type Session struct {
	SessionID               string                      `json:"sessionId"`
	Kind                    SessionKind                 `json:"kind"`
	Status                  SessionStatus               `json:"status"`
	Channel                 string                      `json:"channel"`
	AccountID               string                      `json:"accountId,omitempty"`
	PeerID                  string                      `json:"peerId"`
	ThreadID                string                      `json:"threadId,omitempty"`
	RoutingKey              string                      `json:"routingKey"`
	Generation              int                         `json:"generation"`
	CreatedAt               time.Time                   `json:"createdAt"`
	UpdatedAt               time.Time                   `json:"updatedAt"`
	LastActiveAt            time.Time                   `json:"lastActiveAt"`
	LastResetAt             *time.Time                  `json:"lastResetAt,omitempty"`
	ActiveProfileProjection *profiles.RuntimeProjection `json:"activeProfileProjection,omitempty"`
}

type RouteInput struct {
	Kind      SessionKind `json:"kind"`
	Channel   string      `json:"channel"`
	AccountID string      `json:"accountId"`
	PeerID    string      `json:"peerId"`
	ThreadID  string      `json:"threadId"`
}

type SessionRouter struct {
	mu           sync.RWMutex
	sessionsByID map[string]Session
	sessionIDs   []string
	byRoutingKey map[string]string
}

func NewSessionRouter() *SessionRouter {
	return &SessionRouter{
		sessionsByID: make(map[string]Session),
		byRoutingKey: make(map[string]string),
	}
}

func (r *SessionRouter) Route(input RouteInput) (Session, bool, error) {
	routingKey, err := makeRoutingKey(input)
	if err != nil {
		return Session{}, false, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if sessionID, ok := r.byRoutingKey[routingKey]; ok {
		session := r.sessionsByID[sessionID]
		now := time.Now().UTC()
		session.UpdatedAt = now
		session.LastActiveAt = now
		r.sessionsByID[sessionID] = session
		return session, false, nil
	}

	now := time.Now().UTC()
	session := Session{
		SessionID:    newSessionID(),
		Kind:         normalizeKind(input.Kind),
		Status:       SessionStatusActive,
		Channel:      input.Channel,
		AccountID:    input.AccountID,
		PeerID:       input.PeerID,
		ThreadID:     input.ThreadID,
		RoutingKey:   routingKey,
		Generation:   1,
		CreatedAt:    now,
		UpdatedAt:    now,
		LastActiveAt: now,
	}

	r.sessionsByID[session.SessionID] = session
	r.sessionIDs = append(r.sessionIDs, session.SessionID)
	r.byRoutingKey[routingKey] = session.SessionID

	return session, true, nil
}

func (r *SessionRouter) ListSessions() []Session {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sessions := make([]Session, 0, len(r.sessionIDs))
	for _, sessionID := range r.sessionIDs {
		sessions = append(sessions, r.sessionsByID[sessionID])
	}

	return sessions
}

func (r *SessionRouter) GetSession(sessionID string) (Session, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	session, ok := r.sessionsByID[sessionID]
	return session, ok
}

func (r *SessionRouter) TouchSession(sessionID string) (Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, ok := r.sessionsByID[sessionID]
	if !ok {
		return Session{}, ErrSessionNotFound
	}

	now := time.Now().UTC()
	session.UpdatedAt = now
	session.LastActiveAt = now
	r.sessionsByID[sessionID] = session

	return session, nil
}

func (r *SessionRouter) ResetSession(sessionID string) (Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, ok := r.sessionsByID[sessionID]
	if !ok {
		return Session{}, ErrSessionNotFound
	}

	now := time.Now().UTC()
	session.Generation++
	session.UpdatedAt = now
	session.LastActiveAt = now
	session.LastResetAt = &now
	r.sessionsByID[sessionID] = session

	return session, nil
}

func (r *SessionRouter) RestoreSessions(sessions []Session) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.sessionsByID = make(map[string]Session, len(sessions))
	r.sessionIDs = make([]string, 0, len(sessions))
	r.byRoutingKey = make(map[string]string, len(sessions))

	for _, session := range sessions {
		r.sessionsByID[session.SessionID] = session
		r.sessionIDs = append(r.sessionIDs, session.SessionID)
		r.byRoutingKey[session.RoutingKey] = session.SessionID
	}
}

func makeRoutingKey(input RouteInput) (string, error) {
	kind := normalizeKind(input.Kind)
	if input.Channel == "" {
		return "", ErrChannelRequired
	}
	if input.PeerID == "" {
		return "", ErrPeerRequired
	}

	switch kind {
	case SessionKindGroup:
		if input.ThreadID == "" {
			return "", ErrThreadRequired
		}
		return fmt.Sprintf("%s:%s:%s:%s:%s", kind, input.Channel, input.AccountID, input.PeerID, input.ThreadID), nil
	default:
		return fmt.Sprintf("%s:%s:%s:%s", SessionKindDirect, input.Channel, input.AccountID, input.PeerID), nil
	}
}

func normalizeKind(kind SessionKind) SessionKind {
	if kind == SessionKindGroup {
		return SessionKindGroup
	}
	return SessionKindDirect
}

func newSessionID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "sess_fallback"
	}

	return "sess_" + hex.EncodeToString(buf)
}
