package controllers

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func createPVIfNotExists(pv *corev1.PersistentVolume) {

	pv.Spec = corev1.PersistentVolumeSpec{
		StorageClassName: "mysql",
		Capacity: corev1.ResourceList{
			corev1.ResourceStorage: resource.MustParse("5Gi"),
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
	}
}
