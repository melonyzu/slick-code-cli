package tool

import (
	"fmt"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// Policy decides whether a tool call may run. Allow returns nil to
// permit the call, or an error of kind types.ErrorKindPermissionDenied
// explaining the denial.
type Policy interface {
	Allow(name string, perm Permission) error
}

// PermissionPolicy allows every tool whose required permission is in a
// fixed granted set, regardless of the tool's name.
type PermissionPolicy struct {
	granted map[Permission]bool
}

// NewPermissionPolicy returns a Policy granting exactly the given
// permission levels.
func NewPermissionPolicy(perms ...Permission) *PermissionPolicy {
	granted := make(map[Permission]bool, len(perms))
	for _, p := range perms {
		granted[p] = true
	}
	return &PermissionPolicy{granted: granted}
}

// Allow implements Policy.
func (p *PermissionPolicy) Allow(name string, perm Permission) error {
	if p.granted[perm] {
		return nil
	}
	return types.NewError(types.ErrorKindPermissionDenied,
		fmt.Sprintf("tool %q requires %s permission, which is not granted", name, perm))
}
