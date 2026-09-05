//go:build linux

package main

import "testing"

func TestImageBaseFromMaps(t *testing.T) {
	maps := "00400000-00401000 r--p 00000000 08:01 1 /games/AQUARIUM/Aquarium.exe\n" +
		"00401000-00500000 r-xp 00001000 08:01 1 /games/AQUARIUM/Aquarium.exe\n"
	got, ok := imageBaseFromMaps(maps, "/games/AQUARIUM/Aquarium.exe")
	if !ok || got != 0x00400000 {
		t.Fatalf("base = %#x, %v", got, ok)
	}
}

func TestImageBaseFromMapsHandlesEscapedSpace(t *testing.T) {
	maps := "10000000-10001000 r--p 00000000 08:01 1 /games/Steam\\040Library/Aquarium.exe\n"
	got, ok := imageBaseFromMaps(maps, "/games/Steam Library/Aquarium.exe")
	if !ok || got != 0x10000000 {
		t.Fatalf("base = %#x, %v", got, ok)
	}
}
