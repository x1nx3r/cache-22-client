package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/x1nx3r/cache-22-client/internal/api"
	"github.com/x1nx3r/cache-22-client/internal/config"
	"github.com/x1nx3r/cache-22-client/internal/fusefs"
	"github.com/x1nx3r/cache-22-client/internal/pcsx2"
	"github.com/x1nx3r/cache-22-client/internal/profile"
	"github.com/x1nx3r/cache-22-client/internal/save"
	"github.com/x1nx3r/cache-22-client/internal/store"
	"github.com/x1nx3r/cache-22-client/internal/syncer"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	cfg := config.New()

	var err error
	switch os.Args[1] {
	case "login":
		err = cmdLogin(cfg, os.Args[2:])
	case "logout":
		err = cmdLogout(cfg)
	case "games":
		err = cmdGames(mustClient(cfg))
	case "fetch":
		err = cmdFetch(mustClient(cfg), cfg, os.Args[2:])
	case "sync":
		err = cmdSync(mustClient(cfg), cfg, os.Args[2:])
	case "launch":
		err = cmdLaunch(mustClient(cfg), cfg, os.Args[2:])
	case "play":
		err = cmdPlay(mustClient(cfg), cfg, os.Args[2:])
	case "mount":
		err = cmdMount(mustClient(cfg), cfg, os.Args[2:])
	case "clean":
		err = cmdClean(cfg, os.Args[2:])
	case "profile":
		err = cmdProfile(mustClient(cfg), cfg, os.Args[2:])
	default:
		usage()
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Println("usage: cache22 <login|logout|games|fetch|sync|launch|play|mount|clean|profile> ...")
}

func cmdGames(client *api.Client) error {
	games, err := api.ListGames(client)
	if err != nil {
		return err
	}
	for _, g := range games {
		fmt.Printf("%s\t%s\t%d\n", g.Serial, g.Title, g.SizeBytes)
	}
	return nil
}

func openStore(cfg config.Config, m api.Manifest) (*store.Sparse, error) {
	return store.Open(filepath.Join(cfg.DataDir, "games"), m.Serial, m.SizeBytes, 0)
}

func bootRanges(m api.Manifest) [][2]int64 {
	var out [][2]int64
	for _, r := range m.BootRanges {
		if r[0] >= m.SizeBytes {
			continue
		}
		end := r[1]
		if end > m.SizeBytes {
			end = m.SizeBytes
		}
		out = append(out, [2]int64{r[0], end})
	}
	return out
}

func missingRanges(s *store.Sparse, ranges [][2]int64) [][2]int64 {
	var out [][2]int64
	for _, r := range ranges {
		out = append(out, s.MissingRanges(r[0], r[1]-r[0])...)
	}
	return out
}

func cmdFetch(client *api.Client, cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("fetch", flag.ContinueOnError)
	workers := fs.Int("workers", 4, "parallel chunk downloads")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: cache22 fetch <serial>")
	}
	m, err := api.GetManifest(client, fs.Arg(0))
	if err != nil {
		return err
	}
	if !m.Supported {
		return fmt.Errorf("game is not a raw iso on the server")
	}
	s, err := openStore(cfg, m)
	if err != nil {
		return err
	}
	defer s.Close()
	ranges := bootRanges(m)
	want := missingRanges(s, ranges)
	fmt.Printf("boot set: %d ranges, %d missing\n", len(ranges), len(want))
	if err := syncer.FillRanges(client, m.Serial, s, want, *workers); err != nil {
		return err
	}
	fmt.Printf("ready: %s (%s of %s)\n", s.Path, fmtBytes(s.DoneBytes()), fmtBytes(m.SizeBytes))
	return nil
}

func cmdSync(client *api.Client, cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	workers := fs.Int("workers", 4, "parallel chunk downloads")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: cache22 sync <serial>")
	}
	m, err := api.GetManifest(client, fs.Arg(0))
	if err != nil {
		return err
	}
	s, err := openStore(cfg, m)
	if err != nil {
		return err
	}
	defer s.Close()
	segs := syncer.WholeImage(m.SizeBytes, 0)
	want := missingRanges(s, segs)
	fmt.Printf("full: %d MB, %d ranges missing\n", m.SizeBytes>>20, len(want))
	if err := syncer.FillRanges(client, m.Serial, s, want, *workers); err != nil {
		return err
	}
	fmt.Printf("done: %s (%s of %s)\n", s.Path, fmtBytes(s.DoneBytes()), fmtBytes(m.SizeBytes))
	return nil
}

