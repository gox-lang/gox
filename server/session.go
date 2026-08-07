package server

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrSessionNotAvailable = errors.New("session is not available")
	ErrSessionInvalidKey   = errors.New("session key is invalid")
	ErrSessionInvalidOut   = errors.New("session output target is invalid")
	ErrSessionExpired      = errors.New("session is expired")
)

type SessionConfig struct {
	TTL      time.Duration
	OnCreate func(session *Session)
	OnExpire func(session *ExpireSession)
}

type SessionStorage interface {
	GetName() string
	OpenSessionStorage(ctx context.Context, config SessionStorageConfig) (SessionStorageRuntime, error)
}

type SessionStorageConfig struct {
	ProjectName string
	TTL         time.Duration
}

type SessionStorageRuntime interface {
	GetName() string
	Close() error

	OpenSession(ctx context.Context, request SessionOpenRequest) (SessionOpenResult, error)
	GetSession(ctx context.Context, sessionID string) (SessionStorageInstance, error)
	CreateSession(ctx context.Context, meta *SessionMeta) (SessionStorageInstance, error)
	SaveSession(ctx context.Context, sessionID string) error
	TouchSession(ctx context.Context, sessionID string, ttl time.Duration) error
	DeleteSession(ctx context.Context, sessionID string) error

	SubscribeSessionEvents(ctx context.Context, fn SessionEventFunc) error
}

type SessionOpenStatus uint8

const (
	SessionOpenMissing SessionOpenStatus = iota
	SessionOpenOpened
	SessionOpenInvalidToken
	SessionOpenExpired
)

type SessionOpenRequest struct {
	ID    string
	Token string
	TTL   time.Duration
	Now   time.Time
	Trace *SessionTrace
}

type SessionOpenResult struct {
	Status   SessionOpenStatus
	Instance SessionStorageInstance
}

func (r SessionOpenResult) Opened() bool {
	return r.Status == SessionOpenOpened && r.Instance != nil
}

type SessionStorageInstance interface {
	ID() string
	Token() string

	Get(key string, out any) (bool, error)
	Has(key string) bool
	GetAll() map[string]any

	Set(key string, value any) error
	Delete(key string) error

	SetToken(token string) error

	Clear() error

	SetTTL(ttl time.Duration)
	GetTTL() time.Duration
}

