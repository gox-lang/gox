package server

import (
	"context"
	"time"
)

// ClusterAdapter opens the shared distributed infrastructure used by GOX.
// Implementations provide both session storage and binary inter-node messaging.
type ClusterAdapter interface {
	GetName() string
	OpenCluster(ctx context.Context, config ClusterConfig) (ClusterRuntime, error)
}

type ClusterConfig struct {
	ProjectName    string
	InstanceID     string
	SessionTTL     time.Duration
	MaxMessageSize int64
}

type ClusterEventKind uint8

const (
	ClusterEventSessionInvalidated ClusterEventKind = iota + 1
	ClusterEventSessionDestroyed
	ClusterEventSocketPacket
	ClusterEventSocketRequest
	ClusterEventSocketResponse
	ClusterEventSocketRequestCancel
	ClusterEventSocketDisconnect
	ClusterEventChannelClosed
)

type ClusterTargetKind uint8

const (
	ClusterTargetAll ClusterTargetKind = iota + 1
	ClusterTargetSession
	ClusterTargetSocket
	ClusterTargetGroup
	ClusterTargetChannel
)

type ClusterTarget struct {
	Kind            ClusterTargetKind
	SessionID       string
	SocketID        string
	GroupName       string
	ChannelID       string
	ExcludeSocketID string
}

type ClusterEvent struct {
	Kind             ClusterEventKind
	InstanceID       string
	SessionID        string
	Target           ClusterTarget
	CorrelationID    []byte
	TargetInstanceID string
	CloseCode        uint16
	Payload          []byte
}

type ClusterEventFunc func(event ClusterEvent)
type ClusterErrorFunc func(err error)

// ClusterRuntime is opened once and shared by the session and WebSocket
// runtimes. Each adapter owns its wire protocol and exposes only validated,
// transport-independent events to GOX.
type ClusterRuntime interface {
	SessionStorageRuntime
	ClusterSocketDirectory
	ClusterGroupDirectory
	ClusterChannelDirectory
	InstanceID() string
	PublishSessionInvalidated(ctx context.Context, sessionID string) error
	PublishSessionDestroyed(ctx context.Context, sessionID string) error
	PublishSocketPacket(ctx context.Context, target ClusterTarget, payload []byte) error
	PublishSocketRequest(ctx context.Context, socketID string, correlationID []byte, payload []byte) error
	PublishSocketResponse(ctx context.Context, targetInstanceID string, correlationID []byte, payload []byte) error
	PublishSocketRequestCancel(ctx context.Context, correlationID []byte) error
	PublishSocketDisconnect(ctx context.Context, target ClusterTarget, targetInstanceID string, closeCode uint16) error
	PublishChannelClose(ctx context.Context, channelID string) error
	SubscribeClusterEvents(ctx context.Context, event ClusterEventFunc, failure ClusterErrorFunc) error
}

// ClusterBuildCoordinator is an optional cluster capability used to coordinate
// GOX plugin builds when multiple server instances share the same project
// files. The lease token is a fencing token: adapters must reject renew,
// complete and fail operations made with an expired or superseded token.
//
// WaitBuild must also consult durable coordinator state before waiting for a
// notification. This makes completion observable even when publication occurs
// immediately before another instance begins waiting.
type ClusterBuildCoordinator interface {
	AcquireBuild(ctx context.Context, request ClusterBuildRequest) (ClusterBuildLease, error)
	RenewBuild(ctx context.Context, lease ClusterBuildLease, ttl time.Duration) (ClusterBuildLease, error)
	CompleteBuild(ctx context.Context, lease ClusterBuildLease, artifact ClusterBuildArtifact) error
	FailBuild(ctx context.Context, lease ClusterBuildLease, failure ClusterBuildFailure) error
	WaitBuild(ctx context.Context, request ClusterBuildWaitRequest) (ClusterBuildResult, error)
	// InvalidateBuild conditionally retires a completed result whose shared
	// files failed integrity or availability checks. Implementations must never
	// delete a pending lease or a different completed generation.
	InvalidateBuild(ctx context.Context, artifact ClusterBuildArtifact) error
}

// ClusterBuildRequest identifies one immutable source revision that needs a
// generated Go file and plugin. Key is stable for the logical route while
// SourceVersion is the content fingerprint of all source dependencies.
type ClusterBuildRequest struct {
	Key           string
	SourceVersion string
	LeaseTTL      time.Duration
}

