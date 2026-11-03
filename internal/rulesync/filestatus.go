package rulesync

// EvaluateFileStatus computes the status of one file pair from the source, destination and
// baseline (last synced) hashes. An empty hash means the file is absent or never synced.
func EvaluateFileStatus(sourceHash, targetHash, baselineHash string) string {
	// S == T -> in_sync
	if sourceHash == targetHash && sourceHash != "" {
		return StatusInSync
	}

	// B absent and T absent -> new_in_source
	if baselineHash == "" && targetHash == "" {
		return StatusNewInSource
	}

	// B absent and T != S (with T present) -> untracked_diff
	if baselineHash == "" && targetHash != "" && targetHash != sourceHash {
		return StatusUntrackedDiff
	}

	// S != B and T == B -> source_changed
	if baselineHash != "" && sourceHash != baselineHash && targetHash == baselineHash {
		return StatusSourceChanged
	}

	// S == B and T != B (T present) -> target_changed
	if baselineHash != "" && sourceHash == baselineHash && targetHash != "" && targetHash != baselineHash {
		return StatusTargetChanged
	}

	// S == B and T absent -> target_missing
	if baselineHash != "" && sourceHash == baselineHash && targetHash == "" {
		return StatusTargetMissing
	}

	// S != B and T absent -> source_changed (restore from updated source)
	if baselineHash != "" && sourceHash != baselineHash && targetHash == "" {
		return StatusSourceChanged
	}

	// S != B, T != B and S != T -> conflict, which is also the safe default.
	return StatusConflict
}
