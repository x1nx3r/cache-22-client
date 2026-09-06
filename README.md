# cache-22-client

Plays your own PS2 rips over the network. Streams ISO blocks on demand
through a FUSE filesystem, boots them in stock PCSX2, and syncs your memory
cards to the server when you're done. No emulator fork, no BIOS shipped,
no 4GB downloads to start playing.

The trick: the emulator thinks it's reading a local disc. It's reading HTTP
Range requests wearing a trenchcoat.

## How it came to be

The server existed first and had exactly one client: `curl`. Somebody had
to turn byte ranges into a bootable game, and the options were (a) fork
PCSX2 and teach it about the network, or (b) lie to stock PCSX2 with a
filesystem. Forking an emulator is how you inherit a decade of someone
else's bugs, so: FUSE. The client exposes a sparse ISO file, the emulator
reads it like any disc image, and every cache miss becomes a Range request
with the read blocked until bytes arrive. That last part matters — early
versions returned zeros on a miss and PCSX2 booted into a black void,
which was briefly mistaken for a BIOS problem and cost an evening.

The GUI exists because `mount /mnt/foo && pcsx2 --datapath ...` is not a
user experience. The CLI still does everything, because terminals are
forever.

## The theory

- **Sparse store + block bitmap.** Each game is a sparse file plus a
  progress sidecar tracking 128KB blocks. `MissingRanges` turns any wanted
  interval into exactly the holes, so nothing is ever re-downloaded.
- **Exact-range fetch.** No fixed chunk abstraction — reads are the byte
  ranges the emulator asked for, singleflight-deduplicated, with
  sequential-only readahead (8 blocks; random seeks prefetch nothing).
- **Adaptive tiers.** A link probe (RTT pings + timed 8MB range) classifies
  the connection and picks fetch size × prefetch depth: 128KB×8 on LAN,
  up to 2MB×64 on slow links. Slow links also preload a bigger head so
  boot doesn't stutter. The probe result drives everything and the UI
  shows the active tier, because invisible adaptivity reads as randomness.
- **FUSE blocks on miss.** A read for uncached bytes stalls until the fetch
  lands. No zeros, no black voids, no lying to the emulator.
- **Stock PCSX2, private datapath.** The AppImage sidecar downloads itself,
  BIOS installs from your own dump into the app's data dir, OOBE is
  pre-seeded away. The client wraps the emulator; it doesn't patch it.
- **Cloud saves are three-way sync.** Per-game, both slots, keyed
  `(user, serial, slot)`. Pre-launch pull-if-newer, post-exit push-if-changed
  (the client tracks the emulator process, so it knows exactly when the
  cards are cold), last-synced hashes client-side, last-write-wins with a
  backup kept on conflict. Offline just plays local and says so.

## In practice

```sh
go build -o bin/cache22 ./cmd/cache22
bin/cache22 login
bin/cache22 games
bin/cache22 play "Ace Combat Zero - The Belkan War (USA)"
```

Full CLI: `login logout games fetch sync launch play mount clean profile`.

GUI dev (needs a display, obviously):

```sh
cd gui
wails3 generate bindings   # after touching Go service methods
make dev
```

`gui/frontend/src` is React + Material Web with `@lit/react` wrappers.
Heads-up from the trenches: in Material Web v2.5 the `headline`
*attribute* on list items and select options is dead — headlines come from
`<div slot="headline">` children. The type definitions still show the old
example. Don't be like me; read the `.js`.

Config via env: `CACHE22_SERVER`, `CACHE22_DATA_DIR`, `CACHE22_TOKEN`,
`CACHE22_PCSX2_VERSION`, `CACHE22_PROFILE=1`.

### Controller mapping without launching a game

Keyboard capture in Settings → Controller writes straight to PCSX2's ini.
Gamepads capture off `/dev/input/js*` in pure Go (no SDL dependency) and
emit raw `SDL-0/JoyButton<N>` tokens — verified against PCSX2's actual
binding parser, not vibes. Gamepad capture needs `input` group membership;
the error will tell you so.

## Roadmap

- [ ] **Live input test.** Press something, watch the binding flash. Keyboard
  side is a keydown reverse-map (nearly free); gamepad side needs a poll
  loop while the tab is open.
- [ ] **Save publishing UI.** Server holds the `public` flag already; the
  client needs browse/copy flows so your perfect 100% file can ruin — er,
  help — a friend.
- [ ] **Mac/Windows clients.** No FUSE there, so: sparse-file boot, where
  the client prefetches the whole image to a real file and hands PCSX2 a
  normal path. Less magical, fully functional.
- [ ] **Rooms (P3).** WireGuard + VXLAN LAN rooms for the multiplayer far
  future. Client side: tunnel lifecycle, room presence, latency display.
- [ ] **Smarter preload.** Per-game learned boot sets from the profiler's
  block heatmaps instead of fixed head sizes — the data already exists in
  `profile.json`, nobody reads it yet.
- [ ] ** played-state honesty.** The player bar tracks process + download
  state today; elapsed playtime and per-game history would make the library
  feel alive.

## Test

```sh
go test ./...            # client root
cd gui && go test .      # desktop services
```
