package controller

import (
	"context"
	"crypto/md5"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"

	workersv1alpha1 "github.com/benemon/boundary-worker-operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	boundaryPkiWorkerReplicas   int32 = 1
	restartedAtAnnotation             = "boundaryproject.io/restarted-at"
	resourcesAnnotation               = "boundaryproject.io/resources"
	boundaryWorkerContainerName       = "boundary-worker"
)

// Generate the StatefulSet for the BoundaryPKIWorker
func (r *BoundaryPKIWorkerReconciler) statefulsetForBoundaryPKIWorker(
	boundaryPkiWorker *workersv1alpha1.BoundaryPKIWorker) (*appsv1.StatefulSet, error) {
	ls := labelsForBoundaryPKIWorker(boundaryPkiWorker.Name)
	var replicas int32 = boundaryPkiWorkerReplicas

	// Get the Operand image
	image, err := imageForBoundaryPKIWorker()
	if err != nil {
		return nil, err
	}

	resources, resHash := resourceRequirementsBoundaryPKIWorker(boundaryPkiWorker)
	annotations := make(map[string]string)
	annotations[resourcesAnnotation] = resHash

	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        boundaryPkiWorker.Name,
			Namespace:   boundaryPkiWorker.Namespace,
			Annotations: annotations,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("%s-storage-volume", boundaryPkiWorker.Name),
					},
					Spec: volumeClaimTemplateSpecBoundaryPKIWorker(boundaryPkiWorker.Name, boundaryPkiWorker.Spec.Resources.Storage.StorageClassName),
				},
			},
			ServiceName: boundaryPkiWorker.Name,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &[]bool{true}[0],
						// IMPORTANT: seccomProfile was introduced with Kubernetes 1.19
						// If you are looking for to produce solutions to be supported
						// on lower versions you must remove this option.
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Volumes: []corev1.Volume{{
						Name: fmt.Sprintf("%s-configuration-volume", boundaryPkiWorker.Name),
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: fmt.Sprintf("%s-configuration", boundaryPkiWorker.Name),
								},
							},
						},
					},
					},
					Containers: []corev1.Container{{
						Image:           image,
						Name:            boundaryWorkerContainerName,
						ImagePullPolicy: corev1.PullAlways,
						// Ensure restrictive context for the container
						// More info: https://kubernetes.io/docs/concepts/security/pod-security-standards/#restricted
						SecurityContext: &corev1.SecurityContext{
							// WARNING: Ensure that the image used defines an UserID in the Dockerfile
							// otherwise the Pod will not run and will fail with "container has runAsNonRoot and image has non-numeric user"".
							// If you want your workloads admitted in namespaces enforced with the restricted mode in OpenShift/OKD vendors
							// then, you MUST ensure that the Dockerfile defines a User ID OR you MUST leave the "RunAsNonRoot" and
							// "RunAsUser" fields empty.
							RunAsNonRoot: &[]bool{true}[0],
							// The BoundaryPKIWorker image does not use a non-zero numeric user as the default user.
							// Due to RunAsNonRoot field being set to true, we need to force the user in the
							// container to a non-zero numeric user. We do this using the RunAsUser field.
							// However, if you are looking to provide solution for K8s vendors like OpenShift
							// be aware that you cannot run under its restricted-v2 SCC if you set this value.
							AllowPrivilegeEscalation: &[]bool{false}[0],
							Capabilities: &corev1.Capabilities{
								Drop: []corev1.Capability{
									"ALL",
								},
							},
						},
						Ports: []corev1.ContainerPort{
							{
								ContainerPort: 9202,
								Name:          "proxy",
							},
							{
								ContainerPort: 9203,
								Name:          "ops",
							}},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      fmt.Sprintf("%s-configuration-volume", boundaryPkiWorker.Name),
								MountPath: "/opt/boundary/config/",
							},
							{
								Name:      fmt.Sprintf("%s-storage-volume", boundaryPkiWorker.Name),
								MountPath: "/opt/boundary/data/",
							},
						},
						Resources:      *resources,
						LivenessProbe:  buildProbe(5, 1, 15),
						ReadinessProbe: buildProbe(5, 1, 15),
					}},
				},
			},
		},
	}

	// Set the ownerRef for the StatefulSet
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(boundaryPkiWorker, ss, r.Scheme); err != nil {
		return nil, err
	}
	return ss, nil
}

func buildProbe(initialDelay int32, timeoutSeconds int32, periodSeconds int32) *corev1.Probe {
	lp := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/health",
				Port:   intstr.FromInt(9203),
				Scheme: corev1.URISchemeHTTP,
			},
		},
		InitialDelaySeconds: initialDelay,
		TimeoutSeconds:      timeoutSeconds,
		PeriodSeconds:       periodSeconds,
		SuccessThreshold:    1,
		FailureThreshold:    3,
	}

	return lp
}

func resourceRequirementsBoundaryPKIWorker(boundaryPkiWorker *workersv1alpha1.BoundaryPKIWorker) (*corev1.ResourceRequirements, string) {
	rr := &corev1.ResourceRequirements{
		Requests: buildResourcesBoundaryPKIWorker(&boundaryPkiWorker.Spec.Resources.Requests),
		Limits:   buildResourcesBoundaryPKIWorker(&boundaryPkiWorker.Spec.Resources.Limits),
	}
	return rr, fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%v", rr))))
}

func buildResourcesBoundaryPKIWorker(runtimeResources *workersv1alpha1.BoundaryPKIWorkerRuntimeSpec) corev1.ResourceList {
	rl := corev1.ResourceList{}
	if runtimeResources.CPU != "" {
		rl[corev1.ResourceCPU] = resource.MustParse(runtimeResources.CPU)
	}
	if runtimeResources.Memory != "" {
		rl[corev1.ResourceMemory] = resource.MustParse(runtimeResources.Memory)
	}
	return rl
}

func volumeClaimTemplateSpecBoundaryPKIWorker(boundaryPkiWorkerName string, storageClassName string) corev1.PersistentVolumeClaimSpec {
	if storageClassName != "" {
		pvct := corev1.PersistentVolumeClaimSpec{

			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			StorageClassName: &storageClassName,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("256Mi"),
				},
			},
		}
		return pvct
	} else {
		pvct := corev1.PersistentVolumeClaimSpec{

			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			// default storage class
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("256Mi"),
				},
			},
		}
		return pvct
	}
}

// Update an annotation in the StatefulSet which will trigger Kubernetes to rollout it out again
func (r *BoundaryPKIWorkerReconciler) rolloutStatefulSet(ctx context.Context, statefulSet appsv1.StatefulSet) {
	log := log.FromContext(ctx)
	annotations := make(map[string]string)
	annotations[restartedAtAnnotation] = time.Now().Format("2006-01-02 15:04:05")
	statefulSet.Spec.Template.Annotations = annotations
	if err := r.Update(ctx, &statefulSet); err != nil {
		log.Error(err, "failed to rollout StatefulSet",
			"StatefulSet.Namespace", statefulSet.Namespace, "StatefulSet.Name", statefulSet.Name)
	}
	log.Info("succesfully triggered rollout of StatefulSet",
		"StatefulSet.Namespace", statefulSet.Namespace, "StatefulSet.Name", statefulSet.Name)
}
