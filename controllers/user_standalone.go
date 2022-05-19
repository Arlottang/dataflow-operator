package controllers

import (
	dataflowv1 "github.com/StepOnce7/dataflow-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"
)

//func (r *DataflowEngineReconciler) ReconcileUserStandalone(ctx context.Context, instance *dataflowv1.DataflowEngine, req ctrl.Request) (ctrl.Result, error) {
//
//	logg := log.FromContext(ctx)
//
//	statefulSet := &appsv1.StatefulSet{}
//
//	logg.Info("5 find etcd statefulSet")
//	err := r.Get(ctx, req.NamespacedName, statefulSet)
//	if err != nil {
//		if errors.IsNotFound(err) {
//			logg.Info("5.1 etcd statefulSet is not exists")
//
//			svc := corev1.Service{}
//			svc.Name = instance.Spec.UserStandalone.Name
//			svc.Namespace = instance.Namespace
//
//			or, err := ctrl.CreateOrUpdate(ctx, r, &svc, func() error {
//				MutateHeadlessSvc(instance, &svc)
//				return controllerutil.SetControllerReference(instance, &svc, r.Scheme)
//			})
//
//			if err != nil {
//				logg.Error(err, "5.2 create etcd service error")
//				return ctrl.Result{}, err
//			}
//
//			logg.WithValues("CreateOrUpdate", "Service", or)
//
//			logg.Info("5.3 start create statefulSet")
//
//			var sts appsv1.StatefulSet
//			sts.Name = instance.Spec.UserStandalone.Name
//			sts.Namespace = instance.Namespace
//
//			or, err = ctrl.CreateOrUpdate(ctx, r, &sts, func() error {
//				MutateStatefulSet(instance, &sts)
//				return controllerutil.SetControllerReference(instance, &sts, r.Scheme)
//			})
//
//			if err != nil {
//				logg.Error(err, "5.4 create etcd statefulSet error")
//				return ctrl.Result{}, err
//			}
//			logg.WithValues("CreateOrUpdate", "StatefulSet", or)
//
//			logg.Info("5.5 create statefulSet success")
//		} else {
//			logg.Error(err, "5.6 get etcd statefulSet error")
//			return ctrl.Result{}, err
//		}
//	}
//	return ctrl.Result{}, nil
//}

// MutateStatefulSet is etcd cluster for user standalone
func MutateStatefulSet(de *dataflowv1.DataflowEngine, sts *appsv1.StatefulSet) {

	sts.Labels = map[string]string{
		EtcdClusterCommonLabelKey: "etcd",
	}

	sts.Spec = appsv1.StatefulSetSpec{
		Replicas:    de.Spec.UserStandalone.Size,
		ServiceName: de.Spec.UserStandalone.Name,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				EtcdClusterLabelKey: de.Spec.UserStandalone.Name,
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: de.Namespace,
				Labels: map[string]string{
					EtcdClusterLabelKey:       de.Spec.UserStandalone.Name,
					EtcdClusterCommonLabelKey: "etcd",
				},
			},
			Spec: corev1.PodSpec{
				Containers: newContainers(de),
			},
		},
		VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
			{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: de.Namespace,
					Name:      EtcdDataDirName,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("1Gi"),
						},
					},
				},
			},
		},
	}
}

func newContainers(de *dataflowv1.DataflowEngine) []corev1.Container {
	return []corev1.Container{
		{
			Name:  "etcd",
			Image: de.Spec.UserStandalone.Image,
			Ports: []corev1.ContainerPort{
				{
					Name:          "peer",
					ContainerPort: de.Spec.UserStandalone.Ports[1],
				}, {
					Name:          "client",
					ContainerPort: de.Spec.UserStandalone.Ports[0],
				},
			},
			Env: []corev1.EnvVar{
				{
					Name:  "INITIAL_CLUSTER_SIZE",
					Value: strconv.Itoa(int(*de.Spec.UserStandalone.Size)),
				},
				{
					Name:  "SET_NAME",
					Value: de.Spec.UserStandalone.Name,
				},
				{
					Name: "POD_IP",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "status.podIP",
						},
					},
				},
				{
					Name: "MY_NAMESPACE",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.namespace",
						},
					},
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      EtcdDataDirName,
					MountPath: "/var/run/etcd",
				},
			},
			// todo: need see doc
			Command: de.Spec.UserStandalone.Command,
			Lifecycle: &corev1.Lifecycle{
				PreStop: &corev1.LifecycleHandler{
					Exec: &corev1.ExecAction{
						// todo: need see doc
						Command: []string{},
					},
				},
			},
		},
	}
}

func MutateHeadlessSvc(de *dataflowv1.DataflowEngine, svc *corev1.Service) {

	svc.Labels = map[string]string{
		EtcdClusterCommonLabelKey: "etcd",
	}

	svc.Spec = corev1.ServiceSpec{
		ClusterIP: corev1.ClusterIPNone,
		Selector: map[string]string{
			EtcdClusterLabelKey: de.Spec.UserStandalone.Name,
		},
		Ports: []corev1.ServicePort{
			{
				Name: "peer",
				Port: de.Spec.UserStandalone.Ports[1],
			},
			{
				Name: "client",
				Port: de.Spec.UserStandalone.Ports[0],
			},
		},
	}
}
