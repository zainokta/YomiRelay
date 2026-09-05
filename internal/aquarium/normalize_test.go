package aquarium

import (
	"testing"
	"time"
)

func TestNormalizeCandidate(t *testing.T) {
	got, err := NormalizeCandidate("【トーレス】@n@v20002「だって、たった一瞬とは言え、キミと愛し合えたのだから」", time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if got.Speaker != "トーレス" || got.Text != "だって、たった一瞬とは言え、キミと愛し合えたのだから" || got.Engine != "nexas" {
		t.Fatalf("dialogue = %#v", got)
	}
	for _, raw := range []string{
		"【選択肢】@n＞日本語@n　English@n",
		"【名前】@n「",
		"名前@n「台詞」",
		"【名前】@n\xff「台詞」",
	} {
		if _, err := NormalizeCandidate(raw, time.Unix(1, 0)); err == nil {
			t.Fatalf("accepted invalid candidate %q", raw)
		}
	}
}

func TestNormalizeCandidateConvertsLineBreakTag(t *testing.T) {
	got, err := NormalizeCandidate("【湊あくあ】@n@v10001「おはよう@nございます！」", time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "おはよう\nございます！" {
		t.Fatalf("text = %q", got.Text)
	}
}
