package gox

import (
	"fmt"
	"sync"
)

// ClusterError describes a failure produced while publishing, receiving,
// decoding or routing communication between GOX instances.
type ClusterError struct {
	Adapter    string
	Operation  string
	Message    string
	InstanceID string
	Cause      error
}

func (err ClusterError) Error() string {
	if err.Message != "" {
		return err.Message
	}
	return "GOX cluster communication failed"
}

func (err ClusterError) Unwrap() error { return err.Cause }

type serverEventState struct {
	mu           sync.RWMutex
	clusterError func(ClusterError)
}

func newServerEventState() *serverEventState { return &serverEventState{} }

// ClusterContext exposes read-only information and lifecycle events for the
// active distributed adapter.
type ClusterContext struct {
	events *serverEventState
	state  *clusterInfoState
}

// Cluster is the process-wide distributed runtime facade.
var Cluster = ClusterContext{events: newServerEventState(), state: newClusterInfoState()}

// When registers a distributed runtime handler. Currently error is the only
// supported cluster event.
func (cluster *ClusterContext) When(event string, handler any) {
	if cluster == nil || cluster.events == nil {
		return
	}
	if event != "error" {
		panic(fmt.Sprintf("gox.Cluster.When does not support event %q", event))
	}
	typed, ok := handler.(func(ClusterError))
	if !ok || typed == nil {
		panic(`gox.Cluster.When("error") requires func(gox.ClusterError)`)
	}
	cluster.events.mu.Lock()
	cluster.events.clusterError = typed
	cluster.events.mu.Unlock()
}

// DispatchClusterError is used by the host runtime to report a cluster
// failure without coupling an adapter to the public GOX package.
func DispatchClusterError(err ClusterError) {
	if Cluster.events == nil {
		return
	}
	Cluster.events.mu.RLock()
	handler := Cluster.events.clusterError
	Cluster.events.mu.RUnlock()
	if handler != nil {
		handler(err)
	}
}
