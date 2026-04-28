package cliruntime

import (
	"context"
	"errors"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const imageBuildWorkflowKind = "cli-runtime-image-build"

var ErrBuildInProgress = errors.New("cli runtime image build already in progress")

type ImageBuildJobRunner struct {
	client    ctrlclient.Client
	namespace string
}

func NewImageBuildJobRunner(client ctrlclient.Client, namespace string) (*ImageBuildJobRunner, error) {
	if client == nil {
		return nil, fmt.Errorf("platformk8s/cliruntime: image build job client is nil")
	}
	if namespace == "" {
		return nil, fmt.Errorf("platformk8s/cliruntime: image build namespace is empty")
	}
	return &ImageBuildJobRunner{client: client, namespace: namespace}, nil
}

func (r *ImageBuildJobRunner) RunImageBuild(ctx context.Context, request ImageBuildRequest) error {
	name := imageBuildJobName(request)
	if active, err := r.activeBuildExists(ctx, name); err != nil || active {
		if err != nil {
			return err
		}
		return ErrBuildInProgress
	}
	job := r.jobFor(request, name)
	existing := &batchv1.Job{}
	err := r.client.Get(ctx, types.NamespacedName{Namespace: r.namespace, Name: name}, existing)
	if apierrors.IsNotFound(err) {
		err = r.client.Create(ctx, job)
	}
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("platformk8s/cliruntime: create image build job: %w", err)
	}
	return r.wait(ctx, name)
}

func (r *ImageBuildJobRunner) activeBuildExists(ctx context.Context, name string) (bool, error) {
	list := &batchv1.JobList{}
	if err := r.client.List(ctx, list, ctrlclient.InNamespace(r.namespace), ctrlclient.MatchingLabels{
		"platform.code-code.internal/workflow-kind": imageBuildWorkflowKind,
	}); err != nil {
		return false, fmt.Errorf("platformk8s/cliruntime: list image build jobs: %w", err)
	}
	for i := range list.Items {
		job := &list.Items[i]
		if job.Name != name && jobActive(job) {
			return true, nil
		}
	}
	return false, nil
}

func (r *ImageBuildJobRunner) wait(ctx context.Context, name string) error {
	ctx, cancel := context.WithTimeout(ctx, time.Hour)
	defer cancel()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		job := &batchv1.Job{}
		if err := r.client.Get(ctx, types.NamespacedName{Namespace: r.namespace, Name: name}, job); err != nil {
			return err
		}
		if job.Status.Succeeded > 0 {
			return nil
		}
		if jobFailed(job) {
			return fmt.Errorf("platformk8s/cliruntime: image build job %q failed", name)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (r *ImageBuildJobRunner) jobFor(request ImageBuildRequest, name string) *batchv1.Job {
	labels := map[string]string{
		"code-code.internal/runtime":                imageBuildWorkflowKind,
		"platform.code-code.internal/workflow-kind": imageBuildWorkflowKind,
		"platform.code-code.internal/cli-id":        dnsLabel(request.CLIID),
		"platform.code-code.internal/build-target":  dnsLabel(request.BuildTarget),
	}
	optional := true
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: r.namespace, Labels: labels},
		Spec: batchv1.JobSpec{
			BackoffLimit:            int32Ptr(1),
			ActiveDeadlineSeconds:   int64Ptr(3600),
			TTLSecondsAfterFinished: int32Ptr(86400),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					ServiceAccountName: "cli-runtime-image-build",
					RestartPolicy:      corev1.RestartPolicyNever,
					SecurityContext: &corev1.PodSecurityContext{
						FSGroup:             int64Ptr(1000),
						FSGroupChangePolicy: fsGroupChangePolicyPtr(corev1.FSGroupChangeOnRootMismatch),
					},
					Volumes: []corev1.Volume{
						{Name: "registry-auth", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "cli-runtime-image-build-registry-auth", Optional: &optional}}},
						{Name: "buildkitd", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
						{Name: "egress-trust-bundle", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "code-code-egress-trust-bundle"}}}},
					},
					InitContainers: []corev1.Container{buildAndPushContainer(request)},
					Containers:     []corev1.Container{pruneOldTagsContainer(request)},
				},
			},
		},
	}
}

func buildAndPushContainer(request ImageBuildRequest) corev1.Container {
	return corev1.Container{
		Name:            "build-and-push",
		Image:           "moby/buildkit:v0.29.0-rootless",
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"/bin/sh", "-eu", "-c"},
		Args:            []string{buildAndPushScript},
		Env:             append(imageBuildEnv(request), corev1.EnvVar{Name: "BUILDKITD_FLAGS", Value: "--oci-worker-no-process-sandbox"}),
		VolumeMounts: []corev1.VolumeMount{
			{Name: "registry-auth", MountPath: "/registry-auth", ReadOnly: true},
			{Name: "buildkitd", MountPath: "/home/user/.local/share/buildkit"},
			{Name: "egress-trust-bundle", MountPath: "/var/run/code-code-egress-trust", ReadOnly: true},
		},
		SecurityContext: imageBuildSecurityContext(),
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m"), corev1.ResourceMemory: resource.MustParse("1Gi")},
			Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2"), corev1.ResourceMemory: resource.MustParse("4Gi")},
		},
	}
}

func pruneOldTagsContainer(request ImageBuildRequest) corev1.Container {
	return corev1.Container{
		Name:            "prune-old-tags",
		Image:           "ghcr.io/regclient/regctl:v0.11.3-alpine",
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"/bin/sh", "-eu", "-c"},
		Args:            []string{pruneOldTagsScript},
		Env: append(imageBuildEnv(request),
			corev1.EnvVar{Name: "RETENTION_KEEP_TAGS", Value: "2"},
			corev1.EnvVar{Name: "HOME", Value: "/tmp"},
		),
		VolumeMounts: []corev1.VolumeMount{
			{Name: "registry-auth", MountPath: "/registry-auth", ReadOnly: true},
			{Name: "egress-trust-bundle", MountPath: "/var/run/code-code-egress-trust", ReadOnly: true},
		},
		SecurityContext: imageBuildSecurityContext(),
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("50m"), corev1.ResourceMemory: resource.MustParse("64Mi")},
			Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("250m"), corev1.ResourceMemory: resource.MustParse("256Mi")},
		},
	}
}

func imageBuildEnv(request ImageBuildRequest) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "BUILD_TARGET", Value: request.BuildTarget},
		{Name: "SOURCE_CONTEXT", Value: request.SourceContext},
		{Name: "SOURCE_REVISION", Value: request.SourceRevision},
		{Name: "IMAGE_REPOSITORY", Value: request.ImageRepository},
		{Name: "IMAGE", Value: request.Image},
		{Name: "CLI_VERSION", Value: request.CLIVersion},
		{Name: "BUILD_NPM_REGISTRY", Value: ""},
		{Name: "DOCKER_CONFIG", Value: "/registry-auth"},
		{Name: "SSL_CERT_FILE", Value: "/var/run/code-code-egress-trust/ca-certificates.crt"},
		{Name: "CURL_CA_BUNDLE", Value: "/var/run/code-code-egress-trust/ca-certificates.crt"},
		{Name: "GIT_SSL_CAINFO", Value: "/var/run/code-code-egress-trust/ca-certificates.crt"},
		{Name: "NODE_EXTRA_CA_CERTS", Value: "/var/run/code-code-egress-trust/ca-certificates.crt"},
	}
}
