package aquarium

import (
	"strings"
	"testing"
)

func TestNormalizeCandidate(t *testing.T) {
	got, err := NormalizeCandidate(Candidate{
		Address: "0x1234",
		Raw:     "【トーレス】@n@v20002「だって、たった一瞬とは言え、キミと愛し合えたのだから」",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Address != "0x1234" || got.Speaker != "トーレス" || got.Text != "だって、たった一瞬とは言え、キミと愛し合えたのだから" {
		t.Fatalf("preview = %#v", got)
	}
	for _, raw := range []string{
		"【選択肢】@n＞日本語@n　English@n",
		"【名前】@n「",
		"名前@n「台詞」",
		"【名前】@n\xff「台詞」",
	} {
		if _, err := NormalizeCandidate(Candidate{Raw: raw}); err == nil {
			t.Fatalf("accepted invalid candidate %q", raw)
		}
	}
}

func TestNormalizeCandidateConvertsLineBreakTag(t *testing.T) {
	got, err := NormalizeCandidate(Candidate{Raw: "【湊あくあ】@n@v10001「おはよう@nございます！」"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "おはよう\nございます！" {
		t.Fatalf("text = %q", got.Text)
	}
}

func TestBuildPreviewFiltersNoiseAndWarnsAboutOrdering(t *testing.T) {
	preview := BuildPreview(Snapshot{
		Status:  "unverified",
		Message: "Memory candidates may include backlog copies.",
		Candidates: []Candidate{
			{Address: "0x1", Raw: "【湊あくあ】@n「おはよう！」"},
			{Address: "0x2", Raw: "【選択肢】@n＞日本語@n　English@n"},
		},
	})
	if len(preview.Candidates) != 1 || preview.Candidates[0].Text != "おはよう！" {
		t.Fatalf("preview = %#v", preview)
	}
	if !strings.Contains(preview.Message, "not chronological") || !strings.Contains(preview.Message, "never added to Reader history") {
		t.Fatalf("message = %q", preview.Message)
	}
}
