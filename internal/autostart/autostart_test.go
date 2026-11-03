package autostart_test

import (
	"errors"
	"testing"

	"agentsync/internal/autostart"
)

const testExecutable = `C:\Apps\AgentSync\agentsync.exe`

// fakeStore keeps the entry in memory so no test can touch the real registry.
type fakeStore struct {
	command string
	exists  bool
	getErr  error
	sets    int
}

func (f *fakeStore) Get() (string, bool, error) { return f.command, f.exists, f.getErr }

func (f *fakeStore) Set(command string) error {
	f.command, f.exists = command, true
	f.sets++
	return nil
}

func (f *fakeStore) Remove() error {
	f.command, f.exists = "", false
	return nil
}

func TestEnableWritesQuotedCommandWithMinimizedFlag(t *testing.T) {
	store := &fakeStore{}
	manager := autostart.NewManager(store, testExecutable, true)

	if err := manager.Enable(); err != nil {
		t.Fatal(err)
	}
	want := `"` + testExecutable + `" --minimized`
	if store.command != want {
		t.Fatalf("stored %s, want %s", store.command, want)
	}
	if status := manager.Status(); !status.Supported || !status.Enabled {
		t.Fatalf("status = %+v", status)
	}
}

func TestDisableRemovesEntryAndIsIdempotent(t *testing.T) {
	store := &fakeStore{}
	manager := autostart.NewManager(store, testExecutable, true)
	_ = manager.Enable()

	for range 2 {
		if err := manager.Disable(); err != nil {
			t.Fatal(err)
		}
	}
	if status := manager.Status(); status.Enabled {
		t.Fatalf("entry still enabled: %+v", status)
	}
}

func TestStatusReflectsEntryRemovedOutsideTheApp(t *testing.T) {
	store := &fakeStore{}
	manager := autostart.NewManager(store, testExecutable, true)
	_ = manager.Enable()
	store.exists = false

	if status := manager.Status(); status.Enabled {
		t.Fatalf("status must follow the real entry: %+v", status)
	}
}

func TestRepairRewritesOnlyWhenPathDiffers(t *testing.T) {
	store := &fakeStore{}
	manager := autostart.NewManager(store, testExecutable, true)

	if repaired, err := manager.Repair(); err != nil || repaired || store.exists {
		t.Fatalf("no entry: repaired=%v err=%v exists=%v", repaired, err, store.exists)
	}

	store.command, store.exists = `"D:\Old\agentsync.exe" --minimized`, true
	if repaired, err := manager.Repair(); err != nil || !repaired || store.command != manager.Command() {
		t.Fatalf("moved exe: repaired=%v err=%v command=%s", repaired, err, store.command)
	}

	sets := store.sets
	if repaired, err := manager.Repair(); err != nil || repaired || store.sets != sets {
		t.Fatalf("up to date: repaired=%v err=%v", repaired, err)
	}
}

func TestUnsupportedManagerRefusesChanges(t *testing.T) {
	store := &fakeStore{}
	managers := map[string]*autostart.Manager{
		"development build": autostart.NewManager(store, testExecutable, false),
		"no store":          autostart.NewManager(nil, testExecutable, true),
	}
	for name, manager := range managers {
		if err := manager.Enable(); !errors.Is(err, autostart.ErrUnsupported) {
			t.Errorf("%s: Enable = %v", name, err)
		}
		if err := manager.Apply(false); !errors.Is(err, autostart.ErrUnsupported) {
			t.Errorf("%s: Apply(false) = %v", name, err)
		}
		if status := manager.Status(); status.Supported || status.Enabled {
			t.Errorf("%s: status = %+v", name, status)
		}
	}
	if store.exists || store.sets != 0 {
		t.Fatal("an unsupported manager wrote to the store")
	}
}

func TestStatusReportsDisabledWhenStoreFails(t *testing.T) {
	store := &fakeStore{exists: true, getErr: errors.New("access denied")}
	manager := autostart.NewManager(store, testExecutable, true)
	if status := manager.Status(); !status.Supported || status.Enabled {
		t.Fatalf("status = %+v", status)
	}
}
