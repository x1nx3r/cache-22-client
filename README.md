# cache-22-client

Plays your own PS2 rips over the network. Streams ISO blocks on demand
through a FUSE filesystem, boots them in stock PCSX2, and syncs your memory
cards to the server when you're done. No emulator fork, no BIOS shipped,
no 4GB downloads to start playing.

The trick: the emulator thinks it's reading a local disc. It's reading HTTP
Range requests wearing a trenchcoat.

## Layout

- `cmd/cache22` — CLI: `login logout games fetch sync launch play mount clean profile`
- `gui/` — Wails v3 desktop app (React + Material Web, Catppuccin Macchiato, naturally)
- `internal/` — API client, sparse store, FUSE reader, syncer, profiler, PCSX2 wrapper, joystick capture

## Run

```sh
go build -o bin/cache22 ./cmd/cache22
bin/cache22 login
bin/cache22 games
bin/cache22 play "Ace Combat Zero - The Belkan War (USA)"
```

GUI dev (needs a display, obviously):

```sh
cd gui
wails3 generate bindings   # after touching Go service methods
make dev
```

Frontend notes: `gui/frontend/src` is React + Material Web with
`@lit/react` wrappers. Heads-up from the trenches: in Material Web v2.5 the
`headline` *attribute* on list items and select options is dead — headlines
come from `<div slot="headline">` children. The type definitions still show
the old example. Don't be like me; read the `.js`.

## Config (env)

`CACHE22_SERVER`, `CACHE22_DATA_DIR`, `CACHE22_TOKEN`,
`CACHE22_PCSX2_VERSION`, `CACHE22_PROFILE=1`.

## The good stuff

- **Controller mapping without launching a game.** Keyboard capture in the
  app, plus gamepad capture straight off `/dev/input/js*` (pure Go, no SDL
  dependency). Bindings land in PCSX2's own ini format, including the raw
  `SDL-0/JoyButton<N>` tokens — verified against PCSX2's actual parser, not
  vibes. Gamepad needs `input` group membership; the error will tell you.
- **Cloud saves.** Per-game, both slots, synced both ways: pulled before
  launch when the server is newer, pushed after the emulator exits when
  yours changed. Conflicts resolve last-write-wins with a backup kept.
  Manage them in Settings → Saves. Offline? It plays on local cards and
  tells you about it.
- **Link-adaptive fetching.** The client probes RTT/throughput and picks a
  fetch tier (128KB×8 up to 2MB×64). Slow links preload more; fast links
  barely preload at all.

## Test

```sh
go test ./...            # client root
cd gui && go test .      # desktop services
```
