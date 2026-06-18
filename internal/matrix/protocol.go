// Package matrix speaks the TESmart HDMI matrix switcher's raw-TCP control
// protocol (ASCII, header "MT00", suffix "NT").
//
// Confirmed against the open-source Bitfocus Companion module for these units:
//
//	MT00SWxxyyNT   route input xx -> output yy (yy=00 => all outputs)
//	MT00SW0000NT   mirror (out1<-in1 .. outN<-inN)
//	MT00RD0000NT   read status   -> "LINK:O1I1;O2I2;...;END"
//	MT00SV00ppNT   save current routing to preset pp (01..08)
//	MT00RD01ppNT   recall preset pp (01..08)
//	MT00BZEN00NT   buzzer on
//	MT00BZEN01NT   buzzer off
package matrix

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// term is appended to every command on the wire.
const term = "\r\n"

// tokenRE matches a single "O<out>I<in>" routing token in a status reply.
var tokenRE = regexp.MustCompile(`O(\d+)I(\d+)`)

// CmdSwitch routes input in to output out. out=0 means "all outputs".
func CmdSwitch(in, out int) string { return fmt.Sprintf("MT00SW%02d%02dNT", in, out) }

// CmdMirror routes out1<-in1 .. outN<-inN.
func CmdMirror() string { return "MT00SW0000NT" }

// CmdReadStatus requests the current routing.
func CmdReadStatus() string { return "MT00RD0000NT" }

// CmdSavePreset stores the current routing in preset slot p (1..8).
func CmdSavePreset(p int) string { return fmt.Sprintf("MT00SV00%02dNT", p) }

// CmdRecallPreset recalls preset slot p (1..8).
func CmdRecallPreset(p int) string { return fmt.Sprintf("MT00RD01%02dNT", p) }

// CmdBuzzer toggles the confirmation beep. Per the protocol BZEN00=on, BZEN01=off.
func CmdBuzzer(on bool) string {
	if on {
		return "MT00BZEN00NT"
	}
	return "MT00BZEN01NT"
}

// ParseStatus extracts {output: input} from a "LINK:O1I1;...;END" reply.
// It returns an empty (non-nil) map when no complete LINK line is present.
func ParseStatus(text string) map[int]int {
	routes := make(map[int]int)
	lines := strings.FieldsFunc(text, func(r rune) bool { return r == '\r' || r == '\n' })
	for _, line := range lines {
		if !strings.Contains(line, "LINK:") || !strings.Contains(line, "END") {
			continue
		}
		for _, m := range tokenRE.FindAllStringSubmatch(line, -1) {
			out, _ := strconv.Atoi(m[1])
			in, _ := strconv.Atoi(m[2])
			routes[out] = in
		}
	}
	return routes
}
