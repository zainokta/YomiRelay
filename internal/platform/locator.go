// Package platform locates platform-specific Steam installations.
package platform

// SteamLocator finds existing Steam roots without scanning arbitrary paths.
type SteamLocator interface {
	FindSteamRoots() ([]string, error)
}

// NewSteamLocator returns the implementation for the current build target.
func NewSteamLocator() SteamLocator {
	return newSteamLocator()
}
