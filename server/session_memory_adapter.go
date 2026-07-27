package server

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"time"
)

const defaultTTL = 3 * time.Hour

type memorySessionOptions struct {
	StorageName string
	TTL         time.Duration
}

type memorySessionRuntime struct {
	storageName string
	optionsFn   func() memorySessionOptions

	mu          sync.RWMutex
	items       map[string]*record
	expireIndex *expireIndex

	eventMu sync.RWMutex
	eventFn SessionEventFunc
	eventID uint64

	wake chan struct{}
	stop chan struct{}
	once sync.Once
}

type state uint8

const (
	stateActive state = iota
	stateExpired
	stateDeleted
)

type record struct {
	mu sync.RWMutex

	id    string
	token string

	createdAt time.Time
	updatedAt time.Time
	expireAt  time.Time
	ttl       time.Duration

	state state
	data  map[string]any
}

func newMemorySessionRuntime(optionsFn func() memorySessionOptions) *memorySessionRuntime {
	options := memorySessionOptions{}
	if optionsFn != nil {
		options = optionsFn()
	}

	name := strings.TrimSpace(options.StorageName)
	if name == "" {
		name = "memory"
	}

	adapter := &memorySessionRuntime{
		storageName: name,
		optionsFn:   optionsFn,
		items:       make(map[string]*record),
		expireIndex: newExpireIndex(),
		wake:        make(chan struct{}, 1),
		stop:        make(chan struct{}),
	}

	go adapter.expirationLoop()

	return adapter
}

func (a *memorySessionRuntime) GetName() string {
	if a == nil || a.storageName == "" {
		return "memory"
	}

	return a.storageName
}

func (a *memorySessionRuntime) Close() error {
	if a == nil {
		return nil
	}

	a.once.Do(func() {
		close(a.stop)
	})

	a.eventMu.Lock()
	a.eventFn = nil
	a.eventID++
	a.eventMu.Unlock()

	a.mu.Lock()
	a.items = make(map[string]*record)
	a.expireIndex = newExpireIndex()
	a.mu.Unlock()

	return nil
}

func (a *memorySessionRuntime) OpenSession(ctx context.Context, request SessionOpenRequest) (SessionOpenResult, error) {
	_ = ctx

	if a == nil || request.ID == "" {
		return SessionOpenResult{Status: SessionOpenMissing}, nil
	}

	now := request.Now
	if now.IsZero() {
		now = time.Now()
	}

	ttl := normalizeTTL(request.TTL)
	if ttl <= 0 {
		ttl = a.options().TTL
	}

	rec, ok := a.record(request.ID)
	if !ok {
		return SessionOpenResult{Status: SessionOpenMissing}, nil
	}

	var status SessionOpenStatus
	var expireAt time.Time

	rec.mu.Lock()
	switch {
	case rec.state != stateActive:
		status = SessionOpenMissing

	case !now.Before(rec.expireAt):
		rec.state = stateExpired
		rec.updatedAt = now
		status = SessionOpenExpired

	case request.Token == "" || rec.token != request.Token:
		status = SessionOpenInvalidToken

	default:
		rec.ttl = ttl
		rec.expireAt = now.Add(ttl)
		rec.updatedAt = now
		expireAt = rec.expireAt
		status = SessionOpenOpened
	}
	rec.mu.Unlock()

	if status != SessionOpenOpened {
		return SessionOpenResult{Status: status}, nil
	}

	a.indexRecord(request.ID, expireAt)

	return SessionOpenResult{
		Status:   SessionOpenOpened,
		Instance: &memorySessionInstance{adapter: a, id: request.ID},
	}, nil
}

func (a *memorySessionRuntime) CreateSession(ctx context.Context, sessionMeta *SessionMeta) (SessionStorageInstance, error) {
	_ = ctx

	if sessionMeta == nil || sessionMeta.ID == "" {
		return nil, ErrSessionNotAvailable
	}

	now := time.Now()
	rec := &record{
		id:        sessionMeta.ID,
		token:     sessionMeta.Token,
		createdAt: pickTime(sessionMeta.CreatedAt, now),
		updatedAt: pickTime(sessionMeta.UpdatedAt, now),
		expireAt:  sessionMeta.ExpireAt,
		ttl:       normalizeTTL(sessionMeta.TTL),
		state:     stateActive,
		data:      make(map[string]any),
	}

	a.storeRecord(rec)

	return &memorySessionInstance{adapter: a, id: sessionMeta.ID}, nil
}

