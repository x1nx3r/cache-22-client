package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/x1nx3r/cache-22-client/internal/api"
	"github.com/x1nx3r/cache-22-client/internal/fusefs"
	"github.com/x1nx3r/cache-22-client/internal/joy"
	"github.com/x1nx3r/cache-22-client/internal/pcsx2"
	"github.com/x1nx3r/cache-22-client/internal/profile"
	"github.com/x1nx3r/cache-22-client/internal/save"
	"github.com/x1nx3r/cache-22-client/internal/store"
	"github.com/x1nx3r/cache-22-client/internal/syncer"
)

var ErrNoBIOS = errors.New("no PS2 BIOS found, dump yours from a real PS2 then use Select BIOS file")

type EmulatorService struct {
	servers   *ServerService
	dataDir   string
	version   string
	profiling bool
	pick      func() ([]string, error)

	mu    sync.Mutex
	probe api.ProbeResult

	proc      *exec.Cmd
	activeMnt string
	runSerial string
	runTitle  string
	runSince  time.Time

	stage      string
	stageDone  int64
	stageTotal int64

	saveSeq int64
	saveMsg string
	saveOk  bool
}

type EmuStatus struct {
	Running    bool   `json:"running"`
	Serial     string `json:"serial"`
	Title      string `json:"title"`
	SinceUnix  int64  `json:"sinceUnix"`
	Stage      string `json:"stage"`
	StageDone  int64  `json:"stageDone"`
	StageTotal int64  `json:"stageTotal"`
	SaveSeq    int64  `json:"saveSeq"`
	SaveMsg    string `json:"saveMsg"`
	SaveOk     bool   `json:"saveOk"`
}

type FetchTier struct {
	Probed        bool   `json:"probed"`
	Class         string `json:"class"`
	FetchSize     int64  `json:"fetchSize"`
	PrefetchDepth int64  `json:"prefetchDepth"`
}

func NewEmulatorService(servers *ServerService, dataDir, version string, profiling bool) *EmulatorService {
	return &EmulatorService{servers: servers, dataDir: dataDir, version: version, profiling: profiling}
}

func (e *EmulatorService) SetPicker(pick func() ([]string, error)) {
	e.pick = pick
}

func (e *EmulatorService) BiosOK() bool {
	return pcsx2.HasBIOS(e.dataDir)
}

func (e *EmulatorService) BiosDir() string {
	return pcsx2.BiosDir(e.dataDir)
}

func (e *EmulatorService) ImportBIOS() (int, error) {
	if e.pick == nil {
		return 0, errors.New("file picker not available")
	}
	srcs, err := e.pick()
	if err != nil {
		return 0, err
	}
	if len(srcs) == 0 {
		return 0, nil
	}
	return pcsx2.InstallBIOS(e.dataDir, srcs)
}

func (e *EmulatorService) EnsureEmulator() (string, error) {
	return pcsx2.Ensure(filepath.Join(e.dataDir, "emulator"), e.version)
}

func (e *EmulatorService) Version() string {
	return e.version
}

func (e *EmulatorService) DataDir() string {
	return e.dataDir
}

func (e *EmulatorService) PadButtons() []string {
	return pcsx2.PadButtons
}

func (e *EmulatorService) PadBindings() (map[string]string, error) {
	return pcsx2.ReadPadBindings(e.dataDir)
}

func (e *EmulatorService) SetPadBinding(button, binding string) error {
	e.mu.Lock()
	running := e.proc != nil
	e.mu.Unlock()
	if running {
		return fmt.Errorf("stop the emulator before remapping")
	}
	return pcsx2.SetPadBinding(e.dataDir, button, binding)
}

func (e *EmulatorService) JoyDevices() ([]joy.Device, error) {
	devs, err := joy.List()
	if err != nil {
		return nil, err
	}
	if devs == nil {
		return []joy.Device{}, nil
	}
	return devs, nil
}

func (e *EmulatorService) CaptureJoy(index int) (string, error) {
	e.mu.Lock()
	running := e.proc != nil
	e.mu.Unlock()
	if running {
		return "", fmt.Errorf("stop the emulator before remapping")
	}
	return joy.Capture(index, 12*time.Second)
}

func (e *EmulatorService) setStage(stage string, done, total int64) {
	e.mu.Lock()
	e.stage, e.stageDone, e.stageTotal = stage, done, total
	e.mu.Unlock()
}

