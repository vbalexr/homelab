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
