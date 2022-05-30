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

func (r *DataflowEngineReconciler) ReconcileMaster(ctx context.Context, instance *dataflowv1.DataflowEngine, req ctrl.Request) (ctrl.Result, error) {

	logg := log.FromContext(ctx)

	var pv corev1.PersistentVolume
	pv.Name = "master-pv-volume"
	createMasterPV(&pv)
	err := r.Create(ctx, &pv)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			err = nil
		} else {
			logg.Error(err, "create master pv error")
			return ctrl.Result{}, err
		}
	}

	logg.Info("Create", "Master PV", "success")

	var cfm corev1.ConfigMap
	cfm.Name = "mastercfm"
	cfm.Namespace = instance.Namespace

	or, err := ctrl.CreateOrUpdate(ctx, r.Client, &cfm, func() error {
		createMasterConfigMap(&cfm)
		return controllerutil.SetControllerReference(instance, &cfm, r.Scheme)
	})

	if err != nil {
		logg.Error(err, "create master configMap error")
		return ctrl.Result{}, err
	}
	logg.Info("CreateOrUpdate", "Master ConfigMap", or)

	var pvc corev1.PersistentVolumeClaim
	pvc.Name = "master-pv-claim"
	pvc.Namespace = instance.Namespace
	createMasterPVC(&pvc)
	err = r.Create(ctx, &pvc)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			err = nil
		} else {
			logg.Error(err, "create master pv error")
			return ctrl.Result{}, err
		}
	}

	logg.Info("Create", "Master PVC", "success")

	var svc corev1.Service
	svc.Name = instance.Spec.Master.Name
	svc.Namespace = instance.Namespace

	or, err = ctrl.CreateOrUpdate(ctx, r.Client, &svc, func() error {
		createMasterService(instance, &svc)
		return controllerutil.SetControllerReference(instance, &svc, r.Scheme)
	})

	if err != nil {
		logg.Error(err, "create master service error")
		return ctrl.Result{}, err
	}

	logg.Info("CreateOrUpdate", "Master Service", or)

	var deploy appsv1.Deployment
	deploy.Name = instance.Spec.Master.Name
	deploy.Namespace = instance.Namespace
	or, err = ctrl.CreateOrUpdate(ctx, r.Client, &deploy, func() error {
		createMasterDeployment(instance, &deploy)
		return controllerutil.SetControllerReference(instance, &deploy, r.Scheme)
	})

	if err != nil {
		logg.Error(err, "create master deployment error")
		return ctrl.Result{}, err
	}

	logg.Info("CreateOrUpdate", "Master Deployment", or)

	logg.Info("master reconcile end", "reconcile", "success")

	return ctrl.Result{}, nil

}

func createMasterPV(pv *corev1.PersistentVolume) {

	pv.Spec = corev1.PersistentVolumeSpec{
		StorageClassName: "master",
		Capacity: corev1.ResourceList{
			corev1.ResourceStorage: resource.MustParse("5Gi"),
		},
		AccessModes: []corev1.PersistentVolumeAccessMode{
			corev1.ReadWriteOnce,
		},

		PersistentVolumeSource: corev1.PersistentVolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: "/tmp/df",
			},
		},
		PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimDelete,
	}
}

func createMasterConfigMap(cfm *corev1.ConfigMap) {

	cfm.Data = map[string]string{
		"master.toml": `master-addr = "0.0.0.0:10240"
[etcd]
name = "server-master-1"
data-dir = "/tmp/df/master"`,
	}
}

func createMasterPVC(pvc *corev1.PersistentVolumeClaim) {
	scn := "master"

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

func createMasterService(de *dataflowv1.DataflowEngine, svc *corev1.Service) {

	svc.Spec = corev1.ServiceSpec{
		ClusterIP: corev1.ClusterIPNone,
		Selector: map[string]string{
			"app": "master",
		},
		Ports: []corev1.ServicePort{
			{
				Port: de.Spec.Master.Ports,
			},
		},
	}
}

func createMasterDeployment(de *dataflowv1.DataflowEngine, deploy *appsv1.Deployment) {
	deploy.Spec = appsv1.DeploymentSpec{
		Replicas: pointer.Int32Ptr(1),
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "master",
			},
		},
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"app": "master",
				},
			},
			Spec: corev1.PodSpec{
				InitContainers: []corev1.Container{},
				Containers:     []corev1.Container{},
				Volumes:        []corev1.Volume{},
			},
		},
	}
}