func (e *EmulatorService) EmuStatus() (EmuStatus, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	st := EmuStatus{
		Stage: e.stage, StageDone: e.stageDone, StageTotal: e.stageTotal,
		SaveSeq: e.saveSeq, SaveMsg: e.saveMsg, SaveOk: e.saveOk,
	}
	if e.proc == nil {
		return st, nil
	}
	st.Running = true
	st.Serial, st.Title, st.SinceUnix = e.runSerial, e.runTitle, e.runSince.Unix()
	return st, nil
}

func (e *EmulatorService) noteSave(ok bool, msg string) {
	e.mu.Lock()
	e.saveSeq++
	e.saveMsg, e.saveOk = msg, ok
	e.mu.Unlock()
}

func (e *EmulatorService) StopEmulator() error {
	e.mu.Lock()
	proc := e.proc
	e.mu.Unlock()
	if proc == nil {
		return nil
	}
	if proc.Process == nil {
		return nil
	}
	return proc.Process.Kill()
}

func (e *EmulatorService) ProbeNow() (api.ProbeResult, error) {
	active, err := e.servers.ActiveEntry()
	if err != nil {
		return api.ProbeResult{}, err
	}
	c := api.New(active.URL, active.Token)
	games, err := api.ListGames(c)
	if err != nil {
		return api.ProbeResult{}, err
	}
	if len(games) == 0 {
		return api.ProbeResult{}, errors.New("no games on server to probe with")
	}
	link, err := c.Probe(games[0].Serial)
	if err != nil {
		return api.ProbeResult{}, err
	}
	e.mu.Lock()
	e.probe = link
	e.mu.Unlock()
	return link, nil
}

func (e *EmulatorService) FetchTier() (FetchTier, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.probe.Class == "" {
		cfg := fusefs.LANConfig()
		return FetchTier{Probed: false, FetchSize: cfg.FetchSize, PrefetchDepth: cfg.PrefetchDepth}, nil
	}
	cfg := fusefs.ConfigForRTT(e.probe.RTTMs)
	return FetchTier{Probed: true, Class: e.probe.Class, FetchSize: cfg.FetchSize, PrefetchDepth: cfg.PrefetchDepth}, nil
}
func (e *EmulatorService) LastProbe() (*api.ProbeResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.probe.Class == "" {
		return nil, nil
	}
	p := e.probe
	return &p, nil
}

// prepareSaves syncs both slots for a game before launch. It never fails
// hard: offline or undecided states fall back to local cards with a notice.
func (e *EmulatorService) prepareSaves(c *api.Client, serial string) {
	e.setStage("Syncing saves", 0, 0)
	e.reportSave(save.Prepare(c, e.dataDir, serial))
}

// pushSaves uploads changed working cards after the emulator exits.
func (e *EmulatorService) pushSaves(c *api.Client, serial string) {
	e.reportSave(save.Push(c, e.dataDir, serial))
}

// reportSave replays the last sync note through the toast queue, matching
// the previous inline behavior where each notice overwrote the last.
func (e *EmulatorService) reportSave(rep save.Report) {
	if len(rep.Notes) == 0 {
		return
	}
	n := rep.Notes[len(rep.Notes)-1]
	e.noteSave(n.OK, n.Msg)
}

