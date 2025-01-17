// Copyright Contributors to the Open Cluster Management project

package foundation

import (
	"reflect"

	v1alpha1 "github.com/open-cluster-management/backplane-operator/api/v1alpha1"
	"github.com/open-cluster-management/backplane-operator/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// ImageKey used by multicoud manager deployments
const ImageKey = "multicloud_manager"

// RegistrationImageKey used by registration deployments
const RegistrationImageKey = "registration"

// WorkImageKey used by work deployments
const WorkImageKey = "work"

// PlacementImageKey used by placement deployments
const PlacementImageKey = "placement"

// ServiceAccount used by ocm deployments
const ServiceAccount = "ocm-foundation-sa"

// Image returns image reference for multicloud-manager
func Image(overrides map[string]string) string {
	return overrides[ImageKey]
}

// RegistrationImage ...
func RegistrationImage(overrides map[string]string) string {
	return overrides[RegistrationImageKey]
}

// WorkImage ...
func WorkImage(overrides map[string]string) string {
	return overrides[WorkImageKey]
}

// PlacementImage ...
func PlacementImage(overrides map[string]string) string {
	return overrides[PlacementImageKey]
}

func defaultLabels(app string) map[string]string {
	return map[string]string{
		"app":                       app,
		"ocm-antiaffinity-selector": app,
	}
}

func defaultTolerations() []corev1.Toleration {
	return []corev1.Toleration{
		{
			Effect:   "NoSchedule",
			Key:      "node-role.kubernetes.io/infra",
			Operator: "Exists",
		},
	}
}

// ValidateDeployment returns a deep copy of the deployment with the desired spec based on the MultiClusterEngine spec.
// Returns true if an update is needed to reconcile differences with the current spec.
func ValidateDeployment(m *v1alpha1.MultiClusterEngine, overrides map[string]string, expected, dep *appsv1.Deployment) (*appsv1.Deployment, bool) {
	var log = logf.Log.WithValues("Deployment.Namespace", dep.GetNamespace(), "Deployment.Name", dep.GetName())
	found := dep.DeepCopy()

	pod := &found.Spec.Template.Spec
	container := &found.Spec.Template.Spec.Containers[0]
	needsUpdate := false

	// verify image pull secret
	// if m.Spec.ImagePullSecret != "" {
	// 	ps := corev1.LocalObjectReference{Name: m.Spec.ImagePullSecret}
	// 	if !utils.ContainsPullSecret(pod.ImagePullSecrets, ps) {
	// 		log.Info("Enforcing imagePullSecret from CR spec")
	// 		pod.ImagePullSecrets = append(pod.ImagePullSecrets, ps)
	// 		needsUpdate = true
	// 	}
	// }

	// verify image repository and suffix
	if container.Image != Image(overrides) {
		log.Info("Enforcing image repo and suffix from CR spec")
		container.Image = Image(overrides)
		needsUpdate = true
	}

	// verify image pull policy
	// if container.ImagePullPolicy != utils.GetImagePullPolicy(m) {
	// 	log.Info("Enforcing imagePullPolicy from CR spec")
	// 	container.ImagePullPolicy = utils.GetImagePullPolicy(m)
	// 	needsUpdate = true
	// }

	// verify node selectors
	// desiredSelectors := m.Spec.NodeSelector
	// if !utils.ContainsMap(pod.NodeSelector, desiredSelectors) {
	// 	log.Info("Enforcing node selectors from CR spec")
	// 	pod.NodeSelector = desiredSelectors
	// 	needsUpdate = true
	// }
	// // verify replica count
	// if *found.Spec.Replicas != getReplicaCount(m) {
	// 	log.Info("Enforcing number of replicas")
	// 	replicas := getReplicaCount(m)
	// 	found.Spec.Replicas = &replicas
	// 	needsUpdate = true
	// }

	if !reflect.DeepEqual(container.Args, utils.GetContainerArgs(expected)) {
		log.Info("Enforcing container arguments")
		args := utils.GetContainerArgs(expected)
		container.Args = args
		needsUpdate = true
	}

	if !reflect.DeepEqual(container.Env, utils.GetContainerEnvVars(expected)) {
		log.Info("Enforcing container environment variables")
		envs := utils.GetContainerEnvVars(expected)
		container.Env = envs
		needsUpdate = true
	}

	if !reflect.DeepEqual(pod.Tolerations, defaultTolerations()) {
		log.Info("Enforcing spec tolerations")
		pod.Tolerations = defaultTolerations()
		needsUpdate = true
	}

	if !reflect.DeepEqual(container.VolumeMounts, utils.GetContainerVolumeMounts(expected)) {
		log.Info("Enforcing container volume mounts")
		vms := utils.GetContainerVolumeMounts(expected)
		container.VolumeMounts = vms
		needsUpdate = true
	}

	expectedRequestResourceList := utils.GetContainerRequestResources(expected)
	if !reflect.DeepEqual(container.Resources.Requests.Cpu().MilliValue(), expectedRequestResourceList.Cpu().MilliValue()) {
		log.Info("Enforcing container resource requests and limits")
		container.Resources.Requests = expectedRequestResourceList
		needsUpdate = true
	}

	if !equality.Semantic.DeepEqual(pod.Volumes, expected.Spec.Template.Spec.Volumes) {
		log.Info("Enforcing pod volumes")
		pod.Volumes = expected.Spec.Template.Spec.Volumes
		needsUpdate = true
	}

	return found, needsUpdate
}
