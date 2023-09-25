package controller

import (
	workersv1alpha1 "github.com/benemon/boundary-worker-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

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
