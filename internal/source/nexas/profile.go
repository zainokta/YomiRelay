package nexas

import (
	"fmt"
	"runtime"
	"time"

	"yomirelay/internal/source/nexas/aquarium"
)

type Game struct {
	AppID       string
	Name        string
	InstallPath string
}

type Event struct {
	Speaker   string
	Text      string
	Timestamp time.Time
}

type Sink func(Event)

type Line struct {
	Speaker string
	Text    string
}

type Detection struct {
	EngineConfidence string
	DialogueSource   string
	SourceStatus     string
	SourceMessage    string
	ExecutableHash   string
}

type buildInfo struct {
	Verified  bool
	Hash      string
	ImageSize uint32
}

type profile struct {
	AppID      string
	Executable string
	Inspect    func(string) (buildInfo, error)
	Signature  func() (pattern, mask []byte)
	Normalize  func(string) (Line, error)
}

var profiles = map[string]profile{
	aquarium.AppID: {
		AppID:      aquarium.AppID,
		Executable: "Aquarium.exe",
		Inspect: func(root string) (buildInfo, error) {
			build, err := aquarium.Inspect(root)
			if err != nil {
				return buildInfo{}, err
			}
			return buildInfo{Verified: build.VerifiedBuild, Hash: build.SHA256, ImageSize: build.ImageSize}, nil
		},
		Signature: aquarium.HookSignature,
		Normalize: func(raw string) (Line, error) {
			line, err := aquarium.NormalizeHookText(raw)
			if err != nil {
				return Line{}, err
			}
			return Line{Speaker: line.Speaker, Text: line.Text}, nil
		},
	},
}

func Detect(appID, root string) (Detection, bool) {
	p, ok := profiles[appID]
	if !ok {
		return Detection{}, false
	}
	build, err := p.Inspect(root)
	if err != nil {
		return Detection{}, false
	}
	result := Detection{
		EngineConfidence: "high",
		DialogueSource:   "nexas-exec-hook",
		ExecutableHash:   build.Hash,
		SourceStatus:     "unsupported-build",
		SourceMessage:    "This executable version is not supported by the live NeXAS hook.",
	}
	if !build.Verified {
		return result, true
	}
	if runtime.GOOS != "linux" {
		result.SourceStatus = "unsupported-platform"
		result.SourceMessage = "Live NeXAS hooking currently requires Linux and Steam Proton."
		return result, true
	}
	result.SourceStatus = "native-auto"
	result.SourceMessage = "Live NeXAS execution hook is enabled automatically; its instruction signature is resolved from the loaded Proton process at runtime."
	return result, true
}

func profileFor(appID string) (profile, error) {
	p, ok := profiles[appID]
	if !ok {
		return profile{}, fmt.Errorf("no NeXAS profile for Steam app %s", appID)
	}
	return p, nil
}