func (a *memorySessionRuntime) SaveSession(ctx context.Context, sessionID string) error {
	// Memory writes are applied directly by Instance methods.
	return nil
}

func (a *memorySessionRuntime) TouchSession(ctx context.Context, sessionID string, ttl time.Duration) error {
	_ = ctx
	if a == nil || sessionID == "" {
		return ErrSessionNotAvailable
	}

	rec, ok := a.record(sessionID)
	if !ok {
		return ErrSessionNotAvailable
	}

	now := time.Now()
	if ttl <= 0 {
		ttl = a.options().TTL
	}
	ttl = normalizeTTL(ttl)

	rec.mu.Lock()
	if rec.state != stateActive || !now.Before(rec.expireAt) {
		rec.mu.Unlock()
		return ErrSessionExpired
	}
	rec.ttl = ttl
	rec.expireAt = now.Add(ttl)
	rec.updatedAt = now
	expireAt := rec.expireAt
	rec.mu.Unlock()

	a.indexRecord(sessionID, expireAt)
	return nil
}

func (a *memorySessionRuntime) DeleteSession(ctx context.Context, sessionID string) error {
	_ = ctx
	if sessionID == "" {
		return ErrSessionNotAvailable
	}

	if _, ok := a.record(sessionID); ok {
		a.deleteSession(sessionID)
		a.dispatchSessionEvent(SessionEvent{Kind: SessionEventDestroyed, SessionID: sessionID})
	}
	return nil
}

func (a *memorySessionRuntime) SubscribeSessionEvents(ctx context.Context, fn SessionEventFunc) error {
	if a == nil || fn == nil {
		return nil
	}

	a.eventMu.Lock()
	a.eventID++
	subscriptionID := a.eventID
	a.eventFn = fn
	a.eventMu.Unlock()

	<-ctx.Done()

	a.eventMu.Lock()
	if a.eventID == subscriptionID {
		a.eventFn = nil
	}
	a.eventMu.Unlock()

	return nil
}

func (a *memorySessionRuntime) dispatchSessionEvent(event SessionEvent) {
	a.eventMu.RLock()
	fn := a.eventFn
	a.eventMu.RUnlock()
	if fn != nil {
		fn(event)
	}
}

func (a *memorySessionRuntime) options() memorySessionOptions {
	if a.optionsFn == nil {
		return memorySessionOptions{StorageName: a.GetName(), TTL: defaultTTL}
	}

	options := a.optionsFn()
	if options.StorageName == "" {
		options.StorageName = a.GetName()
	}
	if options.TTL <= 0 {
		options.TTL = defaultTTL
	}

	return options
}

func normalizeTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return defaultTTL
	}

	return ttl
}

func pickTime(value time.Time, fallback time.Time) time.Time {
	if value.IsZero() {
		return fallback
	}

	return value
}

func (a *memorySessionRuntime) storeRecord(rec *record) {
	a.mu.Lock()
	a.items[rec.id] = rec
	moved := a.expireIndex.Upsert(rec.id, rec.expireAt)
	a.mu.Unlock()

	if moved {
		a.wakeExpirationLoop()
	}
}

func (a *memorySessionRuntime) indexRecord(sessionID string, expireAt time.Time) {
	a.mu.Lock()
	moved := a.expireIndex.Upsert(sessionID, expireAt)
	a.mu.Unlock()

	if moved {
		a.wakeExpirationLoop()
	}
}

func (a *memorySessionRuntime) record(id string) (*record, bool) {
	a.mu.RLock()
	rec := a.items[id]
	a.mu.RUnlock()

	return rec, rec != nil
}

func (a *memorySessionRuntime) activeRecord(id string) (*record, bool) {
	rec, ok := a.record(id)
	if !ok {
		return nil, false
	}

	rec.mu.RLock()
	active := rec.state == stateActive && time.Now().Before(rec.expireAt)
	rec.mu.RUnlock()

	if !active {
		return nil, false
	}

	return rec, true
}

