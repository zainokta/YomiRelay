//go:build !linux

package nexas

import (
	"context"
	"fmt"
	"log"
)

func Start(context.Context, Game, Sink, *log.Logger) error {
	return fmt.Errorf("live NeXAS hooking currently requires Linux and Steam Proton")
}
