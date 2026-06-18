"""tessera — Textual TUI for the TESmart HDMI matrix switcher."""

from __future__ import annotations

import argparse

from rich.text import Text
from textual import events
from textual.app import App, ComposeResult
from textual.containers import Grid, Vertical
from textual.coordinate import Coordinate
from textual.screen import ModalScreen
from textual.widgets import Button, DataTable, Input, Label, Static

from . import __version__, config
from .client import MatrixClient

DOT = "●"


class LabelEditScreen(ModalScreen[tuple[str, str] | None]):
    """Modal to rename the input/output under the grid cursor."""

    BINDINGS = [("escape", "cancel", "Cancel")]

    def __init__(self, in_idx: int, in_name: str, out_idx: int, out_name: str):
        super().__init__()
        self._in_idx = in_idx
        self._out_idx = out_idx
        self._in_name = in_name
        self._out_name = out_name

    def compose(self) -> ComposeResult:
        with Vertical(id="dialog"):
            yield Label(f"Edit labels (Input {self._in_idx} / Output {self._out_idx})", id="dialog-title")
            yield Label(f"Input {self._in_idx} name:")
            yield Input(value=self._in_name, id="in_name")
            yield Label(f"Output {self._out_idx} name:")
            yield Input(value=self._out_name, id="out_name")
            with Grid(id="dialog-buttons"):
                yield Button("Save", variant="primary", id="save")
                yield Button("Cancel", id="cancel")

    def on_mount(self) -> None:
        self.query_one("#in_name", Input).focus()

    def _submit(self) -> None:
        self.dismiss(
            (
                self.query_one("#in_name", Input).value.strip() or self._in_name,
                self.query_one("#out_name", Input).value.strip() or self._out_name,
            )
        )

    def on_button_pressed(self, event: Button.Pressed) -> None:
        if event.button.id == "save":
            self._submit()
        else:
            self.dismiss(None)

    def on_input_submitted(self, event: Input.Submitted) -> None:
        self._submit()

    def action_cancel(self) -> None:
        self.dismiss(None)


