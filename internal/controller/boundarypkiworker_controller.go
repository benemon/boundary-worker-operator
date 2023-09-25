/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	workersv1alpha1 "github.com/benemon/boundary-worker-operator/api/v1alpha1"
)

const (
	boundaryPkiWorkerReplicas = 1
)

// Definitions to manage status conditions
const (
	// typeAvailableMemcached represents the status of the Deployment reconciliation
	typeAvailableBoundaryPKIWorker = "Available"
	// typeDegradedMemcached represents the status used when the custom resource is deleted and the finalizer operations are must to occur.
	typeDegradedBoundaryPKIWorker = "Degraded"
)

// BoundaryPKIWorkerReconciler reconciles a BoundaryPKIWorker object
type BoundaryPKIWorkerReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=workers.boundaryproject.io,resources=boundarypkiworkers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=workers.boundaryproject.io,resources=boundarypkiworkers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=workers.boundaryproject.io,resources=boundarypkiworkers/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumes,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the BoundaryPKIWorker object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *BoundaryPKIWorkerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("starting reconciliation loop")
	// Fetch the Memcached instance
	// The purpose is check if the Custom Resource for the Kind Memcached
	// is applied on the cluster if not we return nil to stop the reconciliation
	boundaryPkiWorker := &workersv1alpha1.BoundaryPKIWorker{}
	err := r.Get(ctx, req.NamespacedName, boundaryPkiWorker)
	if err != nil {
		log.Info("No CR")
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then, it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			log.Info("BoundaryPKIWorker resource not found - ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "failed to get BoundaryPKIWorker")
		return ctrl.Result{}, err
	}

	// Let's just set the status as Unknown when no status are available
	if boundaryPkiWorker.Status.Conditions == nil || len(boundaryPkiWorker.Status.Conditions) == 0 {
		meta.SetStatusCondition(&boundaryPkiWorker.Status.Conditions, metav1.Condition{Type: typeAvailableBoundaryPKIWorker, Status: metav1.ConditionUnknown, Reason: "reconciling", Message: "starting reconciliation"})
		if err = r.Status().Update(ctx, boundaryPkiWorker); err != nil {
			log.Error(err, "failed to update BoundaryPKIWorker status")
			return ctrl.Result{}, err
		}

		// Let's re-fetch the memcached Custom Resource after update the status
		// so that we have the latest state of the resource on the cluster and we will avoid
		// raise the issue "the object has been modified, please apply
		// your changes to the latest version and try again" which would re-trigger the reconciliation
		// if we try to update it again in the following operations
		if err := r.Get(ctx, req.NamespacedName, boundaryPkiWorker); err != nil {
			log.Error(err, "failed to re-fetch BoundaryPKIWorker")
			return ctrl.Result{}, err
		}
	}

	foundService := &corev1.Service{}
	err = r.Get(ctx, types.NamespacedName{Name: boundaryPkiWorker.Name, Namespace: boundaryPkiWorker.Namespace}, foundService)
	if err != nil && apierrors.IsNotFound(err) {
		// Define a new Service
		service, err := r.serviceForBoundaryPKIWorker(boundaryPkiWorker)
		if err != nil {
			log.Error(err, "failed to define new ConfigMap resource for BoundaryPKIWorker")

			// The following implementation will update the status
			meta.SetStatusCondition(&boundaryPkiWorker.Status.Conditions, metav1.Condition{Type: typeAvailableBoundaryPKIWorker,
				Status: metav1.ConditionFalse, Reason: "reconciling",
				Message: fmt.Sprintf("failed to create Service for the custom resource (%s): (%s)", boundaryPkiWorker.Name, err)})

			if err := r.Status().Update(ctx, boundaryPkiWorker); err != nil {
				log.Error(err, "failed to update BoundaryPKIWorker status")
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, err
		}
		log.Info("creating a new Service",
			"Service.Namespace", service.Namespace, "Service.Name", service.Name)
		if err = r.Create(ctx, service); err != nil {
			log.Error(err, "failed to create new Service",
				"Service.Namespace", service.Namespace, "Service.Name", service.Name)
			return ctrl.Result{}, err
		}

		// Service created successfully
		// We will requeue the reconciliation so that we can ensure the state
		// and move forward for the next operations
		log.Info("completed Service reconciliation block")
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	} else if err != nil {
		log.Error(err, "failed to get Service")
		// Let's return the error for the reconciliation be re-trigged again
		return ctrl.Result{}, err
	}

	foundCM := &corev1.ConfigMap{}
	err = r.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s-configuration", boundaryPkiWorker.Name), Namespace: boundaryPkiWorker.Namespace}, foundCM)
	if err != nil && apierrors.IsNotFound(err) {
		// Define a new configmap
		configMap, err := r.configMapForBoundaryPKIWorker(boundaryPkiWorker)
		if err != nil {
			log.Error(err, "failed to define new ConfigMap resource for BoundaryPKIWorker")

			// The following implementation will update the status
			meta.SetStatusCondition(&boundaryPkiWorker.Status.Conditions, metav1.Condition{Type: typeAvailableBoundaryPKIWorker,
				Status: metav1.ConditionFalse, Reason: "reconciling",
				Message: fmt.Sprintf("failed to create ConfigMap for the custom resource (%s): (%s)", boundaryPkiWorker.Name, err)})

			if err := r.Status().Update(ctx, boundaryPkiWorker); err != nil {
				log.Error(err, "failed to update BoundaryPKIWorker status")
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, err
		}
		log.Info("creating a new ConfigMap",
			"ConfigMap.Namespace", configMap.Namespace, "ConfigMap.Name", configMap.Name)
		if err = r.Create(ctx, configMap); err != nil {
			log.Error(err, "failed to create new ConfigMap",
				"ConfigMap.Namespace", configMap.Namespace, "ConfigMap.Name", configMap.Name)
			return ctrl.Result{}, err
		}

		// ConfigMap created successfully
		// We will requeue the reconciliation so that we can ensure the state
		// and move forward for the next operations
		log.Info("completed ConfigMap reconciliation block")
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	} else if err != nil {
		log.Error(err, "failed to get ConfigMap")
		// Let's return the error for the reconciliation be re-trigged again
		return ctrl.Result{}, err
	}

	// Check if the deployment already exists, if not create a new one
	foundSS := &appsv1.StatefulSet{}
	log.Info("checking if the statefulset already exists")
	err = r.Get(ctx, types.NamespacedName{Name: boundaryPkiWorker.Name, Namespace: boundaryPkiWorker.Namespace}, foundSS)
	if err != nil && apierrors.IsNotFound(err) {
		// Define a new deployment
		statefulSet, err := r.statefulsetForBoundaryPKIWorker(boundaryPkiWorker)
		if err != nil {
			log.Error(err, "failed to define new StatefulSet resource for BoundaryPKIWorker")

			// The following implementation will update the status
			meta.SetStatusCondition(&boundaryPkiWorker.Status.Conditions, metav1.Condition{Type: typeAvailableBoundaryPKIWorker,
				Status: metav1.ConditionFalse, Reason: "reconciling",
				Message: fmt.Sprintf("failed to create StatefulSet for the custom resource (%s): (%s)", boundaryPkiWorker.Name, err)})

			if err := r.Status().Update(ctx, boundaryPkiWorker); err != nil {
				log.Error(err, "failed to update BoundaryPKIWorker status")
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, err
		}

		log.Info("creating a new StatefulSet",
			"StatefulSet.Namespace", statefulSet.Namespace, "StatefulSet.Name", statefulSet.Name)
		if err = r.Create(ctx, statefulSet); err != nil {
			log.Error(err, "failed to create new StatefulSet",
				"StatefulSet.Namespace", statefulSet.Namespace, "StatefulSet.Name", statefulSet.Name)
			return ctrl.Result{}, err
		}

		// Deployment created successfully
		// We will requeue the reconciliation so that we can ensure the state
		// and move forward for the next operations
		log.Info("completed StatefulSet reconciliation block")
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	} else if err != nil {
		log.Error(err, "failed to get StatefulSet")
		// Let's return the error for the reconciliation be re-trigged again
		return ctrl.Result{}, err
	}

	var replicas int32 = boundaryPkiWorkerReplicas
	log.Info(fmt.Sprintf("desired replicas %d, current replicas %d", replicas, foundSS.Spec.Replicas))
	log.Info("checking if the statefulset has the correct number of replicas")
	if *foundSS.Spec.Replicas != replicas {
		foundSS.Spec.Replicas = &replicas
		if err = r.Update(ctx, foundSS); err != nil {
			log.Error(err, "failed to update StatefulSet",
				"StatefulSet.Namespace", foundSS.Namespace, "StatefulSet.Name", foundSS.Name)

			// Re-fetch the Custom Resource before update the status
			// so that we have the latest state of the resource on the cluster and we will avoid
			// raise the issue "the object has been modified, please apply
			// your changes to the latest version and try again" which would re-trigger the reconciliation
			if err := r.Get(ctx, req.NamespacedName, boundaryPkiWorker); err != nil {
				log.Error(err, "failed to re-fetch boundaryPkiWorker")
				return ctrl.Result{}, err
			}

			// The following implementation will update the status
			meta.SetStatusCondition(&boundaryPkiWorker.Status.Conditions, metav1.Condition{Type: typeAvailableBoundaryPKIWorker,
				Status: metav1.ConditionFalse, Reason: "resizing",
				Message: fmt.Sprintf("failed to update the replicas for the owned resource (%s): (%s)", boundaryPkiWorker.Name, err)})

			if err := r.Status().Update(ctx, boundaryPkiWorker); err != nil {
				log.Error(err, "failed to update BoundaryPkiWorker status")
				return ctrl.Result{}, err
			}
			log.Info("set replicas succesfully")
			return ctrl.Result{}, err
		}

		// Now, that we update the size we want to requeue the reconciliation
		// so that we can ensure that we have the latest state of the resource before
		// update. Also, it will help ensure the desired state on the cluster
		log.Info("completed replica reconciliation block")
		return ctrl.Result{Requeue: true}, nil
	}
	// The following implementation will update the status
	meta.SetStatusCondition(&boundaryPkiWorker.Status.Conditions, metav1.Condition{Type: typeAvailableBoundaryPKIWorker,
		Status: metav1.ConditionTrue, Reason: "reconciling",
		Message: fmt.Sprintf("Resources for custom resource (%s) created successfully in namespace (%s)", boundaryPkiWorker.Name, boundaryPkiWorker.Namespace)})

	if err := r.Status().Update(ctx, boundaryPkiWorker); err != nil {
		log.Error(err, "failed to update BoundaryPKIWorker status")
		return ctrl.Result{}, err
	}

	log.Info("end of reconciliation block")
	return ctrl.Result{}, nil
}

func (r *BoundaryPKIWorkerReconciler) serviceForBoundaryPKIWorker(
	boundaryPkiWorker *workersv1alpha1.BoundaryPKIWorker) (*corev1.Service, error) {

	ls := labelsForBoundaryPKIWorker(boundaryPkiWorker.Name)
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      boundaryPkiWorker.Name,
			Namespace: boundaryPkiWorker.Namespace,
			Labels:    ls,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name: "proxy",
					Port: 9202,
				},
				{
					Name: "ops",
					Port: 9203,
				},
			},
			ClusterIP: corev1.ClusterIPNone,
			Selector:  ls,
		},
	}
	// Set the ownerRef for the Service
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(boundaryPkiWorker, service, r.Scheme); err != nil {
		return nil, err
	}
	return service, nil
}

