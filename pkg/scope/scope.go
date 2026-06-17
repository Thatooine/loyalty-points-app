// Package scope is a pure parser for the optional scope segment of a
// "resource:action[:scope]" permission string ("own", "all", or absent).
package scope

import "strings"

type Scope string

const (
	Own Scope = "own"
	All Scope = "all"
)

// Of returns the scope segment of a permission, or ("", false) when absent.
func Of(permission string) (Scope, bool) {
	parts := strings.Split(permission, ":")
	if len(parts) < 3 || parts[2] == "" {
		return "", false
	}
	return Scope(parts[2]), true
}

func IsAll(permission string) bool {
	s, ok := Of(permission)
	return ok && s == All
}

func IsOwn(permission string) bool {
	s, ok := Of(permission)
	return ok && s == Own
}
