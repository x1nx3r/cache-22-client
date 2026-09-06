package joy

import (
	"encoding/binary"
	"testing"
	"time"
)

func TestParseEvent(t *testing.T) {
	b := make([]byte, 8)
	var neg int16 = -30000
	binary.LittleEndian.PutUint32(b[0:4], 1234)
	binary.LittleEndian.PutUint16(b[4:6], uint16(neg))
	b[6] = jsEventAxis
	b[7] = 2
	ev := parseEvent(b)
	if ev.Time != 1234 || ev.Value != -30000 || ev.Type != jsEventAxis || ev.Number != 2 {
		t.Errorf("event = %+v", ev)
	}
}

func TestListSorts(t *testing.T) {
	devs, err := List()
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(devs); i++ {
		if devs[i].Index <= devs[i-1].Index {
			t.Errorf("unsorted: %+v", devs)
		}
	}
}

func TestCaptureTimeout(t *testing.T) {
	devs, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(devs) == 0 {
		t.Skip("no joystick devices")
	}
	start := time.Now()
	_, err = Capture(devs[0].Index, 300*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout with no input")
	}
	if time.Since(start) > 5*time.Second {
		t.Error("capture must respect timeout")
	}
}
