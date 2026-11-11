package spawner

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/vbalexr/homelab/containers/forgejo-job-controller/internal/kube"
)

// Report says how a finished job pod ended and why. Cause is a short stable label (it is also a metric label).
type Report struct {
	Result     string // succeeded or failed
	Cause      string
	JobReason  string // reason of the Job condition, e.g. DeadlineExceeded
	PodPhase   string
	Containers []string // "runner: terminated exit=137 reason=OOMKilled"
	Events     []string // recent warning events of the pod
}

// diagnose reads the Job conditions, the pod container states and the pod warning events.
func (c *Controller) diagnose(ctx context.Context, job kube.Job) Report {
	r := Report{Result: "succeeded", Cause: "none"}
	cond, _ := job.FinishedCondition()
	if cond.Type == "Failed" {
		r.Result, r.JobReason = "failed", cond.Reason
		r.Cause = strings.ToLower(cond.Reason) // deadlineexceeded, backofflimitexceeded
	}

	pod, ok := c.podOf(ctx, job)
	if !ok {
		return r
	}
	r.PodPhase = pod.Status.Phase

	statuses := append(append([]kube.ContainerStatus{}, pod.Status.InitContainerStatuses...), pod.Status.ContainerStatuses...)
	for _, s := range statuses {
		r.Containers = append(r.Containers, describe(s))
		if cause := causeOf(s); cause != "" && (r.Cause == "none" || r.Cause == "deadlineexceeded" || r.Cause == "backofflimitexceeded") {
			r.Result, r.Cause = "failed", cause
		}
	}

	var events kube.EventList
	q := url.Values{"fieldSelector": {"involvedObject.name=" + pod.Metadata.Name}}
	if err := c.kube.List(ctx, kube.EventsPath(c.cfg.Namespace), q, &events); err == nil {
		sort.Slice(events.Items, func(a, b int) bool { return events.Items[a].LastTimestamp < events.Items[b].LastTimestamp })
		for _, e := range events.Items {
			if e.Type != "Warning" {
				continue
			}
			r.Events = append(r.Events, fmt.Sprintf("%s: %s", e.Reason, e.Message))
			if r.Cause == "deadlineexceeded" && e.Reason == "FailedScheduling" {
				r.Cause = "unschedulable"
			}
		}
		if len(r.Events) > 5 {
			r.Events = r.Events[len(r.Events)-5:]
		}
	}
	return r
}

// podOf returns the pod of a Job (there is one, because backoffLimit is 0).
func (c *Controller) podOf(ctx context.Context, job kube.Job) (kube.Pod, bool) {
	var pods kube.PodList
	sel := url.Values{"labelSelector": {kube.LabelHandle + "=" + job.Metadata.Labels[kube.LabelHandle]}}
	if err := c.kube.List(ctx, kube.PodsPath(c.cfg.Namespace), sel, &pods); err != nil || len(pods.Items) == 0 {
		return kube.Pod{}, false
	}
	return pods.Items[0], true
}

// containerStates describes the init and main containers of a pod, one string each.
func containerStates(pod kube.Pod) []string {
	var out []string
	for _, s := range pod.Status.InitContainerStatuses {
		out = append(out, describe(s))
	}
	for _, s := range pod.Status.ContainerStatuses {
		out = append(out, describe(s))
	}
	return out
}

func describe(s kube.ContainerStatus) string {
	switch {
	case s.State.Terminated != nil:
		t := s.State.Terminated
		return fmt.Sprintf("%s: terminated exit=%d reason=%s message=%q", s.Name, t.ExitCode, t.Reason, t.Message)
	case s.State.Waiting != nil:
		return fmt.Sprintf("%s: waiting reason=%s message=%q", s.Name, s.State.Waiting.Reason, s.State.Waiting.Message)
	default:
		return s.Name + ": running"
	}
}

// causeOf classifies one container status. It returns "" when the container looks fine.
func causeOf(s kube.ContainerStatus) string {
	if t := s.State.Terminated; t != nil {
		switch {
		case t.Reason == "OOMKilled":
			return "oom-killed-" + s.Name
		case t.ExitCode != 0 && s.Name == "runner":
			return fmt.Sprintf("runner-exit-%d", t.ExitCode)
		case t.ExitCode != 0 && s.Name == "dind":
			return "dind-failed"
		}
	}
	if w := s.State.Waiting; w != nil {
		switch w.Reason {
		case "ErrImagePull", "ImagePullBackOff", "InvalidImageName":
			return "image-pull"
		case "CreateContainerConfigError", "CreateContainerError":
			return "container-config"
		}
	}
	return ""
}
