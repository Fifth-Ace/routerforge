package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// NF5B refuses to delete a list while the live config or any safely named
// saved config mentions it. This is deliberately conservative: an uncertain
// reference is safer than a dangling --hostlist/--ipset argument.
func nf5bReferencesInContent(data []byte, name string) bool {
	lists, _ := smartApplyReferencedNames(string(data))
	return lists[name] || strings.Contains(string(data), name)
}

func nf5bReadReferenceConfig(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() > configMaxBytes {
		return nil, errors.New("reference config must be a regular bounded file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) > configMaxBytes {
		return nil, errors.New("reference config grew during read")
	}
	return data, nil
}

func nf5bListDeleteReferences(name string) ([]string, error) {
	if !safeListSourceName(name) {
		return nil, errors.New("unsafe list name")
	}
	refs := []string{}
	active, err := nf5bReadReferenceConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("read active config before deletion: %w", err)
	}
	if nf5bReferencesInContent(active, name) {
		refs = append(refs, "active:nfqws2.conf")
	}

	entries, err := os.ReadDir(configLibraryRoot)
	if errors.Is(err, os.ErrNotExist) {
		return refs, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config library before deletion: %w", err)
	}
	for _, entry := range entries {
		if !safeConfigLibraryName(entry.Name()) {
			continue
		}
		path := filepath.Join(configLibraryRoot, entry.Name())
		data, err := nf5bReadReferenceConfig(path)
		if err != nil {
			return nil, fmt.Errorf("inspect saved config %s: %w", entry.Name(), err)
		}
		if nf5bReferencesInContent(data, name) {
			refs = append(refs, "library:"+entry.Name())
		}
	}
	sort.Strings(refs)
	return refs, nil
}

func nf5bAllowDelete(name string, data []byte, expected string, references []string) error {
	if !safeListSourceName(name) || protectedListName(name) {
		return errors.New("unsafe or protected list cannot be deleted")
	}
	if err := nf5aRequireListPreimage(data, expected); err != nil {
		return err
	}
	if len(references) != 0 {
		return fmt.Errorf("list is referenced by %s; detach it first", strings.Join(references, ", "))
	}
	return nil
}
