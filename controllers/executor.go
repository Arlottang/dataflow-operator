package controllers

import (
	"context"
	dataflowv1 "github.com/StepOnce7/dataflow-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *DataflowEngineReconciler) ReconcileExecutor(ctx context.Context, instance *dataflowv1.DataflowEngine, req ctrl.Request) (ctrl.Result, error) {

	logg := log.FromContext(ctx)

	var svc corev1.Service
	svc.Name = instance.Spec.Executor.Name
	svc.Namespace = instance.Namespace
	or, err := ctrl.CreateOrUpdate(ctx, r.Client, &svc, func() error {
		ExecutorHeadlessService(instance, &svc)
		return controllerutil.SetControllerReference(instance, &svc, r.Scheme)
	})

	if err != nil {
		logg.Error(err, "create executor service error")
		return ctrl.Result{}, err
	}

	logg.Info("CreateOrUpdate", "Executor Service", or)

	var sts appsv1.StatefulSet
	sts.Name = instance.Spec.Executor.Name
	sts.Namespace = instance.Namespace
	or, err = ctrl.CreateOrUpdate(ctx, r.Client, &sts, func() error {
		ExecutorStatefulSet(instance, &sts)
		return controllerutil.SetControllerReference(instance, &sts, r.Scheme)
	})

	if err != nil {
		logg.Error(err, "create executor statefulSet error")
		return ctrl.Result{}, err
	}

	logg.Info("dataflow engine executor reconcile end", "reconcile", "success")

	return ctrl.Result{}, nil
}

// ExecutorHeadlessService expose the port for accessing the Executor, is headless
func ExecutorHeadlessService(de *dataflowv1.DataflowEngine, svc *corev1.Service) {
	svc.Labels = map[string]string{
		DataflowEngineCommonLabelKey: "engine",
	}

	// todo: need more info for ports
	svc.Spec = corev1.ServiceSpec{
		Selector: map[string]string{
			ExecutorClusterCommonLabelKey: "engine-executor",
		},
		Ports: []corev1.ServicePort{
			{
				Name:     "worker-addr",
				Port:     10241,
				Protocol: corev1.ProtocolTCP,
			},
		},
	}
}

func ExecutorStatefulSet(de *dataflowv1.DataflowEngine, sts *appsv1.StatefulSet) {
	sts.Labels = map[string]string{
		DataflowEngineCommonLabelKey: "engine",
	}

	sts.Spec = appsv1.StatefulSetSpec{
		Replicas: de.Spec.Executor.Size,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				ExecutorClusterCommonLabelKey: "engine-executor",
			},
		},
		ServiceName: de.Spec.Executor.Name,
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					DataflowEngineCommonLabelKey:  "engine",
					ExecutorClusterCommonLabelKey: "engine-executor",
				},
			},
			Spec: corev1.PodSpec{
				Containers: newExecutorContainer(de),
			},
		},
	}
	// no need for pvc
}

func newExecutorContainer(de *dataflowv1.DataflowEngine) []corev1.Container {

	// todo: add more command and envs
	return []corev1.Container{
		{
			Name:            "server-executor",
			Image:           de.Spec.Image,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Ports:           []corev1.ContainerPort{},
			Env:             []corev1.EnvVar{},
			Command:         de.Spec.Executor.Command,
			VolumeMounts:    []corev1.VolumeMount{},
		},
	}
}
