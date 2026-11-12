package spawner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vbalexr/homelab/containers/forgejo-job-controller/internal/config"
	"github.com/vbalexr/homelab/containers/forgejo-job-controller/internal/forgejo"
	"github.com/vbalexr/homelab/containers/forgejo-job-controller/internal/kube"
	"github.com/vbalexr/homelab/containers/forgejo-job-controller/internal/metrics"
)

// fakeForgejo serves the three endpoints the controller uses.
type fakeForgejo struct {
	mu      sync.Mutex
	jobs    []forgejo.Job
	nextID  int64
	created []string
	deleted []int64
	failNew bool
	srv     *httptest.Server
}

func newFakeForgejo(t *testing.T) *fakeForgejo {
	f := &fakeForgejo{nextID: 100}
	f.srv = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeForgejo) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Header.Get("Authorization") != "token secret" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/orgs/lab/actions/runners/jobs":
		if f.jobs == nil {
			fmt.Fprint(w, "null")
			return
		}
		_ = json.NewEncoder(w).Encode(f.jobs)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/orgs/lab/actions/runners":
		if f.failNew {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		var body struct {
			Name      string `json:"name"`
			Ephemeral bool   `json:"ephemeral"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if !body.Ephemeral {
			http.Error(w, "runner must be ephemeral", http.StatusBadRequest)
			return
		}
		f.nextID++
		f.created = append(f.created, body.Name)
		fmt.Fprintf(w, `{"id":%d,"uuid":"uuid-%d","token":"token-%d"}`, f.nextID, f.nextID, f.nextID)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/repositories/0":
		fmt.Fprint(w, `{"full_name":"lab/demo"}`)
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/v1/orgs/lab/actions/runners/"):
		var id int64
		fmt.Sscanf(strings.TrimPrefix(r.URL.Path, "/api/v1/orgs/lab/actions/runners/"), "%d", &id)
		f.deleted = append(f.deleted, id)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

// fakeKube stores objects by path and supports create, get, list, merge patch and delete.
type fakeKube struct {
	mu      sync.Mutex
	objects map[string]map[string]any
	failJob bool
	srv     *httptest.Server
}

func newFakeKube(t *testing.T) *fakeKube {
	k := &fakeKube{objects: map[string]map[string]any{}}
	k.srv = httptest.NewServer(http.HandlerFunc(k.handle))
	t.Cleanup(k.srv.Close)
	return k
}

func (k *fakeKube) handle(w http.ResponseWriter, r *http.Request) {
	k.mu.Lock()
	defer k.mu.Unlock()
	path := r.URL.Path
	switch r.Method {
	case http.MethodPost:
		if k.failJob && strings.HasSuffix(path, "/jobs") {
			http.Error(w, "quota exceeded", http.StatusForbidden)
			return
		}
		var obj map[string]any
		_ = json.NewDecoder(r.Body).Decode(&obj)
		meta := obj["metadata"].(map[string]any)
		meta["uid"] = "uid-" + meta["name"].(string)
		meta["creationTimestamp"] = time.Now().UTC().Format(time.RFC3339)
		k.objects[path+"/"+meta["name"].(string)] = obj
		_ = json.NewEncoder(w).Encode(obj)
	case http.MethodGet:
		if obj, ok := k.objects[path]; ok {
			_ = json.NewEncoder(w).Encode(obj)
			return
		}
		items := []map[string]any{}
		for p, obj := range k.objects {
			if strings.HasPrefix(p, path+"/") && matches(obj, r.URL.Query().Get("labelSelector")) {
				items = append(items, obj)
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
	case http.MethodPatch:
		obj, ok := k.objects[path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		var patch map[string]any
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &patch)
		merge(obj, patch)
	case http.MethodDelete:
		if _, ok := k.objects[path]; !ok {
			http.NotFound(w, r)
			return
		}
		delete(k.objects, path)
	}
}

func matches(obj map[string]any, selector string) bool {
	if selector == "" {
		return true
	}
	key, value, _ := strings.Cut(selector, "=")
	labels, _ := obj["metadata"].(map[string]any)["labels"].(map[string]any)
	return labels[key] == value
}

func merge(dst, src map[string]any) {
	for key, value := range src {
		if sub, ok := value.(map[string]any); ok {
			if existing, ok := dst[key].(map[string]any); ok {
				merge(existing, sub)
				continue
			}
		}
		dst[key] = value
	}
}

func (k *fakeKube) names(suffix string) []string {
	k.mu.Lock()
	defer k.mu.Unlock()
	var out []string
	for p := range k.objects {
		if strings.Contains(p, suffix+"/") {
			out = append(out, p[strings.LastIndex(p, "/")+1:])
		}
	}
	return out
}

func (k *fakeKube) finish(name string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	obj := k.objects[kube.JobsPath("ci")+"/"+name]
	obj["status"] = map[string]any{"conditions": []any{map[string]any{"type": "Complete", "status": "True"}}}
}

var lastRegistry *metrics.Registry

func setup(t *testing.T, mutate func(*config.Config)) (*Controller, *fakeForgejo, *fakeKube) {
	f, k := newFakeForgejo(t), newFakeKube(t)
	cfg := config.Config{
		ForgejoURL: f.srv.URL, ForgejoRunnerURL: "http://forgejo.test:3000", ForgejoScope: "orgs/lab", ForgejoToken: "secret",
		Namespace: "ci", Labels: map[string]string{"ubuntu-latest": "docker://node:lts", "docker": "docker://node:lts", "native": "docker://graal"},
		Profiles: map[string]config.Profile{"native": {CPU: "8", Memory: "12Gi", Disk: "30Gi", Max: 1}, "large": {CPU: "8", Memory: "16Gi", Disk: "40Gi", Max: 1}}, PriorityClass: "forgejo-job",
		RunnerImage: "runner:13", DindImage: "docker:dind", RunnerConfigMap: "runner-config",
		PollInterval: time.Second, LongRunningAfter: 30 * time.Minute, MaxJobs: 2, JobTimeout: time.Hour, FinishedTTL: 10 * time.Minute,
		JobCPULimit: "2", JobMemoryLimit: "4Gi", JobDiskLimit: "10Gi",
	}
	if mutate != nil {
		mutate(&cfg)
	}
	kc, err := kube.New(kube.Settings{APIURL: k.srv.URL}, http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}
	lastRegistry = &metrics.Registry{}
	c := New(cfg, forgejo.New(f.srv.URL, "orgs/lab", "secret", http.DefaultClient), kc, NewMetrics(lastRegistry))
	return c, f, k
}

func waiting(id int64, runsOn ...string) forgejo.Job {
	return forgejo.Job{ID: id, Handle: fmt.Sprintf("handle-%d", id), Name: fmt.Sprintf("job-%d", id), RunsOn: runsOn, Status: "waiting"}
}

func TestTickSpawnsOnePodPerWaitingJobOnce(t *testing.T) {
	c, f, k := setup(t, nil)
	f.jobs = []forgejo.Job{waiting(1, "ubuntu-latest")}

	for i := 0; i < 3; i++ {
		if err := c.Tick(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if got := len(k.names("/jobs")); got != 1 {
		t.Fatalf("want 1 job after 3 ticks, got %d", got)
	}
	if got := len(k.names("/secrets")); got != 1 {
		t.Fatalf("want 1 secret, got %d", got)
	}
	if len(f.created) != 1 {
		t.Fatalf("want 1 runner created, got %v", f.created)
	}
	name := JobName("handle-1")
	obj := k.objects[kube.JobsPath("ci")+"/"+name]
	spec := obj["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)
	args := spec["containers"].([]any)[0].(map[string]any)["args"].([]any)
	joined := fmt.Sprint(args...)
	for _, want := range []string{"one-job", "--handle", "handle-1", "--wait", "ubuntu-latest:docker://node:lts", "$(RUNNER_UUID)", "-c"} {
		if !strings.Contains(joined, want) {
			t.Errorf("runner args %v miss %q", args, want)
		}
	}
	owner := k.objects[kube.SecretsPath("ci")+"/"+name]["metadata"].(map[string]any)["ownerReferences"]
	if owner == nil {
		t.Error("secret is not owned by the job")
	}
}

func TestTickIgnoresJobsForOtherRunners(t *testing.T) {
	c, f, k := setup(t, nil)
	f.jobs = []forgejo.Job{waiting(1, "windows"), waiting(2, "ubuntu-latest", "gpu"), {ID: 3, Handle: "h3", Status: "running", RunsOn: []string{"ubuntu-latest"}}}
	if err := c.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(k.names("/jobs")) != 0 || len(f.created) != 0 {
		t.Fatal("jobs for unknown labels or already running must not start pods")
	}
}

func TestTickRespectsMaxJobsAndStartsMoreAfterFinish(t *testing.T) {
	c, f, k := setup(t, func(cfg *config.Config) { cfg.MaxJobs = 2 })
	f.jobs = []forgejo.Job{waiting(1, "docker"), waiting(2, "docker"), waiting(3, "docker")}
	if err := c.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := len(k.names("/jobs")); got != 2 {
		t.Fatalf("want 2 jobs at the limit, got %d", got)
	}
	k.finish(JobName("handle-1"))
	if err := c.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := len(k.names("/jobs")); got != 3 {
		t.Fatalf("want 3 jobs once one finished, got %d", got)
	}
}

func TestReapDeletesRunnerOfFinishedJobOnce(t *testing.T) {
	c, f, k := setup(t, nil)
	f.jobs = []forgejo.Job{waiting(1, "docker")}
	_ = c.Tick(context.Background())
	k.finish(JobName("handle-1"))
	f.jobs = nil
	for i := 0; i < 3; i++ {
		_ = c.Tick(context.Background())
	}
	if len(f.deleted) != 1 || f.deleted[0] != 101 {
		t.Fatalf("want runner 101 deleted exactly once, got %v", f.deleted)
	}
}

func TestSpawnRollsBackWhenJobCreationFails(t *testing.T) {
	c, f, k := setup(t, nil)
	k.failJob = true
	f.jobs = []forgejo.Job{waiting(1, "docker")}
	if err := c.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(k.names("/secrets")) != 0 {
		t.Error("secret was left behind")
	}
	if len(f.deleted) != 1 {
		t.Errorf("runner was not deleted again: %v", f.deleted)
	}
}

func TestTickReportsForgejoFailure(t *testing.T) {
	c, f, _ := setup(t, nil)
	f.srv.Close()
	if err := c.Tick(context.Background()); err == nil {
		t.Fatal("want an error when Forgejo is unreachable")
	}
	if c.Healthy() {
		t.Fatal("must not be healthy before a successful poll")
	}
}

func TestManifestIsSafe(t *testing.T) {
	cfg := config.Config{ForgejoRunnerURL: "http://f", RunnerImage: "r", DindImage: "d", JobTimeout: time.Hour, FinishedTTL: time.Minute,
		JobCPULimit: "1", JobMemoryLimit: "1Gi", JobDiskLimit: "1Gi", DindMTU: 1400}
	m := jobManifest(cfg, "fj-0123456789abcdef", waiting(1, "docker"), "lab/demo", "default", config.Profile{CPU: "1", Memory: "1Gi", Disk: "1Gi", Max: 1}, forgejo.Runner{ID: 1}, nil)
	b, _ := json.Marshal(m)
	s := string(b)
	for _, bad := range []string{"tcp://", "2375", "2376", "hostNetwork", "hostPath"} {
		if strings.Contains(s, bad) {
			t.Errorf("manifest must not contain %q", bad)
		}
	}
	for _, want := range []string{`"automountServiceAccountToken":false`, `"restartPolicy":"Always"`, "--mtu=1400", `"backoffLimit":0`, "unix:///run/dind/docker.sock"} {
		if !strings.Contains(s, want) {
			t.Errorf("manifest misses %s", want)
		}
	}
}

func TestFailedPodIsReportedWithCauseAndLogged(t *testing.T) {
	c, f, k := setup(t, nil)
	f.jobs = []forgejo.Job{waiting(1, "docker")}
	_ = c.Tick(context.Background())
	name := JobName("handle-1")

	k.mu.Lock()
	k.objects[kube.JobsPath("ci")+"/"+name]["status"] = map[string]any{"conditions": []any{
		map[string]any{"type": "Failed", "status": "True", "reason": "BackoffLimitExceeded"}}}
	k.objects[kube.PodsPath("ci")+"/"+name+"-abc"] = map[string]any{
		"metadata": map[string]any{"name": name + "-abc", "labels": map[string]any{kube.LabelHandle: handleHash(name)}},
		"status": map[string]any{"phase": "Failed", "containerStatuses": []any{map[string]any{"name": "runner",
			"state": map[string]any{"terminated": map[string]any{"exitCode": 137, "reason": "OOMKilled"}}}}},
	}
	k.objects[kube.EventsPath("ci")+"/e1"] = map[string]any{"type": "Warning", "reason": "Evicted", "message": "node pressure", "lastTimestamp": "2026-11-11T10:00:00Z"}
	k.mu.Unlock()

	f.jobs = nil
	_ = c.Tick(context.Background())

	report := c.diagnose(context.Background(), kube.Job{Metadata: kube.ObjectMeta{Name: name, Labels: map[string]string{kube.LabelHandle: handleHash(name)}},
		Status: kube.JobStatus{Conditions: []kube.Condition{{Type: "Failed", Status: "True", Reason: "BackoffLimitExceeded"}}}})
	if report.Result != "failed" || report.Cause != "oom-killed-runner" {
		t.Fatalf("want failed/oom-killed-runner, got %+v", report)
	}
	if len(report.Events) != 1 || !strings.Contains(report.Events[0], "Evicted") {
		t.Errorf("events not collected: %v", report.Events)
	}
	if !strings.Contains(strings.Join(report.Containers, ";"), "exit=137 reason=OOMKilled") {
		t.Errorf("container states not collected: %v", report.Containers)
	}
}

func TestDeadlineExceededWithoutPodIsClassified(t *testing.T) {
	c, _, _ := setup(t, nil)
	r := c.diagnose(context.Background(), kube.Job{Metadata: kube.ObjectMeta{Labels: map[string]string{kube.LabelHandle: "none"}},
		Status: kube.JobStatus{Conditions: []kube.Condition{{Type: "Failed", Status: "True", Reason: "DeadlineExceeded"}}}})
	if r.Result != "failed" || r.Cause != "deadlineexceeded" {
		t.Fatalf("got %+v", r)
	}
}

func metricsText() string {
	rec := httptest.NewRecorder()
	lastRegistry.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	return rec.Body.String()
}

func TestLongRunningPodIsFlaggedOnceAndClearsWhenFinished(t *testing.T) {
	c, f, k := setup(t, nil)
	f.jobs = []forgejo.Job{waiting(1, "docker")}
	_ = c.Tick(context.Background())
	name := JobName("handle-1")

	_ = c.Tick(context.Background())
	if out := metricsText(); !strings.Contains(out, "fjc_long_running_jobs 0") {
		t.Fatalf("a new pod must not be long running:\n%s", out)
	}

	c.now = func() time.Time { return time.Now().Add(time.Hour) }
	_ = c.Tick(context.Background())
	_ = c.Tick(context.Background())
	out := metricsText()
	if !strings.Contains(out, "fjc_long_running_jobs 1") || !strings.Contains(out, "fjc_oldest_active_job_seconds 3") {
		t.Fatalf("pod older than the limit must be counted:\n%s", out)
	}
	if !c.warned[name] {
		t.Fatal("the pod must be remembered as warned, so it is logged only once")
	}

	k.finish(name)
	_ = c.Tick(context.Background())
	if out := metricsText(); !strings.Contains(out, "fjc_long_running_jobs 0") || c.warned[name] {
		t.Fatalf("a finished pod must no longer count:\n%s", out)
	}
}

func TestNativeJobGetsItsProfileAndNeverRunsTwoAtOnce(t *testing.T) {
	c, f, k := setup(t, func(cfg *config.Config) { cfg.MaxJobs = 3 })
	f.jobs = []forgejo.Job{waiting(1, "native"), waiting(2, "native"), waiting(3, "docker")}
	if err := c.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := len(k.names("/jobs")); got != 2 {
		t.Fatalf("want the first native job and the docker job, got %d jobs", got)
	}
	native := k.objects[kube.JobsPath("ci")+"/"+JobName("handle-1")]
	b, _ := json.Marshal(native)
	s := string(b)
	for _, want := range []string{`"memory":"12Gi"`, `"cpu":"8"`, `"sizeLimit":"30Gi"`, `"priorityClassName":"forgejo-job"`, `"forgejo.vbalex.com/profile":"native"`} {
		if !strings.Contains(s, want) {
			t.Errorf("native job misses %s", want)
		}
	}
	if _, ok := k.objects[kube.JobsPath("ci")+"/"+JobName("handle-2")]; ok {
		t.Fatal("the second native job must wait while one is running")
	}
	other := k.objects[kube.JobsPath("ci")+"/"+JobName("handle-3")]
	b, _ = json.Marshal(other)
	if !strings.Contains(string(b), `"memory":"4Gi"`) || !strings.Contains(string(b), `"forgejo.vbalex.com/profile":"default"`) {
		t.Errorf("a docker job must keep the default profile: %s", b)
	}

	k.finish(JobName("handle-1"))
	if err := c.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := k.objects[kube.JobsPath("ci")+"/"+JobName("handle-2")]; !ok {
		t.Fatal("the waiting native job must start once the first one finished")
	}
}

func TestMemoryRequestEqualsLimit(t *testing.T) {
	cfg := config.Config{ForgejoRunnerURL: "http://f", RunnerImage: "r", DindImage: "d", JobTimeout: time.Hour, FinishedTTL: time.Minute}
	m := jobManifest(cfg, "fj-0123456789abcdef", waiting(1, "native"), "", "native", config.Profile{CPU: "8", Memory: "12Gi", Disk: "30Gi", Max: 1}, forgejo.Runner{ID: 1}, nil)
	b, _ := json.Marshal(m)
	if strings.Count(string(b), `"memory":"12Gi"`) != 2 {
		t.Fatalf("dind needs the memory both as request and as limit: %s", b)
	}
}

func TestSizeLabelBorrowsTheImageOfTheOtherLabel(t *testing.T) {
	c, f, k := setup(t, nil)
	f.jobs = []forgejo.Job{waiting(1, "ubuntu-latest", "large"), waiting(2, "large")}
	if err := c.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	obj, ok := k.objects[kube.JobsPath("ci")+"/"+JobName("handle-1")]
	if !ok {
		t.Fatal("a job with an image label and a size label must start")
	}
	b, _ := json.Marshal(obj)
	s := string(b)
	for _, want := range []string{`"memory":"16Gi"`, "large:docker://node:lts", "ubuntu-latest:docker://node:lts", `"forgejo.vbalex.com/profile":"large"`} {
		if !strings.Contains(s, want) {
			t.Errorf("job misses %s", want)
		}
	}
	if _, ok := k.objects[kube.JobsPath("ci")+"/"+JobName("handle-2")]; ok {
		t.Fatal("a size label alone has no image, so that job must not start")
	}
}

func TestSizeLabelAloneUsesTheDefaultLabelImage(t *testing.T) {
	c, f, k := setup(t, func(cfg *config.Config) { cfg.DefaultLabel = "ubuntu-latest" })
	f.jobs = []forgejo.Job{waiting(1, "large")}
	if err := c.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	obj, ok := k.objects[kube.JobsPath("ci")+"/"+JobName("handle-1")]
	if !ok {
		t.Fatal("runs-on: large must start when DEFAULT_LABEL gives it an image")
	}
	b, _ := json.Marshal(obj)
	if !strings.Contains(string(b), "large:docker://node:lts") || !strings.Contains(string(b), `"memory":"16Gi"`) {
		t.Errorf("wrong image or size: %s", b)
	}
}
