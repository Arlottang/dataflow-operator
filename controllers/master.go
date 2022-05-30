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
				Name: "master",
				Port: de.Spec.Master.Ports,
			},
			{
				Name: "peer",
				Port: 8291,
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
				InitContainers: []corev1.Container{
					{
						Name:  "init-mysql",
						Image: "busybox",
						Command: []string{
							"sh", "-c",
							`until nslookup frame-mysql-standalone; 
do 
	echo waiting for mysql; 
	sleep 2; 
done;`,
						},
					},
					{
						Name:  "init-etcd",
						Image: "busybox",
						Command: []string{
							"sh", "-c",
							`until nslookup user-etcd-standalone; 
do 
	echo waiting for etcd; 
	sleep 2; 
done;`,
						},
					},
				},
				Containers: []corev1.Container{
					{
						Name:            de.Spec.Master.Name,
						Image:           de.Spec.Image,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Command: []string{
							"./bin/master",
							"--config", "/mnt/config-map/master.toml",
							"--master-addr", "0.0.0.0:10240",
							"--advertise-addr", "server-master:10240",
							"--peer-urls", "http://127.0.0.1:8291",
							"--advertise-peer-urls", "http://server-master:8291",
							"--frame-meta-endpoints", "user-etcd-standalone:2379",
							"--user-meta-endpoints", "user-etcd-standalone:2379",
						},
						Ports: []corev1.ContainerPort{
							{
								Name:          "master",
								ContainerPort: de.Spec.Master.Ports,
								Protocol:      corev1.ProtocolTCP,
							},
							{
								Name:          "peer",
								ContainerPort: 8291,
								Protocol:      corev1.ProtocolTCP,
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "df",
								MountPath: "/tmp/df",
							},
							{
								Name:      "config-map",
								MountPath: "/mnt/config-map",
							},
						},
					},
				},
				RestartPolicy: corev1.RestartPolicyAlways,
				Volumes: []corev1.Volume{
					{
						Name: "df",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: "master-pv-claim",
							},
						},
					},
					{
						Name: "config-map",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "mastercfm",
								},
							},
						},
					},
				},
			},
		},
	}
}
