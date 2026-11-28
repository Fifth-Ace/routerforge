//go:build linux

package apptransaction

import (
	"context"
	"errors"
	"time"
)

// ManagementPreview is deliberately weaker than a lifecycle proof. A working
// management endpoint does not prove boot intent or the data plane.
type ManagementPreview struct {
	Package          string `json:"package"`
	InstalledVersion string `json:"installed_version,omitempty"`
	State            string `json:"state"`
}

// PreviewDeviceManagement does not acquire an execution lease, download assets,
// modify configuration or launch OPKG mutations. Its observations are advisory.
func PreviewDeviceManagement(ctx context.Context, plan Plan) ([]ManagementPreview, error) {
	return previewManagement(ctx, plan, func(ctx context.Context, name string) (string, error) {
		return readInstalledDevVersion(ctx, "/opt/bin/opkg", name)
	}, func(ctx context.Context, name, installed, core string) error {
		if specialLifecyclePackage(name) {
			_, err := ProbeSpecialLifecycleCandidate(ctx, name, installed, core)
			return err
		}
		_, err := ProbeRunningModuleHealth(ctx, name, installed)
		return err
	})
}

func previewManagement(ctx context.Context, plan Plan, status func(context.Context, string) (string, error), probe func(context.Context, string, string, string) error) ([]ManagementPreview, error) {
	if _, err := plan.Hash(); err != nil {
		return nil, err
	}
	if status == nil || probe == nil {
		return nil, errors.New("missing management preview dependencies")
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	out := make([]ManagementPreview, 0, len(plan.Packages))
	for _, pkg := range plan.Packages {
		v := ManagementPreview{Package: pkg.Name, State: "not_checked"}
		if ctx.Err() != nil {
			out = append(out, v)
			continue
		}
		installed, err := status(ctx, pkg.Name)
		if err != nil {
			v.State = "installed_state_unavailable"
			out = append(out, v)
			continue
		}
		v.InstalledVersion = installed
		if installed == pkg.Version || allowUpgrade(installed, pkg.Version) != nil {
			v.State = "version_conflict"
			out = append(out, v)
			continue
		}
		core := installed
		if pkg.Name == "routerforge-profiling" {
			core, err = status(ctx, "routerforge-core")
		}
		if err != nil || probe(ctx, pkg.Name, installed, core) != nil {
			v.State = "management_unavailable"
			out = append(out, v)
			continue
		}
		again, err := status(ctx, pkg.Name)
		if err != nil || again != installed {
			v.State = "version_conflict"
			out = append(out, v)
			continue
		}
		if pkg.Name == "routerforge-profiling" {
			again, err = status(ctx, "routerforge-core")
			if err != nil || again != core {
				v.State = "version_conflict"
				out = append(out, v)
				continue
			}
		}
		if ctx.Err() == nil {
			v.State = "management_ready"
		}
		out = append(out, v)
	}
	return out, nil
}
