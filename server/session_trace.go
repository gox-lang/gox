package server

import "time"

type SessionTrace struct {
	Enabled bool
	fields  []SessionTraceField
	stages  []SessionTraceStage
}

type SessionTraceField struct {
	Name  string
	Value string
}

type SessionTraceStage struct {
	Name     string
	Duration time.Duration
}

func NewSessionTrace(enabled bool) *SessionTrace {
	return &SessionTrace{Enabled: enabled}
}

func (t *SessionTrace) Now() time.Time {
	if t == nil || !t.Enabled {
		return time.Time{}
	}
	return time.Now()
}

func (t *SessionTrace) Add(name string, started time.Time) {
	if t == nil || !t.Enabled || started.IsZero() || name == "" {
		return
	}
	t.stages = append(t.stages, SessionTraceStage{Name: name, Duration: time.Since(started)})
}

func (t *SessionTrace) AddDuration(name string, duration time.Duration) {
	if t == nil || !t.Enabled || name == "" {
		return
	}
	t.stages = append(t.stages, SessionTraceStage{Name: name, Duration: duration})
}

func (t *SessionTrace) AddField(name string, value string) {
	if t == nil || !t.Enabled || name == "" {
		return
	}
	t.fields = append(t.fields, SessionTraceField{Name: name, Value: value})
}

func (t *SessionTrace) AddBool(name string, value bool) {
	if value {
		t.AddField(name, "true")
		return
	}
	t.AddField(name, "false")
}

func (t *SessionTrace) Fields() []SessionTraceField {
	if t == nil || len(t.fields) == 0 {
		return nil
	}
	return t.fields
}

func (t *SessionTrace) Stages() []SessionTraceStage {
	if t == nil || len(t.stages) == 0 {
		return nil
	}
	return t.stages
}
