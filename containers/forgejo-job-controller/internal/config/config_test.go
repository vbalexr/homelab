package config

import "testing"

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func base() map[string]string {
	return map[string]string{
		"FORGEJO_URL": "http://f:3000/", "FORGEJO_SCOPE": "/orgs/lab/", "FORGEJO_TOKEN": "t", "NAMESPACE": "ci",
		"RUNNER_IMAGE": "r:1", "DIND_IMAGE": "d:1", "LABELS": "ubuntu-latest=docker://node:lts, docker=docker://x",
	}
}

func TestLoadDefaultsAndTrimming(t *testing.T) {
	c, err := Load(env(base()))
	if err != nil {
		t.Fatal(err)
	}
	if c.ForgejoURL != "http://f:3000" || c.ForgejoScope != "orgs/lab" || c.ForgejoRunnerURL != c.ForgejoURL {
		t.Errorf("unexpected urls: %+v", c)
	}
	if c.Labels["docker"] != "docker://x" || len(c.Labels) != 2 || c.MaxJobs != 2 {
		t.Errorf("unexpected labels or defaults: %+v", c)
	}
}

func TestLoadRejectsBadInput(t *testing.T) {
	for name, mutate := range map[string]func(map[string]string){
		"no url":        func(m map[string]string) { delete(m, "FORGEJO_URL") },
		"no token":      func(m map[string]string) { delete(m, "FORGEJO_TOKEN") },
		"unpinned":      func(m map[string]string) { delete(m, "RUNNER_IMAGE") },
		"bad label":     func(m map[string]string) { m["LABELS"] = "nolabelimage" },
		"zero max jobs": func(m map[string]string) { m["MAX_JOBS"] = "0" },
		"bad duration":  func(m map[string]string) { m["POLL_INTERVAL"] = "soon" },
	} {
		m := base()
		mutate(m)
		if _, err := Load(env(m)); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestProfiles(t *testing.T) {
	m := base()
	m["LABELS"] += ",native=docker://graal"
	m["PROFILES"] = "native:cpu=8,memory=12Gi,disk=30Gi,max=1"
	c, err := Load(env(m))
	if err != nil {
		t.Fatal(err)
	}
	name, p := c.ProfileFor([]string{"ubuntu-latest", "native"})
	if name != "native" || p.Memory != "12Gi" || p.Max != 1 || p.Disk != "30Gi" {
		t.Fatalf("got %s %+v", name, p)
	}
	name, p = c.ProfileFor([]string{"docker"})
	if name != DefaultProfile || p.Memory != "4Gi" || p.Max != c.MaxJobs {
		t.Fatalf("a job without a profile label must get the default: %s %+v", name, p)
	}
	m["DEFAULT_LABEL"] = "nope"
	if _, err := Load(env(m)); err == nil {
		t.Error("DEFAULT_LABEL must be one of the labels")
	}
	delete(m, "DEFAULT_LABEL")
	for _, bad := range []string{"native:cpu=8", "native:cpu=8,memory=1Gi,max=0", "native:cpu=8,memory=1Gi,color=red", "default:cpu=1,memory=1Gi"} {
		m["PROFILES"] = bad
		if _, err := Load(env(m)); err == nil {
			t.Errorf("PROFILES %q must be rejected", bad)
		}
	}
}
