package nexas

import (
	"testing"

	"yomirelay/internal/source/nexas/aquarium"
	"yomirelay/internal/testutil"
)

func TestDetectUsesRegisteredProfile(t *testing.T) {
	root := testutil.Aquarium(t)
	got, ok := Detect(aquarium.AppID, root)
	if !ok {
		t.Fatal("AQUARIUM profile was not detected")
	}
	if got.EngineConfidence != "high" || got.DialogueSource != "nexas-exec-hook" || got.SourceStatus != "unsupported-build" || got.ExecutableHash == "" {
		t.Fatalf("detection = %#v", got)
	}
	if _, ok := Detect("999", root); ok {
		t.Fatal("unregistered Steam app was treated as NeXAS")
	}
}
