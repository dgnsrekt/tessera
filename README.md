# tessera

A keyboard-driven terminal controller for the **TESmart Ultra HD 4K HDMI 4×4 matrix switcher**.

A *tessera* is a single tile in a mosaic — this app routes every input "tile" into place. It
talks the matrix's raw-TCP control protocol directly, shows a live routing grid, and puts
switching, presets, and the buzzer one keystroke away. Because it's a TUI it works locally or
over SSH.

```
● 10.10.0.1:5000  connected   buzzer off   tessera v0.1.0
┌────────────────────────────────────────────────┐
│            Input 1  Input 2  Input 3  Input 4   │
│  Output 1     ●        ·        ·        ·       │
│  Output 2     ·        ●        ·        ·       │
│  Output 3     ·        ·        ●        ·       │
│  Output 4     ·        ·        ·        ●       │
└────────────────────────────────────────────────┘
↑↓←→ move  enter route  1-4 all→input  m mirror  s+1-8 save  r+1-8 recall  b buzzer  e labels  q quit
```

## Install & run

Requires [uv](https://docs.astral.sh/uv/) and Python ≥ 3.11.

```bash
git clone https://github.com/dgnsrekt/tessera
cd tessera
uv run tessera                 # resolves deps into a local venv and launches
```

Install it as a global command:

```bash
uv tool install .              # then just run: tessera
```

Override the endpoint without editing config:

```bash
uv run tessera --host 192.168.1.10 --port 5000
```

## Keys

| Key | Action |
|---|---|
| `↑ ↓ ← →` | move the grid cursor |
| `enter` | route the cursor's input → output |
| `1`–`4` | send that input to **all** outputs |
| `m` | mirror (out1←in1 … out4←in4) |
| `s` then `1`–`8` | **save** current routing to a preset slot |
| `r` then `1`–`8` | **recall** a preset slot |
| `b` | toggle the unit's buzzer |
| `e` | rename the cursor's input/output |
| `R` | force a status refresh |
| `q` | quit |

The grid auto-refreshes ~once per second, so changes made from the front panel or IR remote
show up on their own.

## Configuration

On first run tessera writes `~/.config/tessera/config.toml`:

```toml
host = "10.10.0.1"
port = 5000
poll_interval = 1.0
inputs  = ["Input 1", "Input 2", "Input 3", "Input 4"]
outputs = ["Output 1", "Output 2", "Output 3", "Output 4"]
```

Custom labels (set with `e`, or edited here) are stored locally because the device protocol has
no naming of its own.

## Protocol

tessera speaks the TESmart ASCII control protocol over raw TCP (default port `5000`). Each
command is sent terminated with `\r\n`.

| Command | Action | Reply |
|---|---|---|
| `MT00SWxxyyNT` | route input `xx` → output `yy` | none |
| `MT00SWxx00NT` | input `xx` → all outputs | none |
| `MT00SW0000NT` | mirror | none |
| `MT00RD0000NT` | read status | `LINK:O1I1;O2I2;O3I3;O4I4;END` |
| `MT00SV00ppNT` | save routing to preset `pp` (01–08) | none |
| `MT00RD01ppNT` | recall preset `pp` (01–08) | none |
| `MT00BZEN00NT` / `MT00BZEN01NT` | buzzer on / off | none |

The status reply arrives across several TCP segments and is slow to send `END`; tessera reads
until `END` (or a timeout) rather than doing a single read.

## License

MIT — see [LICENSE](LICENSE).
