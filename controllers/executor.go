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

func (r *DataflowEngineReconciler) ReconcileExecutor(ctx context.Context, instance *dataflowv1.DataflowEngine, req ctrl.Request) (ctrl.Result, error) {

	logg := log.FromContext(ctx)

	var pv corev1.PersistentVolume
	pv.Name = "executor-pv-volume"
	createExecutorPV(&pv)
	err := r.Create(ctx, &pv)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			err = nil
		} else {
			logg.Error(err, "create executor pv error")
			return ctrl.Result{}, err
		}
	}

	logg.Info("Create", "Executor PV", "success")

	var cfm corev1.ConfigMap
	cfm.Name = "executorcfm"
	cfm.Namespace = instance.Namespace

	or, err := ctrl.CreateOrUpdate(ctx, r.Client, &cfm, func() error {
		createExecutorConfigMap(&cfm)
		return controllerutil.SetControllerReference(instance, &cfm, r.Scheme)
	})

	if err != nil {
		logg.Error(err, "create executor configMap error")
		return ctrl.Result{}, err
	}
	logg.Info("CreateOrUpdate", "Executor ConfigMap", or)

	var pvc corev1.PersistentVolumeClaim
	pvc.Name = "executor-pv-claim"
	pvc.Namespace = instance.Namespace
	createExecutorPVC(&pvc)
	err = r.Create(ctx, &pvc)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			err = nil
		} else {
			logg.Error(err, "create executor pv error")
			return ctrl.Result{}, err
		}
	}

	logg.Info("Create", "Executor PVC", "success")

	var svc corev1.Service
	svc.Name = instance.Spec.Executor.Name
	svc.Namespace = instance.Namespace

	or, err = ctrl.CreateOrUpdate(ctx, r.Client, &svc, func() error {
		createExecutorService(instance, &svc)
		return controllerutil.SetControllerReference(instance, &svc, r.Scheme)
	})

	if err != nil {
		logg.Error(err, "create executor service error")
		return ctrl.Result{}, err
	}

	logg.Info("CreateOrUpdate", "Executor Service", or)

	var deploy appsv1.Deployment
	deploy.Name = instance.Spec.Executor.Name
	deploy.Namespace = instance.Namespace
	or, err = ctrl.CreateOrUpdate(ctx, r.Client, &deploy, func() error {
		createExecutorDeployment(instance, &deploy)
		return controllerutil.SetControllerReference(instance, &deploy, r.Scheme)
	})

	if err != nil {
		logg.Error(err, "create executor deployment error")
		return ctrl.Result{}, err
	}

	logg.Info("CreateOrUpdate", "Executor Deployment", or)

	logg.Info("executor reconcile end", "reconcile", "success")

	return ctrl.Result{}, nil
}

func createExecutorPV(pv *corev1.PersistentVolume) {

	pv.Spec = corev1.PersistentVolumeSpec{
		StorageClassName: "executor",
		Capacity: corev1.ResourceList{
			corev1.ResourceStorage: resource.MustParse("5Gi"),
		},
		AccessModes: []corev1.PersistentVolumeAccessMode{
			corev1.ReadWriteOnce,
		},

		PersistentVolumeSource: corev1.PersistentVolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: "/tmp/dataflow",
			},
		},
		PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimDelete,
	}
}

func createExecutorConfigMap(cfm *corev1.ConfigMap) {

	cfm.Data = map[string]string{
		"config.toml": `keepalive-ttl = "20s"
keepalive-interval = "500ms"
session-ttl = 20

worker-addr = "0.0.0.0:10241"`,
	}
}

func createExecutorPVC(pvc *corev1.PersistentVolumeClaim) {
	scn := "executor"

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

func createExecutorService(de *dataflowv1.DataflowEngine, svc *corev1.Service) {

	svc.Spec = corev1.ServiceSpec{
		ClusterIP: corev1.ClusterIPNone,
		Selector: map[string]string{
			"app": "executor",
		},
		Ports: []corev1.ServicePort{
			{
				Port: de.Spec.Executor.Ports,
			},
		},
	}
}

func createExecutorDeployment(de *dataflowv1.DataflowEngine, deploy *appsv1.Deployment) {
	deploy.Spec = appsv1.DeploymentSpec{
		Replicas: pointer.Int32Ptr(1),
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "executor",
			},
		},
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"app": "executor",
				},
			},
			Spec: corev1.PodSpec{
				InitContainers: []corev1.Container{
					{
						Name:  "init-master",
						Image: "busybox:1.28.3",
						Command: []string{
							"sh", "-c",
							`until nslookup server-master; 
do 
	echo waiting for server-master; 
	sleep 2; 
done;`,
						},
					},
				},
				Containers: []corev1.Container{
					{
						Name:            de.Spec.Executor.Name,
						Image:           de.Spec.Image,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Command: []string{
							"/df-executor",
							"--config", "/mnt/config-map/config.toml",
							"--join", "server-master:10240",
							"--worker-addr", "0.0.0.0:10241",
							"--advertise-addr", "server-executor:10241",
						},
						Ports: []corev1.ContainerPort{
							{
								Name:          "worker",
								ContainerPort: de.Spec.Executor.Ports,
								Protocol:      corev1.ProtocolTCP,
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "dataflow",
								MountPath: "/tmp/dataflow",
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
						Name: "dataflow",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: "executor-pv-claim",
							},
						},
					},
					{
						Name: "config-map",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "executorcfm",
								},
							},
						},
					},
				},
			},
		},
	}
}
