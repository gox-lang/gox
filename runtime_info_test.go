package gox

import (
	"sync"
	"testing"
	"time"
)

func TestServerAndClusterInformationAreReadOnlyZeroSafeViews(t *testing.T) {
	started := time.Now().Add(-time.Second)
	cleanupServer := BindServerInfo(ServerInfoBindings{
		ProjectName:   func() string { return "example" },
		InstanceID:    func() string { return "instance-1" },
		StartedAt:     func() time.Time { return started },
		Host:          func() string { return "127.0.0.1" },
		Port:          func() int { return 3001 },
		IsDevelopment: func() bool { return true },
		IsClustered:   func() bool { return true },
	})
	defer cleanupServer()
	cleanupCluster := BindClusterInfo(ClusterInfoBindings{
		Enabled: func() bool { return true },
		Adapter: func() string { return "redis" },
	})
	defer cleanupCluster()

	if Server.ProjectName() != "example" || Server.InstanceID() != "instance-1" {
		t.Fatalf("server identity = %q/%q", Server.ProjectName(), Server.InstanceID())
	}
	if Server.StartedAt() != started || Server.Uptime() < time.Second {
		t.Fatalf("server time = %v uptime=%v", Server.StartedAt(), Server.Uptime())
	}
	if Server.Host() != "127.0.0.1" || Server.Port() != 3001 || Server.Address() != "127.0.0.1:3001" {
		t.Fatalf("server address = %q", Server.Address())
	}
	if !Server.IsDevelopment() || !Server.IsClustered() || !Cluster.Enabled() || Cluster.Adapter() != "redis" {
		t.Fatal("runtime flags were not exposed")
	}
}

func TestRuntimeInformationCleanupCannotUnbindNewerRuntime(t *testing.T) {
	oldServer := BindServerInfo(ServerInfoBindings{InstanceID: func() string { return "old" }})
	newServer := BindServerInfo(ServerInfoBindings{InstanceID: func() string { return "new" }})
	oldCluster := BindClusterInfo(ClusterInfoBindings{Adapter: func() string { return "old" }})
	newCluster := BindClusterInfo(ClusterInfoBindings{Adapter: func() string { return "new" }})
	oldServer()
	oldCluster()
	if Server.InstanceID() != "new" || Cluster.Adapter() != "new" {
		t.Fatal("older cleanup unbound newer runtime")
	}
	newServer()
	newCluster()
	if Server.InstanceID() != "" || Cluster.Adapter() != "" {
		t.Fatal("active cleanup did not reset runtime information")
	}
}

func TestRuntimeInformationViewsAreRaceSafe(t *testing.T) {
	cleanupServer := BindServerInfo(ServerInfoBindings{InstanceID: func() string { return "instance" }})
	cleanupCluster := BindClusterInfo(ClusterInfoBindings{Enabled: func() bool { return true }})
	defer cleanupServer()
	defer cleanupCluster()
	var group sync.WaitGroup
	for index := 0; index < 64; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			_ = Server.InstanceID()
			_ = Server.Address()
			_ = Server.Uptime()
			_ = Cluster.Enabled()
			_ = Cluster.Adapter()
		}()
	}
	group.Wait()
}