// ClusterBuildLease grants one instance permission to publish a build result.
// Acquired=false means another instance owns the current source revision and
// the caller should use WaitBuild.
type ClusterBuildLease struct {
	Key             string
	SourceVersion   string
	Token           string
	OwnerInstanceID string
	ExpiresAt       time.Time
	Acquired        bool
}

// ClusterBuildArtifact is the committed, transport-independent description of
// a completed plugin build. Paths are project-relative so every instance can
// resolve the same shared files without embedding one machine's project root.
type ClusterBuildArtifact struct {
	Key                   string
	SourceVersion         string
	BuildVersion          string
	GeneratedRelativePath string
	PluginRelativePath    string
	PluginSize            int64
	PluginSHA256          string
	ManifestRelativePath  string
	ManifestSize          int64
	ManifestSHA256        string
}

// ClusterBuildFailure is safe coordinator state, not a serialized Go error.
// Message is intended for server diagnostics and must not contain secrets.
type ClusterBuildFailure struct {
	Key           string
	SourceVersion string
	Code          string
	Message       string
}

type ClusterBuildResultState uint8

const (
	ClusterBuildResultCompleted ClusterBuildResultState = iota + 1
	ClusterBuildResultFailed
)

type ClusterBuildResult struct {
	State    ClusterBuildResultState
	Artifact ClusterBuildArtifact
	Failure  ClusterBuildFailure
}

type ClusterBuildWaitRequest struct {
	Key           string
	SourceVersion string
}

// ClusterGroupDirectory is the authoritative distributed membership index for
// ephemeral socket groups. Implementations must keep socket and group indexes
// mutually consistent.
type ClusterGroupDirectory interface {
	JoinGroup(ctx context.Context, group string, socket ClusterSocketRecord) error
	LeaveGroup(ctx context.Context, group, socketID, instanceID string) error
	LeaveAllGroups(ctx context.Context, socketID, instanceID string) error
	ListGroups(ctx context.Context) ([]string, error)
	ListGroupSockets(ctx context.Context, group string) ([]ClusterSocketRecord, error)
	ListSocketGroups(ctx context.Context, socketID string) ([]string, error)
	CountGroups(ctx context.Context) (int, error)
	CountGroupSockets(ctx context.Context, group string) (int, error)
}

// ClusterChannelDirectory is the authoritative distributed membership index
// for ephemeral logical Channel instances. Channel IDs disappear when their
// last socket membership is removed.
type ClusterChannelDirectory interface {
	JoinChannel(ctx context.Context, channelID string, socket ClusterSocketRecord) error
	LeaveChannel(ctx context.Context, channelID, socketID, instanceID string) error
	LeaveAllChannels(ctx context.Context, socketID, instanceID string) error
	ListChannels(ctx context.Context) ([]string, error)
	ListChannelSockets(ctx context.Context, channelID string) ([]ClusterSocketRecord, error)
	ListSocketChannels(ctx context.Context, socketID string) ([]string, error)
	CountChannels(ctx context.Context) (int, error)
	CountChannelSockets(ctx context.Context, channelID string) (int, error)
	CloseChannel(ctx context.Context, channelID string) error
}

// ClusterSocketRecord is the transport-independent identity required to
// resolve one socket anywhere in a GOX cluster.
type ClusterSocketRecord struct {
	ID         string
	SessionID  string
	InstanceID string
}

// ClusterSocketDirectory provides the authoritative global socket registry.
// Every ClusterRuntime must implement it so public socket APIs never degrade
// into a partial instance-local view.
type ClusterSocketDirectory interface {
	RegisterSocket(ctx context.Context, record ClusterSocketRecord) error
	UnregisterSocket(ctx context.Context, socketID string) error
	GetSocket(ctx context.Context, socketID string) (ClusterSocketRecord, bool, error)
	ListSockets(ctx context.Context) ([]ClusterSocketRecord, error)
	ListSessionSockets(ctx context.Context, sessionID string) ([]ClusterSocketRecord, error)
	CountSockets(ctx context.Context) (int, error)
	RefreshInstance(ctx context.Context, instanceID string) error
	RemoveInstance(ctx context.Context, instanceID string) error
}
