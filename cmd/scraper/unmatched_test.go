package main

import (
	"errors"
	"io"
	"testing"

	"github.com/tedkulp/scrapegoat/internal/scanner"
)

func unmatchedFixture() []scanner.ROMFile {
	return []scanner.ROMFile{
		{Path: "/roms/a.nes", Filename: "a.nes"},
		{Path: "/roms/b.nes", Filename: "b.nes"},
		{Path: "/roms/c.nes", Filename: "c.nes"},
	}
}

type fakeRemover struct {
	calls []string
	fail  map[string]error
}

func (f *fakeRemover) remove(path string) error {
	f.calls = append(f.calls, path)
	return f.fail[path]
}

func answer(s string, err error) func() (string, error) {
	return func() (string, error) { return s, err }
}

func noPrompt(t *testing.T) func() (string, error) {
	return func() (string, error) {
		t.Fatal("prompt must not be called")
		return "", nil
	}
}

func TestDisposeUnmatchedROMs_KeepNeverRemoves(t *testing.T) {
	r := &fakeRemover{}
	res := disposeUnmatchedROMs(dispositionKeep, unmatchedFixture(), noPrompt(t), r.remove)
	if len(r.calls) != 0 {
		t.Fatalf("remover called %v", r.calls)
	}
	if res.Deleting || len(res.Deleted) != 0 || len(res.Failed) != 0 {
		t.Fatalf("result = %+v", res)
	}
}

func TestDisposeUnmatchedROMs_DeleteRemovesAll(t *testing.T) {
	r := &fakeRemover{}
	res := disposeUnmatchedROMs(dispositionDelete, unmatchedFixture(), noPrompt(t), r.remove)
	if len(r.calls) != 3 {
		t.Fatalf("remover calls = %v, want 3", r.calls)
	}
	if !res.Deleting || len(res.Deleted) != 3 || len(res.Failed) != 0 {
		t.Fatalf("result = %+v", res)
	}
}

func TestDisposeUnmatchedROMs_AskYesDeletes(t *testing.T) {
	for _, a := range []string{"y", "Y", " yes\n", "YES"} {
		r := &fakeRemover{}
		res := disposeUnmatchedROMs(dispositionAsk, unmatchedFixture(), answer(a, nil), r.remove)
		if len(r.calls) != 3 || len(res.Deleted) != 3 || !res.Deleting {
			t.Errorf("answer %q: calls=%v result=%+v", a, r.calls, res)
		}
	}
}

func TestDisposeUnmatchedROMs_AskKeeps(t *testing.T) {
	cases := []struct {
		name string
		ans  string
		err  error
	}{
		{"empty", "", nil},
		{"newline", "\n", nil},
		{"no", "n", nil},
		{"EOF", "", io.EOF},
		{"read error", "", errors.New("boom")},
	}
	for _, c := range cases {
		r := &fakeRemover{}
		res := disposeUnmatchedROMs(dispositionAsk, unmatchedFixture(), answer(c.ans, c.err), r.remove)
		if len(r.calls) != 0 || res.Deleting {
			t.Errorf("%s: calls=%v result=%+v", c.name, r.calls, res)
		}
	}
}

func TestDisposeUnmatchedROMs_RemoverErrorDoesNotStopOthers(t *testing.T) {
	boom := errors.New("permission denied")
	r := &fakeRemover{fail: map[string]error{"/roms/b.nes": boom}}
	res := disposeUnmatchedROMs(dispositionDelete, unmatchedFixture(), noPrompt(t), r.remove)
	if len(r.calls) != 3 {
		t.Fatalf("remover calls = %v, want all 3", r.calls)
	}
	if len(res.Deleted) != 2 || res.Deleted[0].Filename != "a.nes" || res.Deleted[1].Filename != "c.nes" {
		t.Fatalf("Deleted = %+v", res.Deleted)
	}
	if len(res.Failed) != 1 || res.Failed[0].ROM.Filename != "b.nes" || !errors.Is(res.Failed[0].Err, boom) {
		t.Fatalf("Failed = %+v", res.Failed)
	}
}
