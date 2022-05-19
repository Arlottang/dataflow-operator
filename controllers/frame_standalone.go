package controllers

import (
	"context"
	dataflowv1 "github.com/StepOnce7/dataflow-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *DataflowEngineReconciler) ReconcileFrameStandalone(ctx context.Context, instance *dataflowv1.DataflowEngine, req ctrl.Request) (ctrl.Result, error) {

	logg := log.FromContext(ctx)

	deployment := &appsv1.Deployment{}

	logg.Info("4.0 find mysql deployment")
	err := r.Get(ctx, req.NamespacedName, deployment)

	if err != nil {
		if errors.IsNotFound(err) {
			logg.Info("4.1 deployment is not exists, will create it")

			if err = createPVCIfNotExists(ctx, r, instance, req); err != nil {
				logg.Error(err, "4.1.a create PVC error")
				return ctrl.Result{}, nil
			}

			if err = createServiceIfNotExists(ctx, r, instance, req); err != nil {
				logg.Error(err, "4.1.b create mysql service error")
				return ctrl.Result{}, err
			}

			if err = createDeploymentIfNotExists(ctx, r, instance, req); err != nil {
				logg.Error(err, "4.1.c create mysql deployment error")
				return ctrl.Result{}, err
			}

		} else {
			logg.Error(err, "4.2 get deployment error")
			return ctrl.Result{}, err
		}
	}

	logg.Info("4.3 frame standalone reconcile success")

	return ctrl.Result{}, nil
}

func createPVCIfNotExists(ctx context.Context, r *DataflowEngineReconciler, dataflow *dataflowv1.DataflowEngine, req ctrl.Request) error {
	logg := log.FromContext(ctx)
	logg.Info("a.1 start create pvc for pc")

	pvc := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, req.NamespacedName, pvc)

	if err == nil {
		logg.Info("a.2 pvc exists")
		return nil
	}

	if !errors.IsNotFound(err) {
		logg.Error(err, "a.3 query pvc error")
		return err
	}

	scn := "mysql"
	pvc = &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: dataflow.Namespace,
			Name:      "mysql-pv-claim",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &scn,
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("5Gi"),
				},
			},
		},
	}

	logg.Info("a.4 set pvc reference")

	err = controllerutil.SetControllerReference(dataflow, pvc, r.Scheme)
	if err != nil {
		logg.Error(err, "a.5 set controller reference error, about pvc")
		return err
	}

	logg.Info("a.6 start create PVC")
	err = r.Create(ctx, pvc)

	if err != nil {
		logg.Error(err, "a.7 create PVC error")
		return err
	}

	logg.Info("a.8 create PVC success")

	return nil

}

func createServiceIfNotExists(ctx context.Context, r *DataflowEngineReconciler, dataflow *dataflowv1.DataflowEngine, req ctrl.Request) error {
	logg := log.FromContext(ctx)

	logg.Info("b.1 start create mysql service if not exists")

	service := &corev1.Service{}

	err := r.Get(ctx, req.NamespacedName, service)
	if err == nil {
		logg.Info("b.2 mysql service exists")
		return nil
	}

	if !errors.IsNotFound(err) {
		logg.Error(err, "b.3 query mysql service error")
		return err
	}

	service = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: dataflow.Namespace,
			Name:      "mysql-service",
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port: dataflow.Spec.FrameStandalone.Port,
				},
			},
			Selector: map[string]string{
				"storage": FRAME_STANDALONE,
			},
			ClusterIP: "None",
		},
	}

	logg.Info("b.4 set mysql service reference")

	if err = controllerutil.SetControllerReference(dataflow, service, r.Scheme); err != nil {
		logg.Error(err, "b.5 set mysql service reference error")
		return err
	}

	logg.Info("b.6 start create mysql service")

	if err = r.Create(ctx, service); err != nil {
		logg.Error(err, "b.7 create mysql service error")
		return err
	}

	logg.Info("b.8 create mysql service success")
	return nil
}

func createDeploymentIfNotExists(ctx context.Context, r *DataflowEngineReconciler, dataflow *dataflowv1.DataflowEngine, req ctrl.Request) error {
	logg := log.FromContext(ctx)
	logg.Info("c.1 start create deployment for mysql")

	deployment := &appsv1.Deployment{}

	err := r.Get(ctx, req.NamespacedName, deployment)
	if err == nil {
		logg.Info("c.2 mysql deployment exists")
		return nil
	}

	if !errors.IsNotFound(err) {
		logg.Error(err, "c.3 query mysql deployment error")
		return err
	}

	deployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: dataflow.Namespace,
			Name:      FRAME_STANDALONE,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"storage": FRAME_STANDALONE,
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"storage": FRAME_STANDALONE,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            FRAME_STANDALONE,
							Image:           dataflow.Spec.FrameStandalone.Image,
							ImagePullPolicy: "IfNotPresent",
							Env: []corev1.EnvVar{
								{
									Name:  "MYSQL_ROOT_PASSWORD",
									Value: "password",
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "mysql",
									ContainerPort: CONTAINER_PORT,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "mysql-persistent-storage",
									MountPath: "/var/lib/mysql",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "mysql-persistent-storage",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "mysql-pv-claim",
								},
							},
						},
					},
				},
			},
		},
	}

	logg.Info("c.4 set mysql deployment reference")
	if err = controllerutil.SetControllerReference(dataflow, deployment, r.Scheme); err != nil {
		logg.Error(err, "c.5 set mysql deployment reference error")
		return err
	}

	logg.Info("c.6 start create deployment")
	if err = r.Create(ctx, deployment); err != nil {
		logg.Error(err, "c.7 create mysql deployment error")
		return err
	}

	logg.Info("c.8 create mysql deployment success")
	return nil
}
