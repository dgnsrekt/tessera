# tessera

A keyboard-driven terminal controller for the **TESmart Ultra HD 4K HDMI 4×4 matrix switcher** —
a single, dependency-free Go binary.

A *tessera* is a single tile in a mosaic — this app routes every input "tile" into place. It
talks the matrix's raw-TCP control protocol directly, shows a live routing grid, and puts
switching, named scenes, and the buzzer one keystroke away. Because it's a TUI it works locally or
over SSH.

```
● 10.10.0.1:5000  connected    buzzer off   [GRID]   tessera 0.1.0

           Input 1  Input 2  Input 3  Input 4
Output 1      ●        ·        ·        ·
Output 2      ·        ●        ·        ·
Output 3      ·        ·        ●        ·
Output 4      ·        ·        ·        ●

↑↓←→ move  enter route  1-4 all→input  m mirror  tab scenes  b buzzer  e labels  R refresh  q quit
```

## Install & run

Requires Go ≥ 1.24.

```bash
git clone https://github.com/dgnsrekt/tessera
cd tessera
go build -o tessera .     # build the binary
./tessera
```

Install it onto your `PATH`:

```bash
go install github.com/dgnsrekt/tessera@latest   # -> $GOBIN/tessera
```

Override the endpoint without editing config:

```bash
./tessera --host 192.168.1.10 --port 5000
```

It's pure Go with no cgo, so it cross-compiles trivially:

```bash
GOOS=darwin  GOARCH=arm64 go build -o tessera-macos .
GOOS=windows GOARCH=amd64 go build -o tessera.exe .
```

## Keys

### Grid view

| Key | Action |
|---|---|
| `↑ ↓ ← →` / `h j k l` | move the grid cursor |
| `enter` / `space` | route the cursor's input → output |
| `1`–`4` | send that input to **all** outputs |
| `m` | mirror (out1←in1 … out4←in4) |
| `tab` | switch to **Scene Mode** |
| `b` | toggle the unit's buzzer |
| `e` | rename the cursor's input/output |
| `R` | force a status refresh |
| `q` / `ctrl+c` | quit |

The grid auto-refreshes ~once per second, so changes made from the front panel or IR remote show
up on their own.

### Scene Mode (`tab`)

Scenes are named, described routing snapshots — a friendlier replacement for raw preset slots.
The menu lists your scenes with a live `●` marker on whichever one matches the current routing,
and a detail pane previews the selected scene's routing using your labels.

| Key | Action |
|---|---|
| `↑ ↓` / `j k` | move the selection |
| `enter` / `space` | **apply** the selected scene (replays its routing) |
| `n` | **new** scene — capture the current routing, then name it |
| `s` | **update** the selected scene's routing from the current state |
| `e` | **edit** the selected scene's name / description / hardware slot |
| `d` | **delete** the selected scene (confirm with `y`) |
| `tab` / `esc` | back to Grid view |

In the scene editor, `tab`/`↑↓` move between fields, `←→` change the hardware slot, `enter` saves,
`esc` cancels.

**Hybrid storage.** Scenes live in `config.toml`, so they're unlimited, previewable, and portable;
applying one replays its routing over TCP. If you assign a scene a **hardware slot (1-8)**, tessera
also writes that preset into the unit's memory when the scene is captured — so those scenes are
recallable from the physical remote / front panel too.

## Configuration

tessera stores its settings in `~/.config/tessera/config.toml` (created with defaults on first run,
without any scenes). A populated file looks like:

```toml
host = "10.10.0.1"
port = 5000
poll_interval = 1.0
inputs = ["Input 1", "Input 2", "Input 3", "Input 4"]
outputs = ["Output 1", "Output 2", "Output 3", "Output 4"]

[[scenes]]
  name = "Movie Night"
  description = "Apple TV everywhere"
  routes = [2, 2, 2, 2]   # index = output-1, value = input (0 = unset)
  slot = 1                # 0 = none; 1-8 = also mirror to a hardware preset

[[scenes]]
  name = "Work"
  description = ""
  routes = [1, 2, 3, 4]
  slot = 0
```

Custom labels (set with `e`, or edited here) and scenes are stored locally because the device
protocol has no naming or rich-preset support of its own. You can hand-edit scenes here or manage
them entirely from Scene Mode.

## Protocol

tessera speaks the TESmart ASCII control protocol over raw TCP (default port `5000`). Each command
is sent terminated with `\r\n`.

| Command | Action | Reply |
|---|---|---|
| `MT00SWxxyyNT` | route input `xx` → output `yy` (yy=00 ⇒ all outputs) | none |
| `MT00SW0000NT` | mirror | none |
| `MT00RD0000NT` | read status | `LINK:O1I1;O2I2;O3I3;O4I4;END` |
| `MT00SV00ppNT` | save routing to preset `pp` (01–08) | none |
| `MT00RD01ppNT` | recall preset `pp` (01–08) | none |
| `MT00BZEN00NT` / `MT00BZEN01NT` | buzzer on / off | none |

The status reply arrives across several TCP segments and is slow to send `END`; tessera reads
until `END` (or a timeout) rather than doing a single read.

## Project layout

```
main.go                       CLI entry: flags, config, runs the Bubble Tea program
internal/matrix/protocol.go   command formatters + status parser (pure)
internal/matrix/client.go     persistent TCP client (read-until-END, mutex-serialized)
internal/config/config.go     TOML config: labels + scenes at ~/.config/tessera/
tui/                          Bubble Tea model, grid + scene views, keys
```

## Development

```bash
go test ./...                              # unit + TUI white-box tests
TESSERA_LIVE=1 go test ./internal/matrix/  # read-only check against a real device
go vet ./... && gofmt -l .
```

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea),
[Lipgloss](https://github.com/charmbracelet/lipgloss), and
[Bubbles](https://github.com/charmbracelet/bubbles).

## License

MIT — see [LICENSE](LICENSE).