class TesseraApp(App):
    TITLE = "tessera"
    SUB_TITLE = "TESmart HDMI matrix control"

    CSS = """
    Screen { layout: vertical; }
    #status { height: 1; padding: 0 1; background: $panel; color: $text; }
    #grid-wrap { height: auto; padding: 1 1 0 1; }
    DataTable { height: auto; }
    #legend { height: auto; padding: 1; color: $text-muted; }
    #toast { height: 1; padding: 0 1; color: $success; }

    LabelEditScreen { align: center middle; }
    #dialog {
        width: 50; height: auto; padding: 1 2;
        background: $surface; border: thick $primary;
    }
    #dialog-title { text-style: bold; padding-bottom: 1; }
    #dialog-buttons { grid-size: 2; grid-gutter: 1; padding-top: 1; height: auto; }
    """

    def __init__(self, cfg: config.Config):
        super().__init__()
        self.cfg = cfg
        self.client = MatrixClient(cfg.host, cfg.port)
        self.routes: dict[int, int] = {}
        self._pending: str | None = None  # 'save' or 'recall' awaiting a digit
        self._buzzer_on = False
        self._toast = ""

    # -- composition --------------------------------------------------------

    def compose(self) -> ComposeResult:
        yield Static(id="status")
        with Vertical(id="grid-wrap"):
            yield DataTable(id="grid", zebra_stripes=False, cursor_type="cell")
        yield Static(id="toast")
        yield Static(self._legend_text(), id="legend")

    def on_mount(self) -> None:
        table = self.query_one("#grid", DataTable)
        self._build_table(table)
        table.focus()
        self._render_status()
        self.set_interval(self.cfg.poll_interval, self.refresh_status)
        self.call_later(self.refresh_status)

    # -- table --------------------------------------------------------------

    def _build_table(self, table: DataTable) -> None:
        """(Re)build columns/rows from current labels. Used at start and on relabel."""
        table.clear(columns=True)
        for i in range(1, self.cfg.num_inputs + 1):
            table.add_column(self.cfg.input_label(i), key=f"in{i}", width=max(8, len(self.cfg.input_label(i)) + 2))
        for o in range(1, self.cfg.num_outputs + 1):
            cells = [self._cell(o, i) for i in range(1, self.cfg.num_inputs + 1)]
            table.add_row(*cells, label=Text(self.cfg.output_label(o)), key=f"out{o}")

    def _cell(self, out: int, inp: int) -> Text:
        if self.routes.get(out) == inp:
            return Text(DOT, style="bold green", justify="center")
        return Text("·", style="grey37", justify="center")

    def _paint_routes(self) -> None:
        table = self.query_one("#grid", DataTable)
        for o in range(1, self.cfg.num_outputs + 1):
            for i in range(1, self.cfg.num_inputs + 1):
                table.update_cell_at(Coordinate(o - 1, i - 1), self._cell(o, i))

    # -- status / legend ----------------------------------------------------

    def _render_status(self) -> None:
        dot = Text(DOT + " ", style="bold green" if self.client.connected else "bold red")
        state = "connected" if self.client.connected else "disconnected"
        bar = Text.assemble(
            dot,
            (f"{self.cfg.host}:{self.cfg.port}", "bold"),
            (f"  {state}", "green" if self.client.connected else "red"),
            ("   buzzer ", "dim"),
            ("on" if self._buzzer_on else "off", "yellow" if self._buzzer_on else "dim"),
            (f"   tessera v{__version__}", "dim"),
        )
        self.query_one("#status", Static).update(bar)

    def _legend_text(self) -> Text:
        return Text.from_markup(
            "[b]↑↓←→[/] move   [b]enter[/] route   [b]1-4[/] all→input   [b]m[/] mirror   "
            "[b]s+1-8[/] save preset   [b]r+1-8[/] recall preset   [b]b[/] buzzer   "
            "[b]e[/] edit labels   [b]R[/] refresh   [b]q[/] quit"
        )

    def _toast_msg(self, msg: str) -> None:
        self.query_one("#toast", Static).update(Text(msg, style="cyan"))

    # -- polling ------------------------------------------------------------

    async def refresh_status(self) -> None:
        result = await self.client.read_status()
        if result is not None:
            self.routes = result
            self._paint_routes()
        self._render_status()

    # -- key handling -------------------------------------------------------

    async def on_data_table_cell_selected(self, event: DataTable.CellSelected) -> None:
        # Enter on a cell -> route that input (column) to that output (row).
        out = event.coordinate.row + 1
        inp = event.coordinate.column + 1
        if await self.client.route(inp, out):
            self._toast_msg(f"routed {self.cfg.input_label(inp)} → {self.cfg.output_label(out)}")
        await self.refresh_status()

    async def on_key(self, event: events.Key) -> None:
        key = event.key

        # Two-key preset sequences: 's'/'r' then a digit.
        if self._pending and key.isdigit() and 1 <= int(key) <= 8:
            slot = int(key)
            if self._pending == "save":
                await self.client.save_preset(slot)
                self._toast_msg(f"saved current routing → preset {slot}")
            else:
                await self.client.recall_preset(slot)
                self._toast_msg(f"recalled preset {slot}")
            self._pending = None
            await self.refresh_status()
            event.stop()
            return

        if key == "escape" and self._pending:
            self._pending = None
            self._toast_msg("cancelled")
            event.stop()
            return

        if key == "s":
            self._pending = "save"
            self._toast_msg("save to preset slot? press 1-8 (esc cancels)")
            event.stop()
        elif key == "r":
            self._pending = "recall"
            self._toast_msg("recall preset slot? press 1-8 (esc cancels)")
            event.stop()
        elif key.isdigit() and 1 <= int(key) <= self.cfg.num_inputs:
            inp = int(key)
            await self.client.all_to(inp)
            self._toast_msg(f"all outputs → {self.cfg.input_label(inp)}")
            await self.refresh_status()
            event.stop()
        elif key == "m":
            await self.client.mirror()
            self._toast_msg("mirrored each output to its same-numbered input")
            await self.refresh_status()
            event.stop()
        elif key == "b":
            self._buzzer_on = not self._buzzer_on
            await self.client.buzzer(self._buzzer_on)
            self._toast_msg(f"buzzer {'on' if self._buzzer_on else 'off'}")
            self._render_status()
            event.stop()
        elif key == "e":
            self._edit_labels()
            event.stop()
        elif key == "R":
            await self.refresh_status()
            self._toast_msg("refreshed")
            event.stop()
        elif key == "q":
            await self.client.close()
            self.exit()

    def _edit_labels(self) -> None:
        coord = self.query_one("#grid", DataTable).cursor_coordinate
        out = coord.row + 1
        inp = coord.column + 1

        def _apply(result: tuple[str, str] | None) -> None:
            if result is None:
                return
            in_name, out_name = result
            self.cfg.inputs[inp - 1] = in_name
            self.cfg.outputs[out - 1] = out_name
            config.save(self.cfg)
            self._build_table(self.query_one("#grid", DataTable))
            self._toast_msg("labels saved")

        self.push_screen(
            LabelEditScreen(inp, self.cfg.input_label(inp), out, self.cfg.output_label(out)),
            _apply,
        )


def main() -> None:
    parser = argparse.ArgumentParser(prog="tessera", description="TESmart HDMI matrix TUI controller")
    parser.add_argument("--host", help="override matrix host (default from config)")
    parser.add_argument("--port", type=int, help="override matrix port (default from config)")
    parser.add_argument("--version", action="version", version=f"tessera {__version__}")
    args = parser.parse_args()

    cfg = config.load()
    if args.host:
        cfg.host = args.host
    if args.port:
        cfg.port = args.port

    TesseraApp(cfg).run()


if __name__ == "__main__":
    main()
