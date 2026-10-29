package dnsrouter

import (
	"context"
	"fmt"
	"time"
)

// RunManagedListUpdater only runs while the management service is alive.
// No network fetch occurs unless a user explicitly persisted an HTTPS URL with
// auto_update_minutes > 0. It never starts DNS, applies shadow rules or calls ndmc.
func RunManagedListUpdater(ctx context.Context, store *ConfigStore, shadowActive func() bool, logf func(string, ...any)) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if shadowActive != nil && shadowActive() {
			continue
		}
		cfg, rev, err := store.Load()
		if err != nil {
			logf("DNS lists: config load failed: %v", err)
			continue
		}
		now := time.Now().UTC()
		for _, list := range cfg.Lists {
			if ctx.Err() != nil {
				return
			}
			if list.Kind != "url" || list.AutoUpdateMinutes == 0 {
				continue
			}
			if list.UpdatedAt != "" {
				last, err := time.Parse(time.RFC3339, list.UpdatedAt)
				if err != nil || now.Sub(last) < time.Duration(list.AutoUpdateMinutes)*time.Minute {
					continue
				}
			}
			if shadowActive != nil && shadowActive() {
				break
			}
			fetchCtx, cancel := context.WithTimeout(ctx, 18*time.Second)
			raw, fetchErr := FetchManagedListText(fetchCtx, list.URL)
			cancel()
			if fetchErr != nil {
				logf("DNS lists: automatic refresh %s skipped: %v", list.ID, fetchErr)
				continue
			}
			list.UpdatedAt = now.Format(time.RFC3339)
			preview, previewErr := PreviewManagedList(cfg, ManagedListRequest{List: list, Text: raw})
			if previewErr != nil || preview.Invalid != 0 {
				logf("DNS lists: automatic refresh %s rejected: %v, invalid=%d", list.ID, previewErr, preview.Invalid)
				continue
			}
			if shadowActive != nil && shadowActive() {
				break
			}
			updated, saveErr := store.Save(preview.Config, rev)
			if saveErr != nil {
				logf("DNS lists: automatic refresh %s CAS skip: %v", list.ID, saveErr)
				break
			}
			logf("DNS lists: automatic refresh %s saved %d rules; runtime NOT applied", list.ID, preview.ListRules)
			cfg = preview.Config
			rev = updated
		}
	}
}

// ManagedListUpdateStatus explains the limited opt-in background behavior.
func ManagedListUpdateStatus(cfg Config) map[string]any {
	enabled := 0
	for _, l := range cfg.Lists {
		if l.Kind == "url" && l.AutoUpdateMinutes > 0 {
			enabled++
		}
	}
	return map[string]any{
		"lists": len(cfg.Lists), "auto_enabled": enabled,
		"background_interval_minutes": 10,
		"shadow_apply":                false, "keenetic_dns_change": false,
		"note": fmt.Sprintf("%d URL subscriptions explicitly opted in; changes remain unapplied to running shadow DNS", enabled),
	}
}
