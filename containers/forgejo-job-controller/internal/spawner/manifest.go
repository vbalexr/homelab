package spawner

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/vbalexr/homelab/containers/forgejo-job-controller/internal/config"
	"github.com/vbalexr/homelab/containers/forgejo-job-controller/internal/forgejo"
	"github.com/vbalexr/homelab/containers/forgejo-job-controller/internal/kube"
)

const (
	managedBy = "forgejo-job-controller"
	socketDir = "/run/dind"
	socket    = "unix://" + socketDir + "/docker.sock"
	userID    = 1000
)

// JobName is the stable name of the Kubernetes Job (and Secret) for a Forgejo job handle. It doubles as the dedupe
// key: a handle that already has a Job is never started twice.
func JobName(handle string) string {
	sum := sha256.Sum256([]byte(handle))
	return "fj-" + hex.EncodeToString(sum[:])[:16]
}

func handleHash(name string) string { return name[len("fj-"):] }

func labelsFor(name string) map[string]string {
	return map[string]string{
		kube.LabelManagedBy:      managedBy,
		kube.LabelHandle:         handleHash(name),
		"app.kubernetes.io/name": "forgejo-job",
	}
}

// labelFlags maps the runs-on labels of a job to --label flags. It returns false when a label has no image, which
// means the job is meant for another runner.
func labelFlags(labels map[string]string, runsOn []string) ([]string, bool) {
	if len(runsOn) == 0 {
		return nil, false
	}
	names := append([]string(nil), runsOn...)
	sort.Strings(names)
	var flags []string
	for _, name := range names {
		image, ok := labels[name]
		if !ok {
			return nil, false
		}
		flags = append(flags, "--label", name+":"+image)
	}
	return flags, true
}

// secretManifest holds the token and UUID of the ephemeral runner of one job.
func secretManifest(name string, runner forgejo.Runner) map[string]any {
	return map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata":   map[string]any{"name": name, "labels": labelsFor(name)},
		"type":       "Opaque",
		"stringData": map[string]string{"token": runner.Token, "uuid": runner.UUID},
	}
}

// jobManifest is one pod that runs exactly one Forgejo job: a Docker-in-Docker native sidecar on a Unix socket (no
// TCP listener) and the runner started with one-job for this handle.
func jobManifest(cfg config.Config, name string, job forgejo.Job, repo string, runner forgejo.Runner, flags []string) map[string]any {
	runnerArgs := []string{}
	if cfg.RunnerConfigMap != "" {
		runnerArgs = append(runnerArgs, "-c", "/config/config.yml")
	}
	runnerArgs = append(runnerArgs, "one-job",
		"--url", cfg.ForgejoRunnerURL,
		"--uuid", "$(RUNNER_UUID)",
		"--token-url", "file:/secret/token",
		"--handle", job.Handle,
		"--wait")
	runnerArgs = append(runnerArgs, flags...)

	dindArgs := []string{"dockerd", "--host=" + socket, fmt.Sprintf("--group=%d", userID)}
	if cfg.DindMTU > 0 {
		dindArgs = append(dindArgs, fmt.Sprintf("--mtu=%d", cfg.DindMTU))
	}

	volumes := []map[string]any{
		{"name": "dind-socket", "emptyDir": map[string]any{}},
		{"name": "docker-data", "emptyDir": map[string]any{"sizeLimit": cfg.JobDiskLimit}},
		{"name": "data", "emptyDir": map[string]any{}},
		{"name": "tmp", "emptyDir": map[string]any{}},
		{"name": "runner-secret", "secret": map[string]any{"secretName": name, "defaultMode": 0o440}},
	}
	runnerMounts := []map[string]any{
		{"name": "dind-socket", "mountPath": socketDir},
		{"name": "data", "mountPath": "/data"},
		{"name": "tmp", "mountPath": "/tmp"},
		{"name": "runner-secret", "mountPath": "/secret", "readOnly": true},
	}
	if cfg.RunnerConfigMap != "" {
		volumes = append(volumes, map[string]any{"name": "runner-config", "configMap": map[string]any{"name": cfg.RunnerConfigMap}})
		runnerMounts = append(runnerMounts, map[string]any{"name": "runner-config", "mountPath": "/config", "readOnly": true})
	}

	dind := map[string]any{
		"name":            "dind",
		"image":           cfg.DindImage,
		"restartPolicy":   "Always", // native sidecar: starts first, is ready before the runner, stops after it
		"args":            dindArgs,
		"securityContext": map[string]any{"privileged": true},
		"startupProbe": map[string]any{
			"exec":             map[string]any{"command": []string{"docker", "-H", socket, "info"}},
			"periodSeconds":    2,
			"failureThreshold": 90,
		},
		"resources": map[string]any{
			"requests": map[string]any{"cpu": "250m", "memory": "512Mi"},
			"limits":   map[string]any{"cpu": cfg.JobCPULimit, "memory": cfg.JobMemoryLimit},
		},
		"volumeMounts": []map[string]any{
			{"name": "dind-socket", "mountPath": socketDir},
			{"name": "docker-data", "mountPath": "/var/lib/docker"},
		},
	}

	runnerContainer := map[string]any{
		"name":       "runner",
		"image":      cfg.RunnerImage,
		"command":    []string{"forgejo-runner"},
		"args":       runnerArgs,
		"workingDir": "/data",
		"env": []map[string]any{
			{"name": "DOCKER_HOST", "value": socket},
			{"name": "HOME", "value": "/data"},
			{"name": "RUNNER_UUID", "valueFrom": map[string]any{"secretKeyRef": map[string]any{"name": name, "key": "uuid"}}},
		},
		"securityContext": map[string]any{
			"runAsNonRoot":             true,
			"runAsUser":                userID,
			"runAsGroup":               userID,
			"allowPrivilegeEscalation": false,
			"readOnlyRootFilesystem":   true,
			"capabilities":             map[string]any{"drop": []string{"ALL"}},
		},
		"resources": map[string]any{
			"requests": map[string]any{"cpu": "100m", "memory": "128Mi"},
			"limits":   map[string]any{"memory": "512Mi"},
		},
		"volumeMounts": runnerMounts,
	}

	return map[string]any{
		"apiVersion": "batch/v1",
		"kind":       "Job",
		"metadata": map[string]any{
			"name":   name,
			"labels": labelsFor(name),
			"annotations": map[string]string{
				kube.AnnotationHandle:  job.Handle,
				kube.AnnotationRunner:  fmt.Sprint(runner.ID),
				kube.AnnotationJobName: job.Name,
				kube.AnnotationJobID:   fmt.Sprint(job.ID),
				kube.AnnotationRepo:    repo,
			},
		},
		"spec": map[string]any{
			"backoffLimit":            0,
			"activeDeadlineSeconds":   int64(cfg.JobTimeout.Seconds()),
			"ttlSecondsAfterFinished": int64(cfg.FinishedTTL.Seconds()),
			"template": map[string]any{
				"metadata": map[string]any{"labels": labelsFor(name)},
				"spec": map[string]any{
					"restartPolicy":                 "Never",
					"automountServiceAccountToken":  false,
					"terminationGracePeriodSeconds": 30,
					"securityContext": map[string]any{
						"fsGroup":        userID,
						"seccompProfile": map[string]any{"type": "RuntimeDefault"},
					},
					"initContainers": []map[string]any{dind},
					"containers":     []map[string]any{runnerContainer},
					"volumes":        volumes,
				},
			},
		},
	}
}
