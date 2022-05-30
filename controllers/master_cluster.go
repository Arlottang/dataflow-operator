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

func (r *DataflowEngineReconciler) ReconcileMasterCluster(ctx context.Context, instance *dataflowv1.DataflowEngine, req ctrl.Request) (ctrl.Result, error) {

	logg := log.FromContext(ctx)

	var svc corev1.Service
	svc.Name = instance.Spec.Master.Name
	svc.Namespace = instance.Namespace
	or, err := ctrl.CreateOrUpdate(ctx, r.Client, &svc, func() error {
		MasterService(instance, &svc)
		return controllerutil.SetControllerReference(instance, &svc, r.Scheme)
	})

	if err != nil {
		logg.Error(err, "create master service error")
		return ctrl.Result{}, err
	}

	logg.Info("CreateOrUpdate", "Master Service", or)

	var sts appsv1.StatefulSet
	sts.Name = instance.Spec.Master.Name
	sts.Namespace = instance.Namespace
	or, err = ctrl.CreateOrUpdate(ctx, r.Client, &sts, func() error {
		MasterStatefulSet(instance, &sts)
		return controllerutil.SetControllerReference(instance, &sts, r.Scheme)
	})

	if err != nil {
		logg.Error(err, "create master statefulSet error")
		return ctrl.Result{}, err
	}

	logg.Info("dataflow engine master reconcile end", "reconcile", "success")

	return ctrl.Result{}, nil
}

// MasterService expose the port for accessing the Master, not headless
func MasterService(de *dataflowv1.DataflowEngine, svc *corev1.Service) {
	svc.Labels = map[string]string{
		DataflowEngineCommonLabelKey: "engine",
	}

	// todo: need more info for ports
	svc.Spec = corev1.ServiceSpec{
		Selector: map[string]string{
			MasterClusterCommonLabelKey: "engine-master",
		},
		Ports: []corev1.ServicePort{
			{
				Name:     "master-addr",
				Port:     10240,
				Protocol: corev1.ProtocolTCP,
			},
		},
	}
}

func MasterStatefulSet(de *dataflowv1.DataflowEngine, sts *appsv1.StatefulSet) {
	sts.Labels = map[string]string{
		DataflowEngineCommonLabelKey: "engine",
	}

	sts.Spec = appsv1.StatefulSetSpec{
		Replicas: de.Spec.Master.Size,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				MasterClusterCommonLabelKey: "engine-master",
			},
		},
		ServiceName: de.Spec.Master.Name,
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					DataflowEngineCommonLabelKey: "engine",
					MasterClusterCommonLabelKey:  "engine-master",
				},
			},
			Spec: corev1.PodSpec{
				Containers: newMasterContainer(de),
			},
		},
	}
	// no need for pvc
}

func newMasterContainer(de *dataflowv1.DataflowEngine) []corev1.Container {

	// todo: add more command and envs
	return []corev1.Container{
		{
			Name:            "server-master",
			Image:           de.Spec.Image,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Ports:           []corev1.ContainerPort{},
			Env:             []corev1.EnvVar{},
			Command:         de.Spec.Master.Command,
			VolumeMounts:    []corev1.VolumeMount{},
		},
	}
}
