package main

// NF5D: an upstream replacement is authorized only against the exact bytes
// shown by the local inventory. Recheck before backup and before final write.
// Missing files, symlinks, directories and oversized contents fail closed.
func nf5dReplacementPreimage(path, expectedOldSHA string) ([]byte, error) {
	before, _, err := nf5aReadRegularList(path)
	if err != nil {
		return nil, err
	}
	if err := nf5aRequireListPreimage(before, expectedOldSHA); err != nil {
		return nil, err
	}
	return before, nil
}