func (r *BoundaryPKIWorkerReconciler) configMapForBoundaryPKIWorker(
	boundaryPkiWorker *workersv1alpha1.BoundaryPKIWorker) (*corev1.ConfigMap, error) {

	ls := labelsForBoundaryPKIWorker(boundaryPkiWorker.Name)
	cmData := make(map[string]string)
	cmData["worker.hcl"] = r.configMapData(boundaryPkiWorker.Spec.Registration.HCPBoundaryClusterID, boundaryPkiWorker.Spec.Registration.ControllerGeneratedActivationToken, boundaryPkiWorker)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-configuration", boundaryPkiWorker.Name),
			Namespace: boundaryPkiWorker.Namespace,
			Labels:    ls,
		},
		Data: cmData,
	}

	// Set the ownerRef for the ConfigMap
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(boundaryPkiWorker, cm, r.Scheme); err != nil {
		return nil, err
	}
	return cm, nil
}

func (r *BoundaryPKIWorkerReconciler) configMapData(hcpBoundaryClusterId string, controllerGeneratedActivationToken string, boundaryPkiWorker *workersv1alpha1.BoundaryPKIWorker) string {
	var sb strings.Builder

	sb.WriteString("disable_mlock = true")
	sb.WriteString("\n")
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("hcp_boundary_cluster_id= \"%s\"", hcpBoundaryClusterId))
	sb.WriteString("\n")
	sb.WriteString("\n")
	sb.WriteString("listener \"tcp\" {")
	sb.WriteString("\n")
	sb.WriteString("	address = \"0.0.0.0:9202\"")
	sb.WriteString("\n")
	sb.WriteString("  	purpose = \"proxy\"")
	sb.WriteString("\n")
	sb.WriteString("}")
	sb.WriteString("\n")
	sb.WriteString("\n")
	sb.WriteString("listener \"tcp\" {")
	sb.WriteString("\n")
	sb.WriteString("	address = \"0.0.0.0:9203\"")
	sb.WriteString("\n")
	sb.WriteString("  	purpose = \"ops\"")
	sb.WriteString("\n")
	sb.WriteString("    tls_disable = true")
	sb.WriteString("\n")
	sb.WriteString("}")
	sb.WriteString("\n")
	sb.WriteString("\n")
	sb.WriteString("worker {")
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("	controller_generated_activation_token = \"%s\"", controllerGeneratedActivationToken))
	sb.WriteString("\n")
	sb.WriteString("	auth_storage_path = \"/opt/boundary/data\"")
	sb.WriteString("\n")
	sb.WriteString("	tags {")
	sb.WriteString("\n")
	sb.WriteString("    		type = [\"kubernetes\"]")
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("	namespace = [\"%s\"]", boundaryPkiWorker.Namespace))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("	boundary_pki_worker = [\"%s\"]", boundaryPkiWorker.Name))
	sb.WriteString("\n")
	sb.WriteString("	}")
	sb.WriteString("\n")
	sb.WriteString("}")

	return sb.String()
}

