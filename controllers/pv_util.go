package controllers

import (
	"context"
	dataflowv1 "github.com/StepOnce7/dataflow-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type pvConfig struct {
	Name             string
	StorageClassName string
	Capacity         string
}

func buildPVConfig() []pvConfig {
	pvConfigs := []pvConfig{
		{
			Name:             "mysql-standalone-pv-volume",
			StorageClassName: "mysql",
			Capacity:         "5Gi",
		},
		{
			Name:             "etcd-standalone-pv-volume-0",
			StorageClassName: "etcd",
			Capacity:         "1Gi",
		},
		{
			Name:             "etcd-standalone-pv-volume-1",
			StorageClassName: "etcd",
			Capacity:         "1Gi",
		},
		{
			Name:             "etcd-standalone-pv-volume-2",
			StorageClassName: "etcd",
			Capacity:         "1Gi",
		},
	}

	return pvConfigs
}

func createPVIfNotExists(ctx context.Context, r *DataflowEngineReconciler, dataflow *dataflowv1.DataflowEngine, req ctrl.Request) error {
	logg := log.FromContext(ctx)

	logg.Info("3.1 start create PV if not exists")

	pv := &corev1.PersistentVolume{}

	// todo: add filters for diff pv
	err := r.Get(ctx, req.NamespacedName, pv)
	if err == nil || errors.IsAlreadyExists(err) {
		logg.Info("3.2 pv exists")
		return nil
	}

	if !errors.IsNotFound(err) {
		logg.Error(err, "3.3 query pv error")
		return err
	}

	logg.Info("3.4 start create PV")
	pvConfigs := buildPVConfig()

	for _, config := range pvConfigs {
		pv = &corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: dataflow.Namespace,
				Name:      config.Name,
				Labels: map[string]string{
					"type": "local",
				},
			},
			Spec: corev1.PersistentVolumeSpec{
				StorageClassName: config.StorageClassName,
				Capacity: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(config.Capacity),
				},
				AccessModes: []corev1.PersistentVolumeAccessMode{
					// 该卷可以被单个节点以读/写模式挂载
					corev1.ReadWriteOnce,
				},

				PersistentVolumeSource: corev1.PersistentVolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/tmp/data",
					},
				},
				PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimDelete,
			},
		}

		err = r.Create(ctx, pv)
		if err != nil {
			logg.Error(err, "3.5 create PV error")
			return err
		}
	}

	logg.Info("3.6 create all PV success")
	return nil
}
