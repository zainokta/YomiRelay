package nexas

import (
	"testing"
	"time"
)

func TestRenderContextFilterSelectsDialogueAndSuppressesUI(t *testing.T) {
	var selected uint64
	filter := newRenderContextFilter(func(contextID uint64) { selected = contextID })
	start := time.Unix(100, 0)

	filter.Add(0x2000, Line{Text: "エミリー"}, start)
	if got := filter.FlushDue(start.Add(100 * time.Millisecond)); len(got) != 0 {
		t.Fatalf("standalone name leaked: %#v", got)
	}

	first := Line{Text: "ごめんなさい……私が、道ならぬ恋を願ってしまったから……"}
	filter.Add(0x1000, first, start.Add(200*time.Millisecond))
	if got := filter.FlushDue(start.Add(300 * time.Millisecond)); len(got) != 0 {
		t.Fatalf("dialogue context selected too early: %#v", got)
	}

	filter.Add(0x3000, Line{Text: "オートセーブしました"}, start.Add(400*time.Millisecond))
	if got := filter.FlushDue(start.Add(500 * time.Millisecond)); len(got) != 0 {
		t.Fatalf("autosave text leaked: %#v", got)
	}

	second := Line{Text: "あなたの恋人になろうとしたばっかりに、館を追われ、隠れ家を焼かれ……"}
	filter.Add(0x1000, second, start.Add(600*time.Millisecond))
	got := filter.FlushDue(start.Add(700 * time.Millisecond))
	if selected != 0x1000 {
		t.Fatalf("selected context = 0x%x", selected)
	}
	if len(got) != 2 || got[0].Text != first.Text || got[1].Text != second.Text {
		t.Fatalf("selected dialogue = %#v", got)
	}

	filter.Add(0x4000, Line{Text: "テキスト表示速度を変更します。"}, start.Add(800*time.Millisecond))
	if got := filter.FlushDue(start.Add(900 * time.Millisecond)); len(got) != 0 {
		t.Fatalf("settings text leaked after selection: %#v", got)
	}

	short := Line{Text: "ええ。"}
	filter.Add(0x1000, short, start.Add(time.Second))
	got = filter.FlushDue(start.Add(1100 * time.Millisecond))
	if len(got) != 1 || got[0].Text != short.Text {
		t.Fatalf("short dialogue from selected context was dropped: %#v", got)
	}
}

func TestRenderContextFilterDoesNotSelectShortUIContext(t *testing.T) {
	filter := newRenderContextFilter(nil)
	start := time.Unix(200, 0)
	for i, text := range []string{"エミリー", "設定", "オートセーブ", "音量", "戻る"} {
		at := start.Add(time.Duration(i) * 200 * time.Millisecond)
		filter.Add(0x9000, Line{Text: text}, at)
		if got := filter.FlushDue(at.Add(100 * time.Millisecond)); len(got) != 0 {
			t.Fatalf("UI text leaked: %#v", got)
		}
	}
	if filter.selected != 0 {
		t.Fatalf("short UI context was selected: 0x%x", filter.selected)
	}
}

func TestDialogueLikelihoodKeepsShortSentenceCandidate(t *testing.T) {
	if score := dialogueLikelihood(Line{Text: "ごめんなさい……"}); score < renderContextCandidateScore {
		t.Fatalf("short sentence score = %d", score)
	}
	if score := dialogueLikelihood(Line{Text: "エミリー"}); score != 0 {
		t.Fatalf("standalone name score = %d", score)
	}
}
