package aquarium

import (
	"strings"
	"testing"
)

func TestCandidatesPreserveCompleteJapaneseAndRejectPartialBuffers(t *testing.T) {
	raw := "【トーレス】@n@v20002「漢字、ひらがな、カタカナ……〜ー」"
	data := append([]byte{0xff, 0x80, 0}, []byte(raw+"\x00menu\x00【途中】@n未完")...)
	got := Candidates(data, 0x1000)
	if len(got) != 1 || got[0].Raw != raw || got[0].Address != "0x1003" {
		t.Fatalf("candidates = %+v", got)
	}
	for _, invalid := range []string{"【設定】音量\x00", "【名前】@n\xff\x00", "【名前】@n" + strings.Repeat("あ", 8192) + "\x00"} {
		if got := Candidates([]byte(invalid), 0); len(got) != 0 {
			t.Fatalf("accepted invalid candidate: %+v", got)
		}
	}
}

func TestCandidateSnapshotIsBounded(t *testing.T) {
	data := []byte(strings.Repeat("【名前】@n「話」\x00", 200))
	if got := Candidates(data, 0); len(got) != 128 {
		t.Fatalf("unbounded candidate count: %d", len(got))
	}
}
