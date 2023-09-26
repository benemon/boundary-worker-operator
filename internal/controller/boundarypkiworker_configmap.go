package controller

import (
	"fmt"
	"strings"

	workersv1alpha1 "github.com/benemon/boundary-worker-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

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
	sb.WriteString("	}")
	sb.WriteString("\n")
	sb.WriteString("}")

	return sb.String()
}
