"""User config for tessera, persisted at ``~/.config/tessera/config.toml``.

Holds the matrix endpoint, poll interval, and the custom input/output labels
(the device protocol has no naming, so labels live here). Read with stdlib
``tomllib``; written with a tiny hand-rolled serializer to avoid an extra dep.
"""

from __future__ import annotations

import os
import tomllib
from dataclasses import dataclass, field
from pathlib import Path

CONFIG_DIR = Path(os.environ.get("XDG_CONFIG_HOME", Path.home() / ".config")) / "tessera"
CONFIG_PATH = CONFIG_DIR / "config.toml"

DEFAULT_HOST = "10.10.0.1"
DEFAULT_PORT = 5000
DEFAULT_POLL = 1.0


@dataclass
class Config:
    host: str = DEFAULT_HOST
    port: int = DEFAULT_PORT
    poll_interval: float = DEFAULT_POLL
    inputs: list[str] = field(default_factory=lambda: [f"Input {i}" for i in range(1, 5)])
    outputs: list[str] = field(default_factory=lambda: [f"Output {i}" for i in range(1, 5)])

    @property
    def num_inputs(self) -> int:
        return len(self.inputs)

    @property
    def num_outputs(self) -> int:
        return len(self.outputs)

    def input_label(self, idx: int) -> str:
        """1-based input label, falling back to a generic name."""
        return self.inputs[idx - 1] if 1 <= idx <= len(self.inputs) else f"Input {idx}"

    def output_label(self, idx: int) -> str:
        return self.outputs[idx - 1] if 1 <= idx <= len(self.outputs) else f"Output {idx}"


def _quote(s: str) -> str:
    return '"' + s.replace("\\", "\\\\").replace('"', '\\"') + '"'


def _toml_list(items: list[str]) -> str:
    return "[" + ", ".join(_quote(i) for i in items) + "]"


def _serialize(cfg: Config) -> str:
    return (
        "# tessera configuration\n"
        f"host = {_quote(cfg.host)}\n"
        f"port = {cfg.port}\n"
        f"poll_interval = {cfg.poll_interval}\n"
        f"inputs = {_toml_list(cfg.inputs)}\n"
        f"outputs = {_toml_list(cfg.outputs)}\n"
    )


def load() -> Config:
    """Load config, writing defaults on first run."""
    if not CONFIG_PATH.exists():
        cfg = Config()
        save(cfg)
        return cfg
    with CONFIG_PATH.open("rb") as fh:
        data = tomllib.load(fh)
    cfg = Config()
    cfg.host = str(data.get("host", cfg.host))
    cfg.port = int(data.get("port", cfg.port))
    cfg.poll_interval = float(data.get("poll_interval", cfg.poll_interval))
    if isinstance(data.get("inputs"), list) and data["inputs"]:
        cfg.inputs = [str(x) for x in data["inputs"]]
    if isinstance(data.get("outputs"), list) and data["outputs"]:
        cfg.outputs = [str(x) for x in data["outputs"]]
    return cfg


def save(cfg: Config) -> None:
    CONFIG_DIR.mkdir(parents=True, exist_ok=True)
    CONFIG_PATH.write_text(_serialize(cfg), encoding="utf-8")
