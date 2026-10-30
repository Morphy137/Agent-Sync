package rulesync

import (
	"bytes"
	"os"
	"unicode/utf8"

	"agentsync/internal/apperr"
	"agentsync/internal/rules"
)

const (
	// binarySniffBytes is how many leading bytes are scanned for NUL when detecting binary content.
	binarySniffBytes = 8000
	diffLeftLabel    = "source"
	diffRightLabel   = "destination"
)

// isBinary reports a NUL byte in the first binarySniffBytes bytes or invalid UTF-8.
func isBinary(data []byte) bool {
	sniff := data
	if len(sniff) > binarySniffBytes {
		sniff = sniff[:binarySniffBytes]
	}
	return bytes.IndexByte(sniff, 0) >= 0 || !utf8.Valid(data)
}

// readForDiff reads a file for the diff view. Missing files read as empty; files larger
// than MaxDiffBytes are reported as too large.
func readForDiff(p string) (data []byte, tooLarge bool, err error) {
	info, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, apperr.Wrap(apperr.WriteFailed, err).WithParam("path", p)
	}
	if info.IsDir() {
		return nil, false, apperr.New(apperr.TargetNotFound).WithParam("path", p)
	}
	if info.Size() > MaxDiffBytes {
		return nil, true, nil
	}
	data, err = os.ReadFile(p)
	if err != nil {
		return nil, false, apperr.Wrap(apperr.WriteFailed, err).WithParam("path", p)
	}
	return data, false, nil
}

// Diff returns both sides of one file. For transformed destinations Left is the output a
// use_source resolution would write and Right is the current destination file.
func (e *Engine) Diff(all []rules.Rule, ruleID, destID, relPath string) (Diff, error) {
	out := Diff{LeftLabel: diffLeftLabel, RightLabel: diffRightLabel}
	de, pair, err := e.locate(all, ruleID, destID, relPath)
	if err != nil {
		return out, err
	}

	right, rightTooLarge, err := readForDiff(pair.TargetAbs)
	if err != nil {
		return out, err
	}

	var left []byte
	leftTooLarge := false
	if de.isTransform() {
		out.Transform = true
		left, err = e.renderPreview(de, pair, right, rightTooLarge)
		if err != nil {
			return out, err
		}
		leftTooLarge = len(left) > MaxDiffBytes
	} else {
		left, leftTooLarge, err = readForDiff(pair.SourceAbs)
		if err != nil {
			return out, err
		}
	}

	if leftTooLarge || rightTooLarge || isBinary(left) || isBinary(right) {
		out.Binary = true
		return out, nil
	}
	out.Left, out.Right = string(left), string(right)
	return out, nil
}

func (e *Engine) renderPreview(de *destEval, pair rules.Pair, existing []byte, existingTooLarge bool) ([]byte, error) {
	if existingTooLarge {
		data, err := os.ReadFile(pair.TargetAbs)
		if err != nil {
			return nil, apperr.Wrap(apperr.WriteFailed, err)
		}
		existing = data
	}
	src, err := os.ReadFile(pair.SourceAbs)
	if err != nil {
		return nil, apperr.Wrap(apperr.SourceNotFound, err).WithParam("path", pair.SourceAbs)
	}
	prev := e.managedEntries(de.rule.ID, de.dest.ID)
	rendered, _, err := e.render(*de.dest.Transform, src, existing, prev, true)
	return rendered, err
}
