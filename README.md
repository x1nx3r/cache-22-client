# Cache-22 Client

Go client for Cache-22: streams private PS2 ISOs from a Cache-22 server
through a FUSE filesystem and boots them in stock PCSX2 (AppImage sidecar,
private datapath, OOBE skipped). No emulator fork, no BIOS shipped.

## Layout

- `cmd/cache22` — CLI (`login`, `games`, `fetch`, `sync`, `play`, `mount`, `clean`, `profile`)
- `gui/` — Wails v3 desktop app (Material Web UI)
- `internal/` — API client, sparse store, FUSE reader, syncer, profiler, PCSX2 wrapper

## Run

```sh
go build -o bin/cache22 ./cmd/cache22
bin/cache22 games
```

GUI dev:

```sh
cd gui
wails3 generate bindings   # after Go service changes
make dev                   # needs a display
```

Frontend notes: `gui/frontend/src` is React + Material Web. After changing
`.jsx`/CSS run nothing special for dev (HMR); `wails3 build` bundles it.

## Config (env)

`CACHE22_SERVER`, `CACHE22_DATA_DIR`, `CACHE22_TOKEN`,
`CACHE22_PCSX2_VERSION`, `CACHE22_PROFILE=1`.
