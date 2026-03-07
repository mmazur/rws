package userutil

import (
	"os"
	"os/user"
)

// DiscoverUsername returns the current OS username and whether it was found.
func DiscoverUsername() (string, bool) {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username, true
	}
	if name := os.Getenv("USER"); name != "" {
		return name, true
	}
	return "", false
}