type SessionMeta struct {
	ID        string
	Token     string
	TTL       time.Duration
	ExpireAt  time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type SessionExpireFunc func(session *ExpireSession)

type SessionEventKind uint8

const (
	SessionEventExpired SessionEventKind = iota + 1
	SessionEventDestroyed
)

type SessionEvent struct {
	Kind      SessionEventKind
	SessionID string
	Session   *ExpireSession
}

type SessionEventFunc func(event SessionEvent)

type SessionRotateFunc func() (string, error)
type SessionDestroyFunc func() error
type SessionSaveFunc func() error

type sessionLifecycle struct {
	mu        sync.Mutex
	destroy   SessionDestroyFunc
	destroyed bool
}

func (lifecycle *sessionLifecycle) destroySession() (bool, error) {
	if lifecycle == nil || lifecycle.destroy == nil {
		return false, ErrSessionNotAvailable
	}
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	if lifecycle.destroyed {
		return false, nil
	}
	if err := lifecycle.destroy(); err != nil {
		return false, err
	}
	lifecycle.destroyed = true
	return true, nil
}

func (lifecycle *sessionLifecycle) isDestroyed() bool {
	if lifecycle == nil {
		return false
	}
	lifecycle.mu.Lock()
	destroyed := lifecycle.destroyed
	lifecycle.mu.Unlock()
	return destroyed
}

type Session struct {
	id    string
	token string

	instance    SessionStorageInstance
	rotate      SessionRotateFunc
	save        SessionSaveFunc
	lifecycle   *sessionLifecycle
	onDestroyed func()
}

type ExpireSession struct {
	id        string
	token     string
	ttl       time.Duration
	expireAt  time.Time
	createdAt time.Time
	updatedAt time.Time
	instance  SessionStorageInstance
}

func NewSession(instance SessionStorageInstance) Session {
	return NewSessionWithRotate(instance, nil)
}

// NewSessionReference creates a read-only session identity for runtime-owned
// references that do not have access to the session storage instance.
func NewSessionReference(id string) Session {
	return Session{id: id}
}

func NewSessionWithRotate(instance SessionStorageInstance, rotate SessionRotateFunc) Session {
	return NewSessionWithLifecycle(instance, rotate, nil, nil)
}

func NewSessionWithLifecycle(instance SessionStorageInstance, rotate SessionRotateFunc, destroy SessionDestroyFunc, onDestroyed func(), save ...SessionSaveFunc) Session {
	if instance == nil {
		return Session{}
	}

	var saveSession SessionSaveFunc
	if len(save) > 0 {
		saveSession = save[0]
	}

	return Session{
		id:          instance.ID(),
		token:       instance.Token(),
		instance:    instance,
		rotate:      rotate,
		save:        saveSession,
		lifecycle:   &sessionLifecycle{destroy: destroy},
		onDestroyed: onDestroyed,
	}
}

// DetachSession removes request-only behavior while preserving the shared
// storage instance. It is intended for runtimes that retain a session beyond
// the lifetime of the HTTP request that opened it.
func DetachSession(session Session) Session {
	session.rotate = nil
	session.onDestroyed = nil
	if session.instance != nil {
		session.token = session.instance.Token()
	}
	return session
}

// ID returns the immutable identifier of this session.
func (s *Session) ID() string {
	if s == nil {
		return ""
	}

	return s.id
}

// Token returns the current token of this session.
func (s *Session) Token() string {
	if s == nil {
		return ""
	}

	return s.token
}

// Destroy permanently removes this session. Repeated calls are idempotent.
func (s *Session) Destroy() error {
	if s == nil || s.instance == nil || s.id == "" || s.lifecycle == nil {
		return ErrSessionNotAvailable
	}
	destroyed, err := s.lifecycle.destroySession()
	if err != nil {
		return err
	}
	if destroyed && s.onDestroyed != nil {
		s.onDestroyed()
	}
	return nil
}

// IsDestroyed reports whether this session was explicitly destroyed through
// any HTTP or WebSocket copy sharing the same lifecycle.
func (s *Session) IsDestroyed() bool {
	return s != nil && s.lifecycle != nil && s.lifecycle.isDestroyed()
}

func NewExpireSession(meta *SessionMeta, instance SessionStorageInstance) ExpireSession {
	if meta == nil || instance == nil {
		return ExpireSession{}
	}

	return ExpireSession{
		id:        meta.ID,
		token:     meta.Token,
		ttl:       meta.TTL,
		expireAt:  meta.ExpireAt,
		createdAt: meta.CreatedAt,
		updatedAt: meta.UpdatedAt,
		instance:  instance,
	}
}

func (s *ExpireSession) ID() string {
	if s == nil {
		return ""
	}

	return s.id
}

func (s *ExpireSession) Token() string {
	if s == nil {
		return ""
	}

	return s.token
}

func (s *ExpireSession) CreatedAt() time.Time {
	if s == nil {
		return time.Time{}
	}

	return s.createdAt
}

func (s *ExpireSession) LastUpdate() time.Time {
	if s == nil {
		return time.Time{}
	}

	return s.updatedAt
}

func (s *ExpireSession) ExpireAt() time.Time {
	if s == nil {
		return time.Time{}
	}

	return s.expireAt
}

func (s *ExpireSession) Get(key string, out any) (bool, error) {
	if s == nil || s.instance == nil || s.id == "" {
		return false, ErrSessionNotAvailable
	}

	return s.instance.Get(key, out)
}

func (s *ExpireSession) Has(key string) bool {
	if s == nil || s.instance == nil || s.id == "" {
		return false
	}

	return s.instance.Has(key)
}

func (s *ExpireSession) GetAll() map[string]any {
	if s == nil || s.instance == nil || s.id == "" {
		return nil
	}

	return s.instance.GetAll()
}

func (s *ExpireSession) GetTTL() time.Duration {
	if s == nil || s.id == "" {
		return 0
	}

	return s.ttl
}

func (s *Session) Get(key string, out any) (bool, error) {
	if !s.available() {
		return false, ErrSessionNotAvailable
	}

	return s.instance.Get(key, out)
}

func (s *Session) Set(key string, value any) error {
	if !s.available() {
		return ErrSessionNotAvailable
	}

	return s.instance.Set(key, value)
}

func (s *Session) Has(key string) bool {
	if !s.available() {
		return false
	}

	return s.instance.Has(key)
}

func (s *Session) GetAll() map[string]any {
	if !s.available() {
		return nil
	}

	return s.instance.GetAll()
}

func (s *Session) Delete(key string) error {
	if !s.available() {
		return ErrSessionNotAvailable
	}

	return s.instance.Delete(key)
}

func (s *Session) Rotate() error {
	if !s.available() || s.rotate == nil {
		return ErrSessionNotAvailable
	}

	token, err := s.rotate()
	if err != nil {
		return err
	}

	s.token = token
	return nil
}

func (s *Session) Clear() error {
	if !s.available() {
		return ErrSessionNotAvailable
	}

	return s.instance.Clear()
}

// Save persists every pending change made to this session.
func (s *Session) Save() error {
	if !s.available() || s.save == nil {
		return ErrSessionNotAvailable
	}

	return s.save()
}

func (s *Session) SetTTL(ttl time.Duration) {
	if !s.available() {
		return
	}

	s.instance.SetTTL(ttl)
}

func (s *Session) GetTTL() time.Duration {
	if !s.available() {
		return 0
	}

	return s.instance.GetTTL()
}

func (s *Session) available() bool {
	return s != nil && s.instance != nil && s.id != "" && !s.IsDestroyed()
}