func (r *BoundaryPKIWorkerReconciler) statefulsetForBoundaryPKIWorker(
	boundaryPkiWorker *workersv1alpha1.BoundaryPKIWorker) (*appsv1.StatefulSet, error) {
	ls := labelsForBoundaryPKIWorker(boundaryPkiWorker.Name)
	var replicas int32 = boundaryPkiWorkerReplicas

	// Get the Operand image
	image, err := imageForBoundaryPKIWorker()
	if err != nil {
		return nil, err
	}

	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      boundaryPkiWorker.Name,
			Namespace: boundaryPkiWorker.Namespace,
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
					Spec: volumeClaimTemplateSpecBoundaryPKIWorker(boundaryPkiWorker.Name, boundaryPkiWorker.Spec.Storage.StorageClassName),
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
						Name:            "boundary-worker",
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
							// The memcached image does not use a non-zero numeric user as the default user.
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

// labelsForMemcached returns the labels for selecting the resources
// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
func labelsForBoundaryPKIWorker(name string) map[string]string {
	var imageTag string
	image, err := imageForBoundaryPKIWorker()
	if err == nil {
		imageTag = strings.Split(image, ":")[1]
	}
	return map[string]string{"app.kubernetes.io/name": "BoundaryPKIWorker",
		"app.kubernetes.io/instance":   name,
		"app.kubernetes.io/version":    imageTag,
		"app.kubernetes.io/part-of":    "boundary-worker-operator",
		"app.kubernetes.io/created-by": "controller-manager",
	}
}

// imageForMemcached gets the Operand image which is managed by this controller
// from the MEMCACHED_IMAGE environment variable defined in the config/manager/manager.yaml
func imageForBoundaryPKIWorker() (string, error) {
	var imageEnvVar = "BOUNDARY_WORKER_IMAGE"
	image, found := os.LookupEnv(imageEnvVar)
	if !found {
		return "", fmt.Errorf("unable to find %s environment variable with the image", imageEnvVar)
	}
	return image, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BoundaryPKIWorkerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&workersv1alpha1.BoundaryPKIWorker{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.PersistentVolume{}).
		Complete(r)
}
