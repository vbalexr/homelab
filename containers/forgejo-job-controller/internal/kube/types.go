package kube

import (
	"fmt"
	"time"
)

// Annotation and label keys the controller puts on the objects it creates.
const (
	LabelManagedBy    = "app.kubernetes.io/managed-by"
	LabelHandle       = "forgejo.vbalex.com/handle"
	AnnotationHandle  = "forgejo.vbalex.com/handle"
	AnnotationRunner  = "forgejo.vbalex.com/runner-id"
	AnnotationCleaned = "forgejo.vbalex.com/runner-cleaned"
	AnnotationJobName = "forgejo.vbalex.com/job-name"
	AnnotationJobID   = "forgejo.vbalex.com/job-id"
	AnnotationRepo    = "forgejo.vbalex.com/repo"
)

// ObjectMeta is the part of the object metadata the controller reads.
type ObjectMeta struct {
	Name              string            `json:"name"`
	UID               string            `json:"uid"`
	CreationTimestamp time.Time         `json:"creationTimestamp"`
	Labels            map[string]string `json:"labels"`
	Annotations       map[string]string `json:"annotations"`
}

// Condition is one entry of a Job status.
type Condition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

// JobStatus is the part of a Job status the controller reads.
type JobStatus struct {
	Active     int         `json:"active"`
	Conditions []Condition `json:"conditions"`
}

// Job is a batch/v1 Job.
type Job struct {
	Metadata ObjectMeta `json:"metadata"`
	Status   JobStatus  `json:"status"`
}

// JobList is the answer of a Job list call.
type JobList struct {
	Items []Job `json:"items"`
}

// Finished reports whether the Job completed or failed.
func (j Job) Finished() bool {
	for _, c := range j.Status.Conditions {
		if (c.Type == "Complete" || c.Type == "Failed") && c.Status == "True" {
			return true
		}
	}
	return false
}

// JobsPath is the collection path of the Jobs of a namespace.
func JobsPath(namespace string) string {
	return fmt.Sprintf("/apis/batch/v1/namespaces/%s/jobs", namespace)
}

// SecretsPath is the collection path of the Secrets of a namespace.
func SecretsPath(namespace string) string {
	return fmt.Sprintf("/api/v1/namespaces/%s/secrets", namespace)
}

// FinishedCondition returns the Complete or Failed condition of a finished Job.
func (j Job) FinishedCondition() (Condition, bool) {
	for _, c := range j.Status.Conditions {
		if (c.Type == "Complete" || c.Type == "Failed") && c.Status == "True" {
			return c, true
		}
	}
	return Condition{}, false
}

// ContainerState is the part of a container state the controller reads.
type ContainerState struct {
	Waiting    *struct{ Reason, Message string } `json:"waiting"`
	Terminated *struct {
		ExitCode int    `json:"exitCode"`
		Signal   int    `json:"signal"`
		Reason   string `json:"reason"`
		Message  string `json:"message"`
	} `json:"terminated"`
}

// ContainerStatus is the status of one container of a pod.
type ContainerStatus struct {
	Name         string         `json:"name"`
	State        ContainerState `json:"state"`
	LastState    ContainerState `json:"lastState"`
	RestartCount int            `json:"restartCount"`
}

// Pod is the part of a pod the controller reads when a Job finished.
type Pod struct {
	Metadata ObjectMeta `json:"metadata"`
	Status   struct {
		Phase                 string            `json:"phase"`
		Reason                string            `json:"reason"`
		Message               string            `json:"message"`
		InitContainerStatuses []ContainerStatus `json:"initContainerStatuses"`
		ContainerStatuses     []ContainerStatus `json:"containerStatuses"`
	} `json:"status"`
}

// PodList is the answer of a pod list call.
type PodList struct {
	Items []Pod `json:"items"`
}

// Event is a Kubernetes event about an object.
type Event struct {
	Type          string `json:"type"`
	Reason        string `json:"reason"`
	Message       string `json:"message"`
	Count         int    `json:"count"`
	LastTimestamp string `json:"lastTimestamp"`
}

// EventList is the answer of an event list call.
type EventList struct {
	Items []Event `json:"items"`
}

// PodsPath is the collection path of the pods of a namespace.
func PodsPath(namespace string) string {
	return fmt.Sprintf("/api/v1/namespaces/%s/pods", namespace)
}

// EventsPath is the collection path of the events of a namespace.
func EventsPath(namespace string) string {
	return fmt.Sprintf("/api/v1/namespaces/%s/events", namespace)
}
