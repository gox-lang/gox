package gox

import (
	"net"
	"strconv"
	"sync"
	"time"
)

type ServerInfoBindings struct {
	ProjectName   func() string
	InstanceID    func() string
	StartedAt     func() time.Time
	Host          func() string
	Port          func() int
	IsDevelopment func() bool
	IsClustered   func() bool
}

type serverInfoState struct {
	mu         sync.RWMutex
	generation uint64
	bindings   ServerInfoBindings
}

type ServerContext struct{ state *serverInfoState }

var Server = ServerContext{state: &serverInfoState{}}

func BindServerInfo(bindings ServerInfoBindings) func() {
	state := Server.state
	state.mu.Lock()
	state.generation++
	generation := state.generation
	state.bindings = bindings
	state.mu.Unlock()
	return generationCleanup(&state.mu, func() uint64 { return state.generation }, generation, func() {
		state.bindings = ServerInfoBindings{}
	})
}

func (server *ServerContext) bindings() ServerInfoBindings {
	if server == nil || server.state == nil {
		return ServerInfoBindings{}
	}
	server.state.mu.RLock()
	bindings := server.state.bindings
	server.state.mu.RUnlock()
	return bindings
}

func (server *ServerContext) ProjectName() string {
	if fn := server.bindings().ProjectName; fn != nil {
		return fn()
	}
	return ""
}
func (server *ServerContext) InstanceID() string {
	if fn := server.bindings().InstanceID; fn != nil {
		return fn()
	}
	return ""
}
func (server *ServerContext) StartedAt() time.Time {
	if fn := server.bindings().StartedAt; fn != nil {
		return fn()
	}
	return time.Time{}
}
func (server *ServerContext) Uptime() time.Duration {
	started := server.StartedAt()
	if started.IsZero() {
		return 0
	}
	return time.Since(started)
}
func (server *ServerContext) Host() string {
	if fn := server.bindings().Host; fn != nil {
		return fn()
	}
	return ""
}
func (server *ServerContext) Port() int {
	if fn := server.bindings().Port; fn != nil {
		return fn()
	}
	return 0
}
func (server *ServerContext) Address() string {
	host := server.Host()
	port := server.Port()
	if host == "" || port <= 0 {
		return ""
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}
func (server *ServerContext) IsDevelopment() bool {
	if fn := server.bindings().IsDevelopment; fn != nil {
		return fn()
	}
	return false
}
func (server *ServerContext) IsClustered() bool {
	if fn := server.bindings().IsClustered; fn != nil {
		return fn()
	}
	return false
}

type ClusterInfoBindings struct {
	Enabled func() bool
	Adapter func() string
}

type clusterInfoState struct {
	mu         sync.RWMutex
	generation uint64
	bindings   ClusterInfoBindings
}

func newClusterInfoState() *clusterInfoState { return &clusterInfoState{} }

func BindClusterInfo(bindings ClusterInfoBindings) func() {
	state := Cluster.state
	state.mu.Lock()
	state.generation++
	generation := state.generation
	state.bindings = bindings
	state.mu.Unlock()
	return generationCleanup(&state.mu, func() uint64 { return state.generation }, generation, func() {
		state.bindings = ClusterInfoBindings{}
	})
}

func (cluster *ClusterContext) bindings() ClusterInfoBindings {
	if cluster == nil || cluster.state == nil {
		return ClusterInfoBindings{}
	}
	cluster.state.mu.RLock()
	bindings := cluster.state.bindings
	cluster.state.mu.RUnlock()
	return bindings
}

func (cluster *ClusterContext) Enabled() bool {
	if fn := cluster.bindings().Enabled; fn != nil {
		return fn()
	}
	return false
}
func (cluster *ClusterContext) Adapter() string {
	if fn := cluster.bindings().Adapter; fn != nil {
		return fn()
	}
	return ""
}

func generationCleanup(mu *sync.RWMutex, current func() uint64, generation uint64, reset func()) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			mu.Lock()
			if current() == generation {
				reset()
			}
			mu.Unlock()
		})
	}
}
