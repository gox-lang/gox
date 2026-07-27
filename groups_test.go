package gox

import (
	"strings"
	"testing"
)

func TestGroupNamesAreStrictAndNeverNormalized(t *testing.T) {
	valid := []string{"admins", "Admins", "my group", "equipo-ñ"}
	for _, name := range valid {
		if err := ValidateGroupName(name); err != nil {
			t.Fatalf("ValidateGroupName(%q): %v", name, err)
		}
	}
	invalid := []string{"", " admins", "admins ", "line\nbreak", strings.Repeat("a", MaxGroupNameBytes+1)}
	for _, name := range invalid {
		if err := ValidateGroupName(name); err == nil {
			t.Fatalf("ValidateGroupName(%q) succeeded", name)
		}
	}
}

func TestSocketGroupShorthandsUseRuntimeBindings(t *testing.T) {
	socket := &Socket{}
	joined := ""
	left := ""
	emitted := ""
	BindSocketGroups(socket, SocketGroupBindings{
		Join:     func(group string) error { joined = group; return nil },
		Leave:    func(group string) error { left = group; return nil },
		LeaveAll: func() error { left = "*"; return nil },
		Groups:   func() []string { return []string{"admins"} },
		EmitTo:   func(group, event string, values []any) { emitted = group + ":" + event },
	})
	if err := socket.Join("admins"); err != nil || joined != "admins" {
		t.Fatalf("Join: joined=%q err=%v", joined, err)
	}
	if groups := socket.Groups(); len(groups) != 1 || groups[0] != "admins" {
		t.Fatalf("Groups = %#v", groups)
	}
	socket.EmitTo("admins", "announcement", 42)
	if emitted != "admins:announcement" {
		t.Fatalf("EmitTo = %q", emitted)
	}
	if err := socket.Leave("admins"); err != nil || left != "admins" {
		t.Fatalf("Leave: left=%q err=%v", left, err)
	}
	if err := socket.LeaveAll(); err != nil || left != "*" {
		t.Fatalf("LeaveAll: left=%q err=%v", left, err)
	}
}

func TestGlobalGroupsAreZeroSafeAndGenerationSafe(t *testing.T) {
	oldCleanup := BindGroups(GroupRegistryBindings{List: func() []string { return []string{"old"} }})
	newCleanup := BindGroups(GroupRegistryBindings{List: func() []string { return []string{"new"} }})
	oldCleanup()
	if list := Groups.List(); len(list) != 1 || list[0] != "new" {
		t.Fatalf("older cleanup changed Groups: %#v", list)
	}
	newCleanup()
	if Groups.Count() != 0 || Groups.List() == nil || Groups.Members("missing").All() == nil {
		t.Fatal("unbound Groups must be safely empty")
	}
}

func TestGlobalGroupMembersRemainDynamicAndEmitThroughGroupTransport(t *testing.T) {
	first := &Socket{ID: "first"}
	current := []*Socket{first}
	received := ""
	cleanup := BindGroups(GroupRegistryBindings{
		Members:      func(string) []*Socket { return current },
		CountMembers: func(string) int { return len(current) },
		EmitTo:       func(group, event string, values []any) { received = group + ":" + event },
	})
	defer cleanup()
	members := Groups.Members("admins")
	if members.Count() != 1 {
		t.Fatalf("members count = %d", members.Count())
	}
	current = append(current, &Socket{ID: "second"})
	if members.Count() != 2 {
		t.Fatalf("dynamic members count = %d", members.Count())
	}
	members.Emit("announcement", 42)
	if received != "admins:announcement" {
		t.Fatalf("group emission = %q", received)
	}
}
