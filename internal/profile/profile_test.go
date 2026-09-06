package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTraceAndAggregate(t *testing.T) {
	dir := t.TempDir()
	tr, err := Start(dir, "G/1")
	if err != nil {
		t.Fatal(err)
	}
	tr.Log(0, 100)
	tr.Log(1<<20, 100)
	tr.Log(0, 50)
	tr.Log(5<<20+10, 20)
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(TracePath(dir, "G/1")); err != nil {
		t.Fatalf("trace not stored: %v", err)
	}
	if filepath.Base(TracePath(dir, "G/1")) != "G_1.trace.jsonl" {
		t.Errorf("trace name = %q", TracePath(dir, "G/1"))
	}

	hm, err := Aggregate(dir, "G/1", 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if len(hm.Chunks) != 3 {
		t.Fatalf("chunks = %+v", hm.Chunks)
	}
	if hm.Chunks[0].Index != 0 || hm.Chunks[0].Count != 2 || hm.Chunks[0].First != 0 {
		t.Errorf("chunk0 = %+v", hm.Chunks[0])
	}
	if hm.Chunks[1].Index != 1 || hm.Chunks[2].Index != 5 {
		t.Errorf("order = %+v", hm.Chunks)
	}
	if got := HotSet(hm, 2); len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Errorf("hotset = %v", got)
	}
	if got := HotSet(hm, 99); len(got) != 3 {
		t.Errorf("hotset capped = %v", got)
	}

	if err := Save(dir, "G/1", hm); err != nil {
		t.Fatal(err)
	}
	hm2, err := Aggregate(dir, "G/1", 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if hm2.Sessions != hm.Sessions+1 {
		t.Errorf("sessions = %d, want merge", hm2.Sessions)
	}
}

func TestAggregateNoTrace(t *testing.T) {
	if _, err := Aggregate(t.TempDir(), "NOPE", 1<<20); err == nil {
		t.Error("want error for missing trace")
	}
}

func TestNilTracer(t *testing.T) {
	var tr *Tracer
	tr.Log(0, 1)
	if err := tr.Close(); err != nil {
		t.Errorf("nil close = %v", err)
	}
}