func cmdLaunch(client *api.Client, cfg config.Config, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: cache22 launch <serial>")
	}
	m, err := api.GetManifest(client, args[0])
	if err != nil {
		return err
	}
	s, err := openStore(cfg, m)
	if err != nil {
		return err
	}
	s.Close()
	if s.DoneBytes() == 0 {
		return fmt.Errorf("nothing downloaded, run fetch first")
	}
	if missing := m.SizeBytes - s.DoneBytes(); missing > 0 {
		fmt.Printf("warning: %s missing, game may stall\n", fmtBytes(missing))
	}
	if !pcsx2.HasBIOS(cfg.DataDir) {
		fmt.Println("warning: no BIOS found, dump yours from a real PS2")
	}
	reportSaves(save.Prepare(client, cfg.DataDir, m.Serial))
	app, err := pcsx2.Ensure(filepath.Join(cfg.DataDir, "emulator"), cfg.PCSX2Version)
	if err != nil {
		return err
	}
	if err := pcsx2.Prepare(cfg.DataDir); err != nil {
		return err
	}
	fmt.Printf("launching %s on %s\n", m.Title, s.Path)
	if err := pcsx2.Launch(app, cfg.DataDir, s.Path); err != nil {
		return err
	}
	reportSaves(save.Push(client, cfg.DataDir, m.Serial))
	return nil
}

func cmdPlay(client *api.Client, cfg config.Config, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: cache22 play <serial>")
	}
	m, err := api.GetManifest(client, args[0])
	if err != nil {
		return err
	}
	if !m.Supported {
		return fmt.Errorf("game is not a raw iso on the server")
	}
	if !pcsx2.HasBIOS(cfg.DataDir) {
		return fmt.Errorf("no BIOS found, dump yours from a real PS2")
	}
	s, err := openStore(cfg, m)
	if err != nil {
		return err
	}
	defer s.Close()
	reportSaves(save.Prepare(client, cfg.DataDir, m.Serial))
	link, err := client.Probe(m.Serial)
	if err != nil {
		fmt.Printf("probe failed (%v), assuming LAN\n", err)
		link = api.ProbeResult{Class: "lan"}
	}
	fmt.Printf("link: %s · %.0fms · %.0fMbps\n", link.Class, link.RTTMs, link.Mbps)
	if link.Mbps > 0 && link.Mbps < 10 {
		fmt.Println("warning: slow link, preload more before playing")
	}
	fetchCfg := fusefs.ConfigForRTT(link.RTTMs)
	preload := syncer.PreloadRanges(m.SizeBytes, bootRanges(m), link.Class)
	if want := missingRanges(s, preload); len(want) > 0 {
		fmt.Printf("preloading: %d ranges\n", len(want))
		if err := syncer.FillRanges(client, m.Serial, s, want, 4); err != nil {
			return err
		}
	}
	mnt := filepath.Join(cfg.DataDir, "mnt", mntName(m.Serial))
	if err := os.MkdirAll(mnt, 0o755); err != nil {
		return err
	}
	reader, stopTrace := tracedReader(cfg, fetchCfg, client, s, m)
	defer stopTrace()
	server, err := fusefs.Mount(mnt, reader, m.SizeBytes)
	if err != nil {
		return fmt.Errorf("mount: %w", err)
	}
	defer server.Unmount()
	iso := filepath.Join(mnt, "game.iso")
	fmt.Printf("mounted %s, launching\n", iso)
	app, err := pcsx2.Ensure(filepath.Join(cfg.DataDir, "emulator"), cfg.PCSX2Version)
	if err != nil {
		return err
	}
	if err := pcsx2.Prepare(cfg.DataDir); err != nil {
		return err
	}
	if err := pcsx2.Launch(app, cfg.DataDir, iso); err != nil {
		return err
	}
	reportSaves(save.Push(client, cfg.DataDir, m.Serial))
	return nil
}

func reportSaves(rep save.Report) {
	for _, n := range rep.Notes {
		fmt.Printf("saves: %s\n", n.Msg)
	}
}

func mntName(serial string) string {
	out := ""
	for _, r := range serial {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			out += string(r)
		} else {
			out += "_"
		}
	}
	if out == "" {
		out = "game"
	}
	return out
}

func cmdMount(client *api.Client, cfg config.Config, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: cache22 mount <serial> [mountpoint]")
	}
	m, err := api.GetManifest(client, args[0])
	if err != nil {
		return err
	}
	mnt := filepath.Join(cfg.DataDir, "mnt", mntName(m.Serial))
	if len(args) > 1 {
		mnt = args[1]
	}
	if err := os.MkdirAll(mnt, 0o755); err != nil {
		return err
	}
	s, err := openStore(cfg, m)
	if err != nil {
		return err
	}
	defer s.Close()
	reader, stopTrace := tracedReader(cfg, fusefs.LANConfig(), client, s, m)
	defer stopTrace()
	server, err := fusefs.Mount(mnt, reader, m.SizeBytes)
	if err != nil {
		return fmt.Errorf("mount: %w", err)
	}
	defer server.Unmount()
	fmt.Printf("mounted %s/game.iso, Ctrl-C to unmount\n", mnt)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	return nil
}

