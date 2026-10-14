package main

// NF5F: inventory references must identify the exact list basename in a
// supported NFQWS selection flag. A substring match erroneously marks
// a.list referenced merely because the config uses data.list. Reuse the
// same static dependency parser already used by Smart Apply.
//
// The destructive delete guard remains independently conservative: this
// helper changes inventory labels only, never deletion authorization.
func nf5fExactListReference(content []byte, name string) bool {
	if !safeListSourceName(name) {
		return false
	}
	lists, _ := smartApplyReferencedNames(string(content))
	return lists[name]
}
