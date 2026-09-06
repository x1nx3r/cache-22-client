// Package joy reads Linux joystick devices (/dev/input/js*) with pure Go,
// no cgo or SDL required. It exists for controller capture: PCSX2 accepts
// raw joystick tokens (SDL-<id>/JoyButton<N>, SDL-<id>/+JoyAxis<N>, ...),
// so capture does not need SDL's gamecontroller mapping database.
package joy

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	jsEventButton = 0x01
	jsEventAxis   = 0x02
	jsEventInit   = 0x80

	jsGetNameLen = 128
	jsGetName    = 0x80806A13 // _IOR('j', 0x13, char[128])

	axisThreshold = 16000
	settleTime    = 200 * time.Millisecond
)

// jsEvent is the 8-byte Linux joystick event.
type jsEvent struct {
	Time   uint32
	Value  int16
	Type   uint8
	Number uint8
}

func parseEvent(b []byte) jsEvent {
	return jsEvent{
		Time:   binary.LittleEndian.Uint32(b[0:4]),
		Value:  int16(binary.LittleEndian.Uint16(b[4:6])),
		Type:   b[6],
		Number: b[7],
	}
}

type Device struct {
	Index int    `json:"index"`
	Path  string `json:"path"`
	Name  string `json:"name"`
}

// List returns joystick devices sorted by index (js0, js1, ...).
// The index doubles as the PCSX2 SDL player id (SDL-<index>).
func List() ([]Device, error) {
	paths, err := filepath.Glob("/dev/input/js*")
	if err != nil {
		return nil, err
	}
	var out []Device
	for _, p := range paths {
		base := filepath.Base(p)
		if strings.Contains(base, "-") {
			continue
		}
		var idx int
		if _, err := fmt.Sscanf(base, "js%d", &idx); err != nil {
			continue
		}
		out = append(out, Device{Index: idx, Path: p, Name: deviceName(p)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Index < out[j].Index })
	return out, nil
}

func deviceName(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return filepath.Base(path)
	}
	defer f.Close()
	buf := make([]byte, jsGetNameLen)
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, f.Fd(), jsGetName, uintptr(unsafe.Pointer(&buf[0])))
	if errno != 0 {
		return filepath.Base(path)
	}
	if i := strings.IndexByte(string(buf), 0); i >= 0 {
		buf = buf[:i]
	}
	name := strings.TrimSpace(string(buf))
	if name == "" {
		return filepath.Base(path)
	}
	return name
}

// Capture waits for the first meaningful input on the device and returns
// the PCSX2 SDL token for it, e.g. SDL-0/JoyButton3 or SDL-0/+JoyAxis1.
// Axis baselines settle briefly first so resting triggers don't fire.
func Capture(index int, timeout time.Duration) (string, error) {
	path := fmt.Sprintf("/dev/input/js%d", index)
	f, err := os.Open(path)
	if err != nil {
		if os.IsPermission(err) {
			return "", fmt.Errorf("cannot open %s: permission denied, join the input group", path)
		}
		return "", err
	}
	defer f.Close()

	events := make(chan jsEvent, 64)
	done := make(chan struct{})
	defer close(done)
	go func() {
		buf := make([]byte, 8)
		for {
			select {
			case <-done:
				return
			default:
			}
			n, err := f.Read(buf)
			if err != nil || n != 8 {
				return
			}
			select {
			case events <- parseEvent(buf):
			case <-done:
				return
			}
		}
	}()

	base := map[uint8]int16{}
	deadline := time.Now().Add(timeout)
	settled := time.Now().Add(settleTime)
	for {
		remain := time.Until(deadline)
		if remain <= 0 {
			return "", fmt.Errorf("no input received, press a button or move a stick")
		}
		select {
		case ev := <-events:
			if ev.Type&jsEventInit != 0 {
				continue
			}
			switch {
			case ev.Type&jsEventButton != 0:
				if ev.Value == 1 {
					return fmt.Sprintf("SDL-%d/JoyButton%d", index, ev.Number), nil
				}
			case ev.Type&jsEventAxis != 0:
				b, seen := base[ev.Number]
				if !seen {
					base[ev.Number] = ev.Value
					continue
				}
				if time.Now().Before(settled) {
					base[ev.Number] = ev.Value
					continue
				}
				if d := int(ev.Value) - int(b); d > axisThreshold {
					return fmt.Sprintf("SDL-%d/+JoyAxis%d", index, ev.Number), nil
				} else if d < -axisThreshold {
					return fmt.Sprintf("SDL-%d/-JoyAxis%d", index, ev.Number), nil
				}
			}
		case <-time.After(remain):
			return "", fmt.Errorf("no input received, press a button or move a stick")
		}
	}
}
