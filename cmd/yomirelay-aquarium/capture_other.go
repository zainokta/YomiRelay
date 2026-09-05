//go:build !linux

package main

import (
	"fmt"

	"yomirelay/internal/source/aquarium"
)

func capture(string) (aquarium.Snapshot, error) {
	return aquarium.Snapshot{}, fmt.Errorf("AQUARIUM native preview currently requires Linux and Steam Proton")
}
