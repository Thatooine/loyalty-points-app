// Package scope reasons about the scope segment of a permission string.
//
// A permission is formatted "resource:action[:scope]" — for example
// "account:read:own", "wallet:transact:all", or "wallet:batch:all". The
// optional third colon-separated segment is the scope: "own" restricts the
// caller to their own data, "all" grants access to every owner's data. Some
// permissions carry no scope segment at all (e.g. a hypothetical
// "system:ping").
//
// The package is a pure parser: it inspects permission strings and has no
// dependencies. Whether a caller actually holds a permission is answered by
// authorization.IsGranted, which reads the login claim from the request
// context.
package scope

import "strings"

// Scope is the access breadth encoded in a permission string.
type Scope string

const (
	// Own restricts the caller to data they own.
	Own Scope = "own"
	// All grants the caller access across every owner.
	All Scope = "all"
)

// Of returns the scope segment of a permission and true when one is present,
// or ("", false) when the permission carries no scope segment.
func Of(permission string) (Scope, bool) {
	parts := strings.Split(permission, ":")
	if len(parts) < 3 || parts[2] == "" {
		return "", false
	}
	return Scope(parts[2]), true
}

// IsAll reports whether the permission's scope is "all".
func IsAll(permission string) bool {
	s, ok := Of(permission)
	return ok && s == All
}

// IsOwn reports whether the permission's scope is "own".
func IsOwn(permission string) bool {
	s, ok := Of(permission)
	return ok && s == Own
}
