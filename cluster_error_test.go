package gox

import "testing"

func TestClusterErrorHandlerReceivesCanonicalError(t *testing.T) {
	received := make(chan ClusterError, 1)
	Cluster.When("error", func(err ClusterError) { received <- err })
	DispatchClusterError(ClusterError{Adapter: "redis", Operation: "decode", Message: "malformed"})
	err := <-received
	if err.Adapter != "redis" || err.Operation != "decode" || err.Error() != "malformed" {
		t.Fatalf("cluster error = %#v", err)
	}
}
