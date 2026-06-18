"""Async TCP client for the TESmart HDMI matrix switcher.

Protocol (ASCII, header ``MT00``, suffix ``NT``), spoken over a raw TCP socket.
Confirmed against the open-source Bitfocus Companion module for these units:

    MT00SWxxyyNT   route input xx -> output yy
    MT00SWxx00NT   route input xx -> ALL outputs
    MT00SW0000NT   mirror (out1<-in1 .. outN<-inN)
    MT00RD0000NT   read status   -> "LINK:O1I1;O2I2;...;END"
    MT00SV00ppNT   save current routing to preset pp (01..08)
    MT00RD01ppNT   recall preset pp (01..08)
    MT00BZEN00NT   buzzer on
    MT00BZEN01NT   buzzer off

Two device quirks this client hides from the UI:

* The unit dribbles its status reply across several TCP segments and is slow to
  send ``END``; a single ``recv`` often returns nothing useful. We accumulate
  chunks until the buffer contains ``END`` (or a timeout elapses).
* Switch / save / buzzer commands produce no reply — we fire them and let the
  next status poll confirm the new state.
"""

from __future__ import annotations

import asyncio
import re

# Each command is terminated with CRLF on the wire.
_TERM = "\r\n"
# Parses tokens like "O1I3" out of a "LINK:O1I3;O2I2;...;END" reply.
_TOKEN_RE = re.compile(r"O(\d+)I(\d+)")


def fmt_switch(inp: int, out: int) -> str:
    """``MT00SWxxyyNT`` — route ``inp`` to ``out`` (out=0 means all outputs)."""
    return f"MT00SW{inp:02d}{out:02d}NT"


def fmt_mirror() -> str:
    return "MT00SW0000NT"


def fmt_read_status() -> str:
    return "MT00RD0000NT"


def fmt_save_preset(preset: int) -> str:
    return f"MT00SV00{preset:02d}NT"


def fmt_recall_preset(preset: int) -> str:
    return f"MT00RD01{preset:02d}NT"


def fmt_buzzer(on: bool) -> str:
    # Per the protocol: BZEN00 = on, BZEN01 = off.
    return "MT00BZEN00NT" if on else "MT00BZEN01NT"


def parse_status(text: str) -> dict[int, int]:
    """Parse a ``LINK:O1I1;...;END`` reply into ``{output: input}``.

    Returns ``{}`` if no ``LINK:`` line is present.
    """
    routes: dict[int, int] = {}
    for line in text.replace("\r", "\n").split("\n"):
        if "LINK:" in line and "END" in line:
            for out_s, in_s in _TOKEN_RE.findall(line):
                routes[int(out_s)] = int(in_s)
    return routes


class MatrixClient:
    """Maintains one persistent TCP connection, serialized with a lock.

    Every public coroutine acquires ``_lock`` so the periodic status poll can
    never interleave on the socket with a user-triggered command.
    """

    def __init__(self, host: str, port: int, *, read_timeout: float = 2.0):
        self.host = host
        self.port = port
        self.read_timeout = read_timeout
        self.connected = False
        self.last_error: str | None = None
        self._reader: asyncio.StreamReader | None = None
        self._writer: asyncio.StreamWriter | None = None
        self._lock = asyncio.Lock()

    async def _ensure_connection(self) -> None:
        if self._writer is not None and not self._writer.is_closing():
            return
        self._reader, self._writer = await asyncio.wait_for(
            asyncio.open_connection(self.host, self.port), timeout=self.read_timeout
        )
        self.connected = True
        self.last_error = None

    async def _drop(self, err: Exception | None) -> None:
        self.connected = False
        self.last_error = str(err) if err else None
        if self._writer is not None:
            try:
                self._writer.close()
            except Exception:
                pass
        self._reader = None
        self._writer = None

    async def _write(self, cmd: str) -> None:
        await self._ensure_connection()
        assert self._writer is not None
        self._writer.write((cmd + _TERM).encode("ascii"))
        await self._writer.drain()

    async def _read_until_end(self) -> str:
        """Accumulate chunks until ``END`` appears or the read times out."""
        assert self._reader is not None
        buf = b""
        loop = asyncio.get_event_loop()
        deadline = loop.time() + self.read_timeout
        while b"END" not in buf:
            remaining = deadline - loop.time()
            if remaining <= 0:
                break
            try:
                chunk = await asyncio.wait_for(self._reader.read(128), timeout=remaining)
            except asyncio.TimeoutError:
                break
            if not chunk:  # connection closed by peer
                break
            buf += chunk
        return buf.decode("ascii", errors="replace")

    # -- public API ---------------------------------------------------------

    async def read_status(self) -> dict[int, int] | None:
        """Poll current routing. Returns ``{output: input}`` or ``None`` on error."""
        async with self._lock:
            try:
                await self._write(fmt_read_status())
                reply = await self._read_until_end()
                self.connected = True
                self.last_error = None
                return parse_status(reply)
            except Exception as err:  # noqa: BLE001 - surface as disconnect
                await self._drop(err)
                return None

    async def _fire(self, cmd: str) -> bool:
        """Send a no-reply command. Returns True on success."""
        async with self._lock:
            try:
                await self._write(cmd)
                return True
            except Exception as err:  # noqa: BLE001
                await self._drop(err)
                return False

    async def route(self, inp: int, out: int) -> bool:
        return await self._fire(fmt_switch(inp, out))

    async def all_to(self, inp: int) -> bool:
        return await self._fire(fmt_switch(inp, 0))

    async def mirror(self) -> bool:
        return await self._fire(fmt_mirror())

    async def save_preset(self, preset: int) -> bool:
        return await self._fire(fmt_save_preset(preset))

    async def recall_preset(self, preset: int) -> bool:
        return await self._fire(fmt_recall_preset(preset))

    async def buzzer(self, on: bool) -> bool:
        return await self._fire(fmt_buzzer(on))

    async def close(self) -> None:
        async with self._lock:
            await self._drop(None)