func cmdClean(cfg config.Config, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: cache22 clean <serial>|all")
	}
	gamesDir := filepath.Join(cfg.DataDir, "games")
	mntDir := filepath.Join(cfg.DataDir, "mnt")
	if args[0] == "all" {
		if entries, err := os.ReadDir(mntDir); err == nil {
			for _, e := range entries {
				unmount(filepath.Join(mntDir, e.Name()))
			}
		}
		if err := os.RemoveAll(gamesDir); err != nil {
			return err
		}
		if err := os.RemoveAll(mntDir); err != nil {
			return err
		}
		fmt.Println("cache cleared")
		return nil
	}
	unmount(filepath.Join(mntDir, mntName(args[0])))
	iso, prog := store.Paths(gamesDir, args[0])
	removed := false
	for _, p := range []string{iso, prog} {
		if err := os.Remove(p); err == nil {
			removed = true
		}
	}
	if !removed {
		return fmt.Errorf("nothing cached for %q", args[0])
	}
	fmt.Printf("cleared %q\n", args[0])
	return nil
}

func unmount(dir string) {
	if _, err := os.Stat(dir); err != nil {
		return
	}
	if err := exec.Command("fusermount3", "-u", dir).Run(); err != nil {
		_ = exec.Command("umount", "-l", dir).Run()
	}
}

func maybeTrace(cfg config.Config, serial string) (*profile.Tracer, error) {
	if !cfg.Profiling {
		return nil, nil
	}
	return profile.Start(cfg.DataDir, serial)
}

func cmdProfile(client *api.Client, cfg config.Config, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: cache22 profile <serial> [hot-n]")
	}
	m, err := api.GetManifest(client, args[0])
	if err != nil {
		return err
	}
	hm, err := profile.Aggregate(cfg.DataDir, m.Serial, store.DefaultBlockLen)
	if err != nil {
		return fmt.Errorf("no trace yet, play the game first: %w", err)
	}
	if err := profile.Save(cfg.DataDir, m.Serial, hm); err != nil {
		return err
	}
	n := 64
	if len(args) > 1 {
		fmt.Sscanf(args[1], "%d", &n)
	}
	hot := profile.HotSet(hm, n)
	var touched int64
	for _, c := range hm.Chunks {
		touched += c.Count
	}
	fmt.Printf("%s: %d sessions, %d reads, %d unique chunks, hot set %d:\n", hm.Serial, hm.Sessions, touched, len(hm.Chunks), len(hot))
	fmt.Printf("%v\n", hot)
	return nil
}

func mustClient(cfg config.Config) *api.Client {
	c, err := serverClient(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	return c
}

func fmtBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

func tracedReader(cfg config.Config, fetchCfg fusefs.FetchConfig, client *api.Client, s *store.Sparse, m api.Manifest) (*fusefs.Reader, func()) {
	r := fusefs.NewReader(client, s, m.SizeBytes, m.Serial, fetchCfg)
	if !cfg.Profiling {
		return r, func() {}
	}
	tr, err := profile.Start(cfg.DataDir, m.Serial)
	if err != nil {
		fmt.Fprintln(os.Stderr, "profile off:", err)
		return r, func() {}
	}
	r.Trace(tr)
	return r, func() { tr.Close() }
}

func tokenPath(cfg config.Config) string {
	return filepath.Join(cfg.DataDir, "auth.json")
}

func savedToken(cfg config.Config) string {
	raw, err := os.ReadFile(tokenPath(cfg))
	if err != nil {
		return ""
	}
	var saved struct {
		Server string `json:"server"`
		Token  string `json:"token"`
	}
	if json.Unmarshal(raw, &saved) != nil || saved.Server != cfg.ServerURL {
		return ""
	}
	return saved.Token
}

func serverClient(cfg config.Config) (*api.Client, error) {
	token := cfg.Token
	if token == "" {
		token = savedToken(cfg)
	}
	if token == "" {
		return nil, fmt.Errorf("not logged in, run: cache22 login [--password=...] <username> (or set CACHE22_TOKEN)")
	}
	return api.New(cfg.ServerURL, token), nil
}

func cmdLogin(cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	password := fs.String("password", "", "password (or CACHE22_PASSWORD)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: cache22 login [--password=...] <username>")
	}
	username := fs.Arg(0)
	pw := *password
	if pw == "" {
		pw = os.Getenv("CACHE22_PASSWORD")
	}
	if pw == "" {
		return fmt.Errorf("set --password or CACHE22_PASSWORD")
	}
	token, err := api.Login(cfg.ServerURL, username, pw)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return err
	}
	raw, err := json.Marshal(map[string]string{"server": cfg.ServerURL, "username": username, "token": token})
	if err != nil {
		return err
	}
	if err := os.WriteFile(tokenPath(cfg), raw, 0o600); err != nil {
		return err
	}
	fmt.Printf("logged in as %s\n", username)
	return nil
}

func cmdLogout(cfg config.Config) error {
	if err := os.Remove(tokenPath(cfg)); err != nil && !os.IsNotExist(err) {
		return err
	}
	fmt.Println("logged out")
	return nil
}
