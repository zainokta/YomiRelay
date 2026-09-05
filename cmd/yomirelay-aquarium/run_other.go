//go:build !linux

package main

import (
	"context"
	"fmt"
)

func run(context.Context, string, string, string) error {
	return fmt.Errorf("AQUARIUM NeXAS live hook currently requires Linux and Steam Proton")
}
