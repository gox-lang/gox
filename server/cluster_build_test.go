package server

import (
	"context"
	"testing"
	"time"
)

type clusterBuildCoordinatorContract struct{}

func (clusterBuildCoordinatorContract) AcquireBuild(context.Context, ClusterBuildRequest) (ClusterBuildLease, error) {
	return ClusterBuildLease{}, nil
}

func (clusterBuildCoordinatorContract) RenewBuild(context.Context, ClusterBuildLease, time.Duration) (ClusterBuildLease, error) {
	return ClusterBuildLease{}, nil
}

func (clusterBuildCoordinatorContract) CompleteBuild(context.Context, ClusterBuildLease, ClusterBuildArtifact) error {
	return nil
}

func (clusterBuildCoordinatorContract) FailBuild(context.Context, ClusterBuildLease, ClusterBuildFailure) error {
	return nil
}

func (clusterBuildCoordinatorContract) WaitBuild(context.Context, ClusterBuildWaitRequest) (ClusterBuildResult, error) {
	return ClusterBuildResult{}, nil
}

func (clusterBuildCoordinatorContract) InvalidateBuild(context.Context, ClusterBuildArtifact) error {
	return nil
}

func TestClusterBuildCoordinatorContractIsOptionalCapability(t *testing.T) {
	var capability any = clusterBuildCoordinatorContract{}
	if _, ok := capability.(ClusterBuildCoordinator); !ok {
		t.Fatal("build coordinator does not satisfy its public contract")
	}
}

func TestClusterBuildResultStatesAreWireStable(t *testing.T) {
	if ClusterBuildResultCompleted != 1 {
		t.Fatalf("completed state = %d", ClusterBuildResultCompleted)
	}
	if ClusterBuildResultFailed != 2 {
		t.Fatalf("failed state = %d", ClusterBuildResultFailed)
	}
}
