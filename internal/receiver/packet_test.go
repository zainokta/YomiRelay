package receiver

import (
	"testing"
	"time"
)

func TestParsePacketPreservesJapaneseUTF8(t *testing.T) {
	data := []byte(`{"v":1,"gameId":"111","gameName":"影の物語","speaker":"林黙","text":"今日はどうする？","timestamp":1787895000}`)
	got, err := ParsePacket(data)
	if err != nil {
		t.Fatal(err)
	}
	if got.GameID != "111" || got.GameName != "影の物語" || got.Speaker != "林黙" || got.Text != "今日はどうする？" || !got.Timestamp.Equal(time.Unix(1787895000, 0)) {
		t.Fatalf("packet = %#v", got)
	}
}

func TestParsePacketRejectsMalformedInput(t *testing.T) {
	cases := [][]byte{
		[]byte(`{"v":1`),
		[]byte(`{"v":2,"gameId":"111","gameName":"name","text":"text","timestamp":1}`),
		[]byte(`{"v":1,"gameId":"111","gameName":"name","text":"","timestamp":1}`),
		[]byte(`{"v":1,"gameId":"111","text":"text","timestamp":1}`),
		[]byte(`{"v":1,"gameId":"111","gameName":"name","text":"text","timestamp":0}`),
		{0xff, '{', '}'},
	}
	for i, data := range cases {
		if _, err := ParsePacket(data); err == nil {
			t.Errorf("case %d accepted malformed packet", i)
		}
	}
}
