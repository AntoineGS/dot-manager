package packages

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/AntoineGS/tidydots/internal/cmdexec"
)

func TestInstallPlanPreferenceAndDependencyOnly(t *testing.T) {
	for _, installer := range []bool{false, true} {
		m, stub := newStubManager(t, "linux")
		setAvailable(m, Apt, Brew)
		m.Config.ManagerPriority = []PackageManager{Brew, Apt}
		pkg := Package{Name: "tool", Managers: map[PackageManager]ManagerValue{
			Apt: {Deps: []string{"dependency"}}, Brew: {PackageName: "tool"},
		}}
		want := "brew"
		if installer {
			pkg.Managers[Installer] = ManagerValue{Installer: &InstallerConfig{Command: map[string]string{"linux": "install-tool"}}}
			want = "installer"
		}
		if got := m.GetInstallMethod(pkg); got != want {
			t.Errorf("method = %s, want %s", got, want)
		}
		result := m.Install(pkg)
		if !result.Success || result.Method != want {
			t.Fatalf("result = %+v", result)
		}
		if len(stub.Calls) != 2 || stub.Calls[0].Args[len(stub.Calls[0].Args)-1] != "dependency" {
			t.Fatalf("expected dependency then main, got %+v", stub.Calls)
		}
	}
}

func TestDependencyOnlyIsNotInstallable(t *testing.T) {
	m, stub := newStubManager(t, "linux")
	setAvailable(m, Apt)
	pkg := Package{Managers: map[PackageManager]ManagerValue{Apt: {Deps: []string{"dependency"}}}}
	if m.CanInstall(pkg) {
		t.Fatal("dependency-only entry cannot provide a main install method")
	}
	if m.Install(pkg).Success || len(stub.Calls) != 0 {
		t.Fatal("no main method must fail without installing dependencies")
	}
}

func TestPlanInstallationPriority(t *testing.T) {
	pkg := Package{Managers: map[PackageManager]ManagerValue{
		Apt:  {PackageName: "apt-main", Deps: []string{"apt-dep"}},
		Brew: {PackageName: "brew-main", Deps: []string{"brew-dep"}},
		Dnf:  {PackageName: "dnf-main", Deps: []string{"dnf-dep"}},
	}}
	for _, tc := range []struct {
		name string
		cfg  Config
		want string
	}{
		{name: "priority", cfg: Config{ManagerPriority: []PackageManager{Dnf, Brew, Apt}, DefaultManager: Apt}, want: "brew"},
		{name: "default", cfg: Config{DefaultManager: Brew}, want: "brew"},
		{name: "discovery order", want: "apt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, stub := newStubManager(t, "linux")
			setAvailable(m, Apt, Brew)
			m.Config = &tc.cfg
			plan := PlanInstallation(pkg, m.Config, m.OS, m.Available)
			if plan.Method != tc.want || len(plan.Dependencies) != 2 {
				t.Fatalf("plan = %+v", plan)
			}
			if plan.Dependencies[0].Manager != PackageManager(tc.want) {
				t.Fatalf("dependency order = %+v", plan.Dependencies)
			}
			result := m.Install(pkg)
			if !result.Success || result.Method != tc.want {
				t.Fatalf("result = %+v", result)
			}
			last := stub.Calls[len(stub.Calls)-1]
			if last.Args[len(last.Args)-1] != tc.want+"-main" {
				t.Fatalf("main command = %+v", last)
			}
			before := len(stub.Calls)
			m.DryRun = true
			if !m.Install(pkg).Success || len(stub.Calls) != before {
				t.Fatal("dry-run executed commands")
			}
		})
	}
}

func TestInstalledCacheInvalidation(t *testing.T) {
	ResetInstalledCache()
	t.Cleanup(ResetInstalledCache)
	ids := map[string]bool{}
	mc := managerCmd{bulkList: func(context.Context) map[string]bool { return ids }}
	if isInstalledBulk(context.Background(), "tool", "test-cache", mc) {
		t.Fatal("unexpected initial install")
	}
	ids = map[string]bool{"tool": true}
	if isInstalledBulk(context.Background(), "tool", "test-cache", mc) {
		t.Fatal("cache unexpectedly refreshed")
	}
	ResetInstalledCache()
	if !isInstalledBulk(context.Background(), "tool", "test-cache", mc) {
		t.Fatal("refresh retained stale installed state")
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				ResetInstalledCache()
				isInstalledBulk(context.Background(), "tool", "test-cache", mc)
			}
		}()
	}
	wg.Wait()
	if !reflect.DeepEqual(ids, map[string]bool{"tool": true}) {
		t.Fatal("cache mutated installed IDs")
	}
}

func TestInstallInvalidatesCacheEvenOnDependencyFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		ResetInstalledCache()
		t.Cleanup(ResetInstalledCache)
		ids := map[string]bool{}
		mc := managerCmd{bulkList: func(context.Context) map[string]bool { return ids }}
		if isInstalledBulk(context.Background(), "tool", "test-cache", mc) {
			t.Fatal("unexpected initial install")
		}
		ids = map[string]bool{"tool": true}
		m, stub := newStubManager(t, "linux")
		setAvailable(m, Apt)
		if fail {
			m = m.WithRunner(failingDependencyRunner{StubRunner: stub})
		}
		pkg := Package{Managers: map[PackageManager]ManagerValue{Apt: {PackageName: "tool", Deps: []string{"dep"}}}}
		if result := m.Install(pkg); result.Success == fail {
			t.Fatalf("unexpected install result: %+v", result)
		}
		if !isInstalledBulk(context.Background(), "tool", "test-cache", mc) {
			t.Fatal("install retained stale cache")
		}
	}
}

type failingDependencyRunner struct{ *cmdexec.StubRunner }

func (r failingDependencyRunner) Run(ctx context.Context, name string, args ...string) (cmdexec.Result, error) {
	result, _ := r.StubRunner.Run(ctx, name, args...)
	return result, errors.New("dependency failed")
}
