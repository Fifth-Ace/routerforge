package main

import (
	"net/http"
	"sort"
	"time"
)

type rfSyslogSourceSummary struct {
	Source string `json:"source"`
	Files  int    `json:"files"`
	Bytes  int64  `json:"bytes"`
	Latest string `json:"latest"`
}

// rfSyslogCleanupCandidates calculates the hourly policy without deleting anything.
// Today's files remain protected even if they exceed the configured size limit.
func rfSyslogCleanupCandidates(files []rfSyslogEntry, s rfSyslogSettings, now time.Time) (int, int64) {
	cutoff := now.Local().AddDate(0, 0, -s.KeepDays).Format("2006-01-02")
	today := now.Local().Format("2006-01-02")
	ordered := append([]rfSyslogEntry(nil), files...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Date == ordered[j].Date {
			return ordered[i].Modified.Before(ordered[j].Modified)
		}
		return ordered[i].Date < ordered[j].Date
	})
	total := int64(0)
	for _, entry := range ordered {
		total += entry.Size
	}
	count := 0
	size := int64(0)
	limit := int64(s.MaxMB) * 1024 * 1024
	for _, entry := range ordered {
		if entry.Date >= today {
			continue
		}
		if entry.Date >= cutoff && total <= limit {
			continue
		}
		count++
		size += entry.Size
		total -= entry.Size
	}
	return count, size
}
func rfSyslogSummary(w http.ResponseWriter, r *http.Request) {
	s, err := rfSyslogManagedRoot()
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
		return
	}
	files, err := rfSyslogFiles(s)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot inspect managed log files"})
		return
	}
	bySource := map[string]*rfSyslogSourceSummary{}
	var total int64
	first, last := "", ""
	for _, f := range files {
		total += f.Size
		if first == "" || f.Date < first {
			first = f.Date
		}
		if f.Date > last {
			last = f.Date
		}
		m := bySource[f.Source]
		if m == nil {
			m = &rfSyslogSourceSummary{Source: f.Source}
			bySource[f.Source] = m
		}
		m.Files++
		m.Bytes += f.Size
		if f.Date > m.Latest {
			m.Latest = f.Date
		}
	}
	list := make([]rfSyslogSourceSummary, 0, len(bySource))
	for _, m := range bySource {
		list = append(list, *m)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Source < list[j].Source })
	candidates, bytes := rfSyslogCleanupCandidates(files, s, time.Now())
	writeJSON(w, http.StatusOK, map[string]any{"files": len(files), "total_bytes": total, "oldest_date": first, "latest_date": last, "sources": list, "cleanup_files": candidates, "cleanup_bytes": bytes, "keep_days": s.KeepDays, "max_mb": s.MaxMB, "cleanup_schedule": "hourly", "today_protected": true, "read_only": true})
}
