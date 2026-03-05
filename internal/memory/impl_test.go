package memory

import (
	"os"
	"testing"
	"time"

	"github.com/clawfactory/clawfactory/internal/store"
	"pgregory.net/rapid"
)

func newTestMemory(t *testing.T) (*FileSystemMemory, string) {
	tmpDir, err := os.MkdirTemp("", "clawfactory-mem-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	tmpDB, err := os.CreateTemp("", "clawfactory-mem-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpDB.Close()
	t.Cleanup(func() { os.Remove(tmpDB.Name()) })

	s, err := store.NewSQLiteStore(tmpDB.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	return NewFileSystemMemory(tmpDir, s), tmpDir
}

// Property 18: 产出物存储与工作流隔离
// **Validates: Requirements 12.1, 12.2, 12.3**
func TestProperty18_ArtifactWorkflowIsolation(t *testing.T) {
	mem, _ := newTestMemory(t)

	rapid.Check(t, func(rt *rapid.T) {
		wf1 := "wf-" + rapid.StringMatching("[a-z]{3}").Draw(rt, "wf1")
		wf2 := "wf-" + rapid.StringMatching("[a-z]{3}").Draw(rt, "wf2")
		// Ensure different workflow IDs
		for wf2 == wf1 {
			wf2 = "wf-" + rapid.StringMatching("[a-z]{3}").Draw(rt, "wf2retry")
		}

		// Store artifacts in wf1
		n1 := rapid.IntRange(1, 3).Draw(rt, "n1")
		for i := 0; i < n1; i++ {
			name := rapid.StringMatching("[a-z]{4}").Draw(rt, "name1") + ".txt"
			_, err := mem.StoreArtifact(wf1, "task-1", name, []byte("data1"))
			if err != nil {
				rt.Fatal(err)
			}
		}

		// Store artifacts in wf2
		n2 := rapid.IntRange(1, 3).Draw(rt, "n2")
		for i := 0; i < n2; i++ {
			name := rapid.StringMatching("[a-z]{4}").Draw(rt, "name2") + ".txt"
			_, err := mem.StoreArtifact(wf2, "task-2", name, []byte("data2"))
			if err != nil {
				rt.Fatal(err)
			}
		}

		// Query wf1 artifacts — should not contain wf2 artifacts
		arts1, err := mem.GetArtifacts(wf1)
		if err != nil {
			rt.Fatal(err)
		}
		for _, a := range arts1 {
			if a.WorkflowID != wf1 {
				rt.Fatalf("wf1 query returned artifact from %s", a.WorkflowID)
			}
		}

		// Query wf2 artifacts — should not contain wf1 artifacts
		arts2, err := mem.GetArtifacts(wf2)
		if err != nil {
			rt.Fatal(err)
		}
		for _, a := range arts2 {
			if a.WorkflowID != wf2 {
				rt.Fatalf("wf2 query returned artifact from %s", a.WorkflowID)
			}
		}

		// Read artifact round-trip
		art, err := mem.StoreArtifact(wf1, "task-rt", "round.txt", []byte("hello"))
		if err != nil {
			rt.Fatal(err)
		}
		data, err := mem.ReadArtifact(art)
		if err != nil {
			rt.Fatal(err)
		}
		if string(data) != "hello" {
			rt.Fatalf("read artifact: got %q, want %q", string(data), "hello")
		}
	})
}

// Unit test: store and retrieve artifact
func TestStoreAndReadArtifact(t *testing.T) {
	mem, _ := newTestMemory(t)
	art, err := mem.StoreArtifact("wf-1", "task-1", "output.txt", []byte("test content"))
	if err != nil {
		t.Fatal(err)
	}
	if art.WorkflowID != "wf-1" || art.TaskID != "task-1" || art.Name != "output.txt" {
		t.Fatalf("unexpected artifact: %+v", art)
	}
	if art.CreatedAt.Before(time.Now().Add(-time.Minute)) {
		t.Fatal("created_at too old")
	}
	data, err := mem.ReadArtifact(art)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "test content" {
		t.Fatalf("got %q", string(data))
	}
}
