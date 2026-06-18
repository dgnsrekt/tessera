package matrix

import (
	"reflect"
	"testing"
)

func TestCommandFormatters(t *testing.T) {
	cases := []struct {
		got, want string
	}{
		{CmdSwitch(2, 3), "MT00SW0203NT"},
		{CmdSwitch(1, 0), "MT00SW0100NT"}, // all outputs
		{CmdMirror(), "MT00SW0000NT"},
		{CmdReadStatus(), "MT00RD0000NT"},
		{CmdSavePreset(1), "MT00SV0001NT"},
		{CmdRecallPreset(8), "MT00RD0108NT"},
		{CmdBuzzer(true), "MT00BZEN00NT"},
		{CmdBuzzer(false), "MT00BZEN01NT"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}

func TestParseStatus(t *testing.T) {
	want := map[int]int{1: 1, 2: 2, 3: 3, 4: 4}
	if got := ParseStatus("LINK:O1I1;O2I2;O3I3;O4I4;END"); !reflect.DeepEqual(got, want) {
		t.Errorf("clean: got %v, want %v", got, want)
	}

	// tolerant of CRLF framing and surrounding junk
	want2 := map[int]int{1: 3, 2: 3, 3: 3, 4: 3}
	if got := ParseStatus("\r\nLINK:O1I3;O2I3;O3I3;O4I3;END\r\n"); !reflect.DeepEqual(got, want2) {
		t.Errorf("framed: got %v, want %v", got, want2)
	}

	// no LINK line -> empty (non-nil) map
	if got := ParseStatus("garbage"); len(got) != 0 {
		t.Errorf("garbage: got %v, want empty", got)
	}

	// a partial line without END must be ignored
	if got := ParseStatus("LINK:O1I1;O2I2"); len(got) != 0 {
		t.Errorf("partial: got %v, want empty", got)
	}
}