func (a *memorySessionRuntime) instanceRecord(id string) (*record, bool) {
	rec, ok := a.record(id)
	if !ok {
		return nil, false
	}

	rec.mu.RLock()
	available := rec.state == stateActive || rec.state == stateExpired
	rec.mu.RUnlock()

	if !available {
		return nil, false
	}

	return rec, true
}

func (a *memorySessionRuntime) deleteSession(sessionID string) {
	rec, _ := a.record(sessionID)
	if rec != nil {
		rec.mu.Lock()
		rec.state = stateDeleted
		rec.updatedAt = time.Now()
		rec.mu.Unlock()
	}

	a.mu.Lock()
	removed := a.expireIndex.Remove(sessionID)
	delete(a.items, sessionID)
	a.mu.Unlock()

	if removed {
		a.wakeExpirationLoop()
	}
}

func (a *memorySessionRuntime) expirationLoop() {
	timer := time.NewTimer(nextMinuteDelay(time.Now()))
	defer timer.Stop()

	for {
		select {
		case now := <-timer.C:
			a.expireDueSessions(now)
			timer.Reset(nextMinuteDelay(time.Now()))

		case <-a.wake:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(nextMinuteDelay(time.Now()))

		case <-a.stop:
			return
		}
	}
}

func (a *memorySessionRuntime) expireDueSessions(now time.Time) {
	for {
		a.mu.Lock()
		sessionID, ok := a.expireIndex.PopDue(now)
		a.mu.Unlock()

		if !ok {
			return
		}

		a.expireSession(sessionID, now)
	}
}

func (a *memorySessionRuntime) expireSession(sessionID string, now time.Time) {
	rec, ok := a.record(sessionID)
	if !ok {
		return
	}

	rec.mu.Lock()
	if rec.state != stateActive || now.Before(rec.expireAt) {
		rec.mu.Unlock()
		return
	}

	rec.state = stateExpired

	meta := &SessionMeta{
		ID:        rec.id,
		Token:     rec.token,
		TTL:       rec.ttl,
		ExpireAt:  rec.expireAt,
		CreatedAt: rec.createdAt,
		UpdatedAt: rec.updatedAt,
	}
	data := cloneMap(rec.data)
	rec.mu.Unlock()

	instance := &memoryExpiredSessionInstance{
		id:    meta.ID,
		token: meta.Token,
		ttl:   meta.TTL,
		data:  data,
	}

	expired := NewExpireSession(meta, instance)
	a.dispatchSessionEvent(SessionEvent{
		Kind:      SessionEventExpired,
		SessionID: sessionID,
		Session:   &expired,
	})

	a.deleteSession(sessionID)
}

func (a *memorySessionRuntime) wakeExpirationLoop() {
	select {
	case a.wake <- struct{}{}:
	default:
	}
}

func assign(out any, value any) error {
	target := reflect.ValueOf(out)
	if target.Kind() != reflect.Pointer || target.IsNil() {
		return ErrSessionInvalidOut
	}

	elem := target.Elem()
	if !elem.CanSet() {
		return ErrSessionInvalidOut
	}

	if value == nil {
		elem.Set(reflect.Zero(elem.Type()))
		return nil
	}

	source := reflect.ValueOf(value)
	if elem.Kind() == reflect.Slice && elem.Type().Elem().Kind() == reflect.Uint8 {
		if bytes, ok := value.([]byte); ok {
			copied := append(elem.Bytes()[:0], bytes...)
			elem.SetBytes(copied)
			return nil
		}
	}

	if source.Type().AssignableTo(elem.Type()) {
		elem.Set(source)
		return nil
	}

	if source.Type().ConvertibleTo(elem.Type()) {
		elem.Set(source.Convert(elem.Type()))
		return nil
	}

	return ErrSessionInvalidOut
}

func cloneMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return map[string]any{}
	}

	out := make(map[string]any, len(src))
	for key, value := range src {
		out[key] = cloneSessionValue(value)
	}

	return out
}

func cloneSessionValue(value any) any {
	switch v := value.(type) {
	case []byte:
		if v == nil {
			return []byte(nil)
		}

		out := make([]byte, len(v))
		copy(out, v)
		return out

	default:
		return value
	}
}
