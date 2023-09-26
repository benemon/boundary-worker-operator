package controller

import (
	"fmt"
	"strings"

	workersv1alpha1 "github.com/benemon/boundary-worker-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	currentActivationTokenAnnotation = "boundaryproject.io/activation-token"
	hcpClusterIDAnnotation           = "boundaryproject.io/hcp-cluster-id"
)

// Generate the ConfigMap for the BoundaryPKIWorker configuration. Will be added into the StatefulSet as a VolumeMount
func (r *BoundaryPKIWorkerReconciler) configMapForBoundaryPKIWorker(
	boundaryPkiWorker *workersv1alpha1.BoundaryPKIWorker) (*corev1.ConfigMap, error) {

	ls := labelsForBoundaryPKIWorker(boundaryPkiWorker.Name)
	cmData := make(map[string]string)
	annotations := make(map[string]string)
	annotations[currentActivationTokenAnnotation] = boundaryPkiWorker.Spec.Registration.ControllerGeneratedActivationToken
	annotations[hcpClusterIDAnnotation] = boundaryPkiWorker.Spec.Registration.HCPBoundaryClusterID

	cmData["worker.hcl"] = r.configMapData(boundaryPkiWorker.Spec.Registration.HCPBoundaryClusterID, boundaryPkiWorker.Spec.Registration.ControllerGeneratedActivationToken, boundaryPkiWorker)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%s-configuration", boundaryPkiWorker.Name),
			Namespace:   boundaryPkiWorker.Namespace,
			Labels:      ls,
			Annotations: annotations,
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

// solve for custom tags
func tagsForBoundaryPKIWorker(
	boundaryPkiWorker *workersv1alpha1.BoundaryPKIWorker) string {
	var sa strings.Builder
	tags := boundaryPkiWorker.Spec.Tags
	for k, v := range tags {
		sa.WriteString(fmt.Sprintf("%s = [", k))

		tags := strings.Split(v, ",")
		for i, val := range tags {
			sa.WriteString(fmt.Sprintf("\"%s\"", val))
			if i != len(tags) {
				sa.WriteString(", ")
			}
		}
		sa.WriteString("]")
		sa.WriteString("\n")
	}
	return sa.String()
}

// If there's a cleaner way of doing this, I'm all ears
func (r *BoundaryPKIWorkerReconciler) configMapData(hcpBoundaryClusterId string,
	controllerGeneratedActivationToken string,
	boundaryPkiWorker *workersv1alpha1.BoundaryPKIWorker) string {

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
	if controllerGeneratedActivationToken != "" {
		sb.WriteString(fmt.Sprintf("	controller_generated_activation_token = \"%s\"", controllerGeneratedActivationToken))
		sb.WriteString("\n")
	}
	sb.WriteString("	auth_storage_path = \"/opt/boundary/data\"")
	sb.WriteString("\n")
	sb.WriteString("	tags {")
	sb.WriteString("\n")
	sb.WriteString("    		type = [\"kubernetes\"]")
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("		namespace = [\"%s\"]", boundaryPkiWorker.Namespace))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("		boundary_pki_worker = [\"%s\"]", boundaryPkiWorker.Name))
	sb.WriteString("\n")
	if boundaryPkiWorker.Spec.Tags != nil && len(boundaryPkiWorker.Spec.Tags) > 0 {
		sb.WriteString(tagsForBoundaryPKIWorker(boundaryPkiWorker))
	}
	sb.WriteString("	}")
	sb.WriteString("\n")
	sb.WriteString("}")

	return sb.String()
}
