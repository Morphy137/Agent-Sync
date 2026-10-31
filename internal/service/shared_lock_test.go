package service_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"agentsync/internal/config"
	"agentsync/internal/rules"
	"agentsync/internal/service"
)

const watchTestInterval = 10 * time.Millisecond

func sharedFixture(t *testing.T) (string, string) {
	t.Helper()
	base := t.TempDir()
	t.Setenv(config.ConfigEnvVar, filepath.Join(base, "config"))
	t.Setenv(rules.HomeOverrideEnvVar, filepath.Join(base, "home"))
	folderA := filepath.Join(base, "a")
	folderB := filepath.Join(base, "b")
	for _, d := range []string{folderA, folderB} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return folderA, folderB
}

func hasAlias(locs []service.LocationInfo, alias string) bool {
	for _, l := range locs {
		if l.Alias == alias {
			return true
		}
	}
	return false
}

// Two Services on the same config dir model the GUI and a CLI/MCP process running at once:
// each must see the other's writes and never overwrite them with a stale copy.
func TestSharedModeReloadsChangesFromOtherInstances(t *testing.T) {
	folderA, folderB := sharedFixture(t)
	gui := service.New(nil)
	cli := service.New(nil)

	if _, err := gui.SaveLocation("alpha", folderA); err != nil {
		t.Fatal(err)
	}
	if _, err := cli.SaveLocation("beta", folderB); err != nil {
		t.Fatal(err)
	}
	locs, err := gui.ListLocations()
	if err != nil {
		t.Fatal(err)
	}
	if !hasAlias(locs, "alpha") || !hasAlias(locs, "beta") {
		t.Fatalf("gui must see both aliases, got %+v", locs)
	}

	// A write from the GUI after the CLI's write must keep the CLI's alias.
	if _, err := gui.RemoveLocation("alpha"); err != nil {
		t.Fatal(err)
	}
	disk, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(disk.Locations) != 1 || disk.Locations[0].Alias != "beta" {
		t.Fatalf("stale overwrite: %+v", disk.Locations)
	}
}

func TestSharedModeConcurrentWritersKeepEveryChange(t *testing.T) {
	folderA, _ := sharedFixture(t)
	const writers = 4
	services := make([]*service.Service, writers)
	for i := range services {
		services[i] = service.New(nil)
	}
	aliases := []string{"w0", "w1", "w2", "w3"}
	var wg sync.WaitGroup
	for i, svc := range services {
		wg.Add(1)
		go func(svc *service.Service, alias string) {
			defer wg.Done()
			if _, err := svc.SaveLocation(alias, folderA); err != nil {
				t.Errorf("save %s: %v", alias, err)
			}
		}(svc, aliases[i])
	}
	wg.Wait()
	disk, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(disk.Locations) != writers {
		t.Fatalf("lost updates: %+v", disk.Locations)
	}
}

func TestWatchExternalChangesEmitsEvents(t *testing.T) {
	folderA, _ := sharedFixture(t)
	events := make(chan string, 16)
	gui := service.New(func(name string, _ any) { events <- name })
	cli := service.New(nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go gui.WatchExternalChanges(ctx, watchTestInterval)

	if _, err := cli.SaveLocation("alpha", folderA); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(5 * time.Second)
	for {
		select {
		case name := <-events:
			if name == "rules:changed" {
				return
			}
		case <-deadline:
			t.Fatal("gui did not notice the external change")
		}
	}
}
