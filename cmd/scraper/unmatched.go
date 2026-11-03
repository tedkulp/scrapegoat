package main

import (
	"bufio"
	"io"
	"strings"

	"github.com/tedkulp/scrapegoat/internal/scanner"
)

// unmatchedDisposition is what happens to a run's unmatched ROMs (not found,
// non-game, or failed) once the run ends. The zero value asks the user.
type unmatchedDisposition int

const (
	dispositionAsk unmatchedDisposition = iota
	dispositionKeep
	dispositionDelete
)

func (d unmatchedDisposition) String() string {
	switch d {
	case dispositionKeep:
		return "keep"
	case dispositionDelete:
		return "delete"
	default:
		return "ask"
	}
}

// unmatchedRemoval is an unmatched ROM the remover could not delete.
type unmatchedRemoval struct {
	ROM scanner.ROMFile
	Err error
}

// unmatchedDisposal reports what disposeUnmatchedROMs did.
type unmatchedDisposal struct {
	// Deleting is true when the ROMs were to be deleted: the disposition
	// was delete, or it was ask and the user answered yes.
	Deleting bool
	Deleted  []scanner.ROMFile
	Failed   []unmatchedRemoval
}

// disposeUnmatchedROMs applies the disposition to the unmatched ROMs. For
// ask it calls prompt once; only an answer of "y" or "yes" (any case,
// surrounding space ignored) deletes, and a read error keeps. A remover error
// on one ROM does not stop the others. It logs nothing: the caller reports
// from the result.
func disposeUnmatchedROMs(d unmatchedDisposition, roms []scanner.ROMFile, prompt func() (string, error), remove func(path string) error) unmatchedDisposal {
	var res unmatchedDisposal
	switch d {
	case dispositionDelete:
		res.Deleting = true
	case dispositionAsk:
		answer, err := prompt()
		if err == nil {
			answer = strings.ToLower(strings.TrimSpace(answer))
			res.Deleting = answer == "y" || answer == "yes"
		}
	}
	if !res.Deleting {
		return res
	}
	for _, rom := range roms {
		if err := remove(rom.Path); err != nil {
			res.Failed = append(res.Failed, unmatchedRemoval{ROM: rom, Err: err})
		} else {
			res.Deleted = append(res.Deleted, rom)
		}
	}
	return res
}

// lineReader returns a prompt that reads one line from r. A final line
// without a newline still counts as an answer; nothing at all is an error.
func lineReader(r io.Reader) func() (string, error) {
	br := bufio.NewReader(r)
	return func() (string, error) {
		line, err := br.ReadString('\n')
		if err != nil && line == "" {
			return "", err
		}
		return line, nil
	}
}
