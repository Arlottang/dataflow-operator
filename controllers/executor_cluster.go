package controllers

import (
	"context"
	dataflowv1 "github.com/StepOnce7/dataflow-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strconv"
	"strings"
)

func (r *DataflowEngineReconciler) ReconcileExecutorCluster(ctx context.Context, instance *dataflowv1.DataflowEngine, req ctrl.Request) (ctrl.Result, error) {

	logg := log.FromContext(ctx)

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

	var svc corev1.Service
	svc.Name = instance.Spec.Executor.Name
	svc.Namespace = instance.Namespace
	or, err = ctrl.CreateOrUpdate(ctx, r.Client, &svc, func() error {
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

	logg.Info("CreateOrUpdate", "Executor StatefulSet", or)

	logg.Info("dataflow engine executor reconcile end", "reconcile", "success")

	return ctrl.Result{}, nil
}

// ExecutorHeadlessService expose the port for accessing the Executor, is headless
func ExecutorHeadlessService(de *dataflowv1.DataflowEngine, svc *corev1.Service) {
	svc.Labels = map[string]string{
		DataflowEngineCommonLabelKey: "engine",
	}

	svc.Spec = corev1.ServiceSpec{
		ClusterIP: corev1.ClusterIPNone,
		Selector: map[string]string{
			ExecutorClusterCommonLabelKey: "executor",
		},
		Ports: []corev1.ServicePort{
			{
				Name:     "worker",
				Port:     de.Spec.Executor.Ports,
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
				ExecutorClusterCommonLabelKey: "executor",
			},
		},
		ServiceName: de.Spec.Executor.Name,
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					DataflowEngineCommonLabelKey:  "engine",
					ExecutorClusterCommonLabelKey: "executor",
				},
			},
			Spec: corev1.PodSpec{
				InitContainers: []corev1.Container{
					{
						Name:    "init-master",
						Image:   "busybox:1.28.3",
						Command: loadExecutorInitContainerCommand(de),
					},
				},
				Containers: newExecutorContainer(de),
				Volumes: []corev1.Volume{
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
		VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "dataflow",
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("1G"),
						},
					},
				},
			},
		},
	}
	// no need for pvc
}

func loadExecutorInitContainerCommand(de *dataflowv1.DataflowEngine) []string {
	if *(de.Spec.Master.Size) == 1 {
		return []string{
			"sh", "-c",
			`until nslookup server-master; 
do 
	echo waiting for server-master; 
	sleep 2; 
done;`,
		}
	}

	listenStr := "server-master-" + strconv.Itoa(int(*(de.Spec.Master.Size))-1) + "." + "server-master"

	origin := `until nslookup tmp; 
do 
	echo waiting for tmp; 
	sleep 2; 
done;`

	currentStr := strings.Replace(origin, "tmp", listenStr, -1)

	return []string{
		"sh", "-c",
		currentStr,
	}
}

func newExecutorContainer(de *dataflowv1.DataflowEngine) []corev1.Container {

	return []corev1.Container{
		{
			Name:            de.Spec.Executor.Name,
			Image:           de.Spec.Image,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Ports: []corev1.ContainerPort{
				{
					Name:          "worker",
					ContainerPort: de.Spec.Executor.Ports,
					Protocol:      corev1.ProtocolTCP,
				},
			},
			Env: []corev1.EnvVar{
				{
					Name: "POD_HOSTNAME",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.name",
						},
					},
				},
				{
					Name:  "EXECUTOR_SERVICE_NAME",
					Value: de.Spec.Executor.Name,
				},
				{
					Name:  "JOIN_MASTER_CLUSTER",
					Value: getJoinMasterClusterInfo(de),
				},
			},
			Command: loadExecutorContainerCommand(de),
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
	}
}

func loadExecutorContainerCommand(de *dataflowv1.DataflowEngine) []string {
	if *(de.Spec.Master.Size) == 1 {
		return []string{
			"sh", "-c",
			`exec /df-executor --config /mnt/config-map/config.toml \
					--join server-master:10240 \
					--worker-addr 0.0.0.0:10241 \
					--advertise-addr ${POD_HOSTNAME}.${EXECUTOR_SERVICE_NAME}:10241`,
		}
	}

	return []string{
		"sh", "-c",
		`exec /df-executor --config /mnt/config-map/config.toml \
					--join ${JOIN_MASTER_CLUSTER} \
					--worker-addr 0.0.0.0:10241 \
					--advertise-addr ${POD_HOSTNAME}.${EXECUTOR_SERVICE_NAME}:10241`,
	}

}

func getJoinMasterClusterInfo(de *dataflowv1.DataflowEngine) string {

	joinStr := ""

	num := int(*(de.Spec.Master.Size))

	for i := 0; i < num; i++ {
		joinStr += "server-master-" + strconv.Itoa(i) + "." + "server-master" + ":10240"
		if i == num-1 {
			break
		}
		joinStr += ","
	}

	return joinStr
}
