package tool

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// fakeTool is a configurable Tool for framework tests.
type fakeTool struct {
	name    string
	perm    Permission
	execute func(ctx context.Context, exec ExecContext, input json.RawMessage) (string, error)
}

func (f *fakeTool) Definition() types.Tool {
	return types.Tool{Name: f.name, Description: "a fake tool for tests"}
}

func (f *fakeTool) Permission() Permission {
	return f.perm
}

func (f *fakeTool) Execute(ctx context.Context, exec ExecContext, input json.RawMessage) (string, error) {
	if f.execute == nil {
		return "ok", nil
	}
	return f.execute(ctx, exec, input)
}

func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	ft := &fakeTool{name: "fake", perm: PermissionRead}

	if err := r.Register(ft); err != nil {
		t.Fatalf("Register: %v", err)
	}

	got, err := r.Get("fake")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != Tool(ft) {
		t.Errorf("Get returned %v, want the registered tool", got)
	}
}

func TestRegistryRejectsDuplicates(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(&fakeTool{name: "fake"}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	err := r.Register(&fakeTool{name: "fake"})
	if err == nil {
		t.Fatal("Register accepted a duplicate name")
	}
	if kind := types.KindOf(err); kind != types.ErrorKindInternal {
		t.Errorf("duplicate registration error kind = %q, want %q", kind, types.ErrorKindInternal)
	}
}

func TestRegistryRejectsEmptyName(t *testing.T) {
	if err := NewRegistry().Register(&fakeTool{}); err == nil {
		t.Fatal("Register accepted a tool with an empty name")
	}
}

func TestRegistryGetUnknown(t *testing.T) {
	_, err := NewRegistry().Get("missing")
	if err == nil {
		t.Fatal("Get returned no error for an unregistered tool")
	}
	if kind := types.KindOf(err); kind != types.ErrorKindValidation {
		t.Errorf("unknown tool error kind = %q, want %q", kind, types.ErrorKindValidation)
	}
}

func TestRegistryDiscovery(t *testing.T) {
	r := NewRegistry()
	for _, name := range []string{"zeta", "alpha", "mid"} {
		if err := r.Register(&fakeTool{name: name}); err != nil {
			t.Fatalf("Register %s: %v", name, err)
		}
	}

	want := []string{"alpha", "mid", "zeta"}
	if got := r.List(); !reflect.DeepEqual(got, want) {
		t.Errorf("List() = %v, want %v", got, want)
	}

	defs := r.Definitions()
	if len(defs) != len(want) {
		t.Fatalf("Definitions() returned %d entries, want %d", len(defs), len(want))
	}
	for i, def := range defs {
		if def.Name != want[i] {
			t.Errorf("Definitions()[%d].Name = %q, want %q", i, def.Name, want[i])
		}
	}
}
