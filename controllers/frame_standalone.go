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

	var pv corev1.PersistentVolume
	pv.Name = "mysql-standalone-pv-volume"
	createPVIfNotExists(&pv)
	err := r.Create(ctx, &pv)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			err = nil
		} else {
			logg.Error(err, "create mysql pv error")
			return ctrl.Result{}, err
		}
	}

	logg.Info("Create", "Mysql PV", "success")

	var pvc corev1.PersistentVolumeClaim
	pvc.Name = "mysql-pv-claim"
	pvc.Namespace = instance.Namespace
	createPVCIfNotExists(&pvc)
	err = r.Create(ctx, &pvc)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			err = nil
		} else {
			logg.Error(err, "create mysql pv error")
			return ctrl.Result{}, err
		}
	}

	logg.Info("Create", "Mysql PVC", "success")

	var svc corev1.Service
	svc.Name = "mysql-service"
	svc.Namespace = instance.Namespace

	or, err := ctrl.CreateOrUpdate(ctx, r.Client, &svc, func() error {
		createServiceIfNotExists(instance, &svc)
		return controllerutil.SetControllerReference(instance, &svc, r.Scheme)
	})

	if err != nil {
		logg.Error(err, "create mysql service error")
	}

	logg.Info("CreateOrUpdate", "Mysql Service", or)

	var deploy appsv1.Deployment
	deploy.Name = FRAME_STANDALONE
	deploy.Namespace = instance.Namespace
	or, err = ctrl.CreateOrUpdate(ctx, r.Client, &deploy, func() error {
		createDeploymentIfNotExists(instance, &deploy)
		return controllerutil.SetControllerReference(instance, &deploy, r.Scheme)
	})

	if err != nil {
		logg.Error(err, "create mysql deployment error")
	}

	logg.Info("CreateOrUpdate", "Mysql Deployment", or)

	logg.Info("frame standalone reconcile end", "reconcile", "success")

	return ctrl.Result{}, nil
}

func createPVCIfNotExists(pvc *corev1.PersistentVolumeClaim) {

	scn := "mysql"
	pvc.Spec = corev1.PersistentVolumeClaimSpec{
		StorageClassName: &scn,
		AccessModes: []corev1.PersistentVolumeAccessMode{
			corev1.ReadWriteOnce,
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("5Gi"),
			},
		},
	}

}

func createServiceIfNotExists(de *dataflowv1.DataflowEngine, svc *corev1.Service) {

	svc.Spec = corev1.ServiceSpec{
		ClusterIP: corev1.ClusterIPNone,
		Selector: map[string]string{
			"storage": FRAME_STANDALONE,
		},
		Ports: []corev1.ServicePort{
			{
				Port: de.Spec.FrameStandalone.Port,
			},
		},
	}
}

func createDeploymentIfNotExists(de *dataflowv1.DataflowEngine, deploy *appsv1.Deployment) {

	deploy.Spec = appsv1.DeploymentSpec{
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
						Image:           de.Spec.FrameStandalone.Image,
						ImagePullPolicy: "IfNotPresent",
						Env: []corev1.EnvVar{
							{
								Name:  "MYSQL_ROOT_PASSWORD",
								Value: "123456",
							},
						},
						Ports: []corev1.ContainerPort{
							{
								Name:          "mysql",
								ContainerPort: CONTAINER_PORT,
								Protocol:      corev1.ProtocolTCP,
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
	}
}
