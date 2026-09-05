//go:build !linux

package main

import (
	"fmt"
	"yomirelay/internal/aquarium"
)

func capture(string) (aquarium.Snapshot, error) {
	return aquarium.Snapshot{}, fmt.Errorf("AQUARIUM native diagnostics currently require Linux and Steam Proton")
}
