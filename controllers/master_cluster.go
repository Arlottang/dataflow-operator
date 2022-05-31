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
)

func (r *DataflowEngineReconciler) ReconcileMasterCluster(ctx context.Context, instance *dataflowv1.DataflowEngine, req ctrl.Request) (ctrl.Result, error) {

	logg := log.FromContext(ctx)

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

	var svc corev1.Service
	svc.Name = instance.Spec.Master.Name
	svc.Namespace = instance.Namespace
	or, err = ctrl.CreateOrUpdate(ctx, r.Client, &svc, func() error {
		MasterHeadlessService(instance, &svc)
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

	logg.Info("CreateOrUpdate", "Master StatefulSet", or)

	logg.Info("dataflow engine master reconcile end", "reconcile", "success")

	return ctrl.Result{}, nil
}

// MasterHeadlessService expose the port for accessing the Master, not headless
func MasterHeadlessService(de *dataflowv1.DataflowEngine, svc *corev1.Service) {
	svc.Labels = map[string]string{
		DataflowEngineCommonLabelKey: "engine",
	}

	svc.Spec = corev1.ServiceSpec{
		ClusterIP: corev1.ClusterIPNone,
		Selector: map[string]string{
			MasterClusterCommonLabelKey: "master",
		},
		Ports: []corev1.ServicePort{
			{
				Name:     "master",
				Port:     de.Spec.Master.Ports,
				Protocol: corev1.ProtocolTCP,
			},
			{
				Name:     "peer",
				Port:     8291,
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
				MasterClusterCommonLabelKey: "master",
			},
		},
		ServiceName: de.Spec.Master.Name,
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					DataflowEngineCommonLabelKey: "engine",
					MasterClusterCommonLabelKey:  "master",
				},
			},
			Spec: corev1.PodSpec{
				InitContainers: newMasterInitContainer(de),
				Containers:     newMasterContainer(de),
				Volumes: []corev1.Volume{
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
		VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "df",
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

// todo
func newMasterInitContainer(de *dataflowv1.DataflowEngine) []corev1.Container {

	return nil
}

func newMasterContainer(de *dataflowv1.DataflowEngine) []corev1.Container {

	return []corev1.Container{
		{
			Name:            de.Spec.Master.Name,
			Image:           de.Spec.Image,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command:         loadMasterContainerCommand(de),
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
					Name:  "MASTER_SERVICE",
					Value: "server-master",
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
	}
}

func loadMasterContainerCommand(de *dataflowv1.DataflowEngine) []string {
	initStr := ""

	num := int(*(de.Spec.Master.Size))

	// master0=http://server-master-0:8291,master1=http://server-master-1:8291,master2=http://server-master-2:8291"

	for i := 0; i < num; i++ {
		initStr += "server-master-" + strconv.Itoa(i) + "=" + "https://" + "server-master-" + strconv.Itoa(i)
		initStr += "." + "server-master" + ":8291"
		if i == num-1 {
			break
		}
		initStr += ","
	}

	return []string{
		"/df-master",
		"--name", "${POD_HOSTNAME}",
		"--config", "/mnt/config-map/master.toml",
		"--master-addr", "0.0.0.0:10240",
		"--advertise-addr", "${POD_HOSTNAME}.server-master:10240",
		"--peer-urls", "http://127.0.0.1:8291",
		"--advertise-peer-urls", "http://${POD_HOSTNAME}.server-master:8291",
		"--initial-cluster", initStr,
		"--frame-meta-endpoints", "frame-mysql-standalone:3306",
		"--user-meta-endpoints", "user-etcd-standalone:2379",
	}
}
