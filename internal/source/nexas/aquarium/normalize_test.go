package aquarium

import "testing"

func TestNormalizeHookText(t *testing.T) {
	got, err := NormalizeHookText("【トーレス】@n@v20002「だって、たった一瞬とは言え、キミと愛し合えたのだから」")
	if err != nil {
		t.Fatal(err)
	}
	if got.Speaker != "トーレス" || got.Text != "だって、たった一瞬とは言え、キミと愛し合えたのだから" {
		t.Fatalf("line = %#v", got)
	}
}

func TestNormalizeHookTextSupportsNarrationAndLineBreaks(t *testing.T) {
	got, err := NormalizeHookText("これは@n地の文です。")
	if err != nil {
		t.Fatal(err)
	}
	if got.Speaker != "" || got.Text != "これは\n地の文です。" {
		t.Fatalf("line = %#v", got)
	}
}

func TestNormalizeHookTextRejectsNoiseAndUnknownTags(t *testing.T) {
	for _, raw := range []string{
		"【選択肢】@n＞日本語@n　English@n",
		"Settings",
		"【名前】@x123「台詞」",
		"【名前】@v12「台詞」",
	} {
		if _, err := NormalizeHookText(raw); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
}