func (e *EmulatorService) Play(serial string) error {
	if !pcsx2.HasBIOS(e.dataDir) {
		return ErrNoBIOS
	}
	e.mu.Lock()
	if e.proc != nil {
		name := e.runTitle
		if name == "" {
			name = e.runSerial
		}
		e.mu.Unlock()
		return fmt.Errorf("already playing %s, stop it first", name)
	}
	e.mu.Unlock()
	active, err := e.servers.ActiveEntry()
	if err != nil {
		e.setStage("Failed: no server", 0, 0)
		return err
	}
	c := api.New(active.URL, active.Token)
	e.setStage("Fetching game info", 0, 0)
	m, err := api.GetManifest(c, serial)
	if err != nil {
		e.setStage("Failed: no manifest", 0, 0)
		return err
	}
	if !m.Supported {
		e.setStage("Failed: unsupported image", 0, 0)
		return errors.New("game is not a raw iso on the server")
	}
	s, err := store.Open(filepath.Join(e.dataDir, "games"), m.Serial, m.SizeBytes, 0)
	if err != nil {
		e.setStage("Failed: storage error", 0, 0)
		return err
	}
	e.prepareSaves(c, m.Serial)
	e.setStage("Probing link", 0, 0)
	link, err := c.Probe(m.Serial)
	if err != nil {
		link = api.ProbeResult{Class: "lan"}
	}
	e.mu.Lock()
	e.probe = link
	e.mu.Unlock()
	fetchCfg := fusefs.ConfigForRTT(link.RTTMs)
	preload := syncer.PreloadRanges(m.SizeBytes, m.BootRanges, link.Class)
	var want [][2]int64
	for _, r := range preload {
		want = append(want, s.MissingRanges(r[0], r[1]-r[0])...)
	}
	if len(want) > 0 {
		var total int64
		for _, r := range want {
			total += r[1] - r[0]
		}
		e.setStage("Preloading boot data", 0, total)
		if err := syncer.FillRangesProgress(c, m.Serial, s, want, 4, func(done, totalRanges int) {
			e.setStage("Preloading boot data", s.DoneBytes(), total)
		}); err != nil {
			s.Close()
			e.setStage("Failed: preload interrupted", 0, 0)
			return err
		}
	}
	mnt := filepath.Join(e.dataDir, "mnt", mntName(m.Serial))
	if err := os.MkdirAll(mnt, 0o755); err != nil {
		s.Close()
		e.setStage("Failed: storage error", 0, 0)
		return err
	}
	e.setStage("Mounting game image", 0, 0)
	reader := fusefs.NewReader(c, s, m.SizeBytes, m.Serial, fetchCfg)
	stopTrace := func() {}
	if e.profiling {
		if tr, err := profile.Start(e.dataDir, m.Serial); err == nil {
			reader.Trace(tr)
			stopTrace = func() { tr.Close() }
		}
	}
	server, err := fusefs.Mount(mnt, reader, m.SizeBytes)
	if err != nil {
		stopTrace()
		s.Close()
		e.setStage("Failed: couldn't mount", 0, 0)
		return err
	}
	e.mu.Lock()
	e.activeMnt = mnt
	e.mu.Unlock()
	e.setStage("Preparing emulator", 0, 0)
	app, err := pcsx2.Ensure(filepath.Join(e.dataDir, "emulator"), e.version)
	if err != nil {
		server.Unmount()
		stopTrace()
		s.Close()
		e.setStage("Failed: no emulator", 0, 0)
		return err
	}
	if err := pcsx2.Prepare(e.dataDir); err != nil {
		server.Unmount()
		stopTrace()
		s.Close()
		e.setStage("Failed: emulator setup", 0, 0)
		return err
	}
	iso := filepath.Join(mnt, "game.iso")
	e.setStage("Starting PCSX2", 0, 0)
	cmd, err := pcsx2.Start(app, e.dataDir, iso)
	if err != nil {
		server.Unmount()
		stopTrace()
		s.Close()
		e.setStage("Failed: couldn't start", 0, 0)
		return err
	}
	e.mu.Lock()
	e.proc = cmd
	e.runSerial = m.Serial
	e.runTitle = m.Title
	e.runSince = time.Now()
	e.stage = ""
	e.stageDone, e.stageTotal = 0, 0
	e.mu.Unlock()
	go func() {
		defer func() {
			server.Unmount()
			e.mu.Lock()
			e.activeMnt = ""
			e.mu.Unlock()
		}()
		defer stopTrace()
		defer s.Close()
		_ = cmd.Wait()
		e.pushSaves(c, m.Serial)
		e.mu.Lock()
		if e.proc == cmd {
			e.proc = nil
		}
		e.mu.Unlock()
	}()
	return nil
}

// ServiceShutdown implements the Wails lifecycle hook: on app quit, kill
// the emulator and unmount the game image so the next launch mounts clean.
func (e *EmulatorService) ServiceShutdown() error {
	if err := e.StopEmulator(); err != nil {
		return err
	}
	e.mu.Lock()
	mnt := e.activeMnt
	e.mu.Unlock()
	if mnt != "" {
		unmountGUI(mnt)
	}
	return nil
}

func mntName(serial string) string {
	var b strings.Builder
	for _, r := range serial {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "game"
	}
	return b.String()
}
