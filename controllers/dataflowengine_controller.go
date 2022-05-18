/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strconv"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dataflowv1 "github.com/StepOnce7/dataflow-operator/api/v1"
)

const (
	FRAME_STANDALONE          = "mysql-standalone"
	USER_STANDALONE           = "etcd-standalone"
	CONTAINER_PORT            = 3306
	pvFinalizer               = "kubernetes.io/pv-protection"
	EtcdClusterLabelKey       = "etcd.io/cluster"
	EtcdClusterCommonLabelKey = "storage-etcd"
	EtcdDataDirName           = "datadir"
)

// DataflowEngineReconciler reconciles a DataflowEngine object
type DataflowEngineReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=dataflow.pingcap.com,resources=dataflowengines,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dataflow.pingcap.com,resources=dataflowengines/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dataflow.pingcap.com,resources=dataflowengines/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the DataflowEngine object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *DataflowEngineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logg := log.FromContext(ctx)

	logg.Info("1 start dataflow engine reconcile logic ")

	instance := &dataflowv1.DataflowEngine{}

	logg.Info("2 find dataflow engine instance")
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		if errors.IsNotFound(err) {
			logg.Info("2.1 dataflow engine instance is not found")
			return ctrl.Result{}, nil
		}

		logg.Error(err, "2.2 find dataflow engine instance error")
		return ctrl.Result{}, err
	}

	logg.Info("3 get dataflow engine instance success : " + instance.String())

	deployment := &appsv1.Deployment{}

	logg.Info("4 find mysql deployment")
	err := r.Get(ctx, req.NamespacedName, deployment)

	if err != nil {
		if errors.IsNotFound(err) {
			logg.Info("4.1 deployment is not exists")

			if err = createPVIfNotExists(ctx, r, instance, req); err != nil {
				logg.Error(err, "4.1.a create PV error")
				return ctrl.Result{}, err
			}

			if err = createPVCIfNotExists(ctx, r, instance, req); err != nil {
				logg.Error(err, "4.1.b create PVC error")
				return ctrl.Result{}, nil
			}

			if err = createServiceIfNotExists(ctx, r, instance, req); err != nil {
				logg.Error(err, "4.1.c create mysql service error")
				return ctrl.Result{}, err
			}

			if err = createDeploymentIfNotExists(ctx, r, instance, req); err != nil {
				logg.Error(err, "4.1.d create mysql deployment error")
				return ctrl.Result{}, err
			}

		} else {
			logg.Error(err, "4.2 get deployment error")
			return ctrl.Result{}, err
		}
	}

	statefulSet := &appsv1.StatefulSet{}

	logg.Info("5 find etcd statefulSet")
	err = r.Get(ctx, req.NamespacedName, statefulSet)
	if err != nil {
		if errors.IsNotFound(err) {
			logg.Info("5.1 etcd statefulSet is not exists")

			svc := corev1.Service{}
			svc.Name = instance.Spec.UserStandalone.Name
			svc.Namespace = instance.Namespace

			or, err := ctrl.CreateOrUpdate(ctx, r, &svc, func() error {
				MutateHeadlessSvc(instance, &svc)
				return controllerutil.SetControllerReference(instance, &svc, r.Scheme)
			})

			if err != nil {
				logg.Error(err, "5.2 create etcd service error")
				return ctrl.Result{}, err
			}

			logg.WithValues("CreateOrUpdate", "Service", or)

			logg.Info("5.3 start create statefulSet")

			var sts appsv1.StatefulSet
			sts.Name = instance.Spec.UserStandalone.Name
			sts.Namespace = instance.Namespace

			or, err = ctrl.CreateOrUpdate(ctx, r, &sts, func() error {
				MutateStatefulSet(instance, &sts)
				return controllerutil.SetControllerReference(instance, &sts, r.Scheme)
			})

			if err != nil {
				logg.Error(err, "5.4 create etcd statefulSet error")
				return ctrl.Result{}, err
			}
			logg.WithValues("CreateOrUpdate", "StatefulSet", or)

			logg.Info("5.5 create statefulSet success")
		} else {
			logg.Error(err, "5.6 get etcd statefulSet error")
			return ctrl.Result{}, err
		}
	}

	logg.Info(fmt.Sprintf("6. Finalizers info : [%v]", instance.Finalizers))

	if !instance.DeletionTimestamp.IsZero() {
		logg.Info("Start delete Finalizers for PV")
		return ctrl.Result{}, r.PVFinalizer(ctx, instance)
	}

	//if !containsString(instance.Finalizers, pvFinalizer) {
	//	logg.Info("add pv Finalizer")
	//	instance.Finalizers = append(instance.Finalizers, pvFinalizer)
	//	if err = r.Client.Update(ctx, instance); err != nil {
	//		logg.Error(err, "add pv Finalizer error")
	//		return ctrl.Result{}, err
	//	}
	//
	//}

	logg.Info("7 dataflow engine reconcile success")

	return ctrl.Result{}, nil
}

func createPVCIfNotExists(ctx context.Context, r *DataflowEngineReconciler, dataflow *dataflowv1.DataflowEngine, req ctrl.Request) error {
	logg := log.FromContext(ctx)
	logg.Info("b.1 start create pvc for pc")

	pvc := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, req.NamespacedName, pvc)

	if err == nil {
		logg.Info("b.2 pvc exists")
		return nil
	}

	if !errors.IsNotFound(err) {
		logg.Error(err, "b.3 query pvc error")
		return err
	}

	scn := "manual"
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
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
		},
	}

	logg.Info("b.4 set pvc reference")

	err = controllerutil.SetControllerReference(dataflow, pvc, r.Scheme)
	if err != nil {
		logg.Error(err, "b.5 set controller reference error, about pvc")
		return err
	}

	logg.Info("b.6 start create PVC")
	err = r.Create(ctx, pvc)

	if err != nil {
		logg.Error(err, "b.7 create PVC error")
		return err
	}

	logg.Info("b.8 create PVC success")

	return nil

}

func createPVIfNotExists(ctx context.Context, r *DataflowEngineReconciler, dataflow *dataflowv1.DataflowEngine, req ctrl.Request) error {
	logg := log.FromContext(ctx)

	logg.Info("a.1 start create PV if not exists")

	pv := &corev1.PersistentVolume{}

	// todo: pv may be exists system,
	err := r.Get(ctx, req.NamespacedName, pv)
	if err == nil || errors.IsAlreadyExists(err) {
		logg.Info("a.2 pv exists")
		return nil
	}

	if !errors.IsNotFound(err) {
		logg.Error(err, "a.3 query pv error")
		return err
	}

	pv = &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: dataflow.Namespace,
			Name:      "mysql-pv-volume",
			Labels: map[string]string{
				"type": "local",
			},
		},
		Spec: corev1.PersistentVolumeSpec{
			StorageClassName: "manual",
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("10Gi"),
			},
			AccessModes: []corev1.PersistentVolumeAccessMode{
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

	logg.Info("a.4 start create PV")
	err = r.Create(ctx, pv)

	if err != nil {
		logg.Error(err, "a.5 create PV error")
		return err
	}

	logg.Info("a.6 create PV success")
	return nil
}

func createServiceIfNotExists(ctx context.Context, r *DataflowEngineReconciler, dataflow *dataflowv1.DataflowEngine, req ctrl.Request) error {
	logg := log.FromContext(ctx)

	logg.Info("c.1 start create mysql service if not exists")

	service := &corev1.Service{}

	err := r.Get(ctx, req.NamespacedName, service)
	if err == nil {
		logg.Info("c.2 mysql service exists")
		return nil
	}

	if !errors.IsNotFound(err) {
		logg.Error(err, "c.3 query mysql service error")
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

	logg.Info("c.4 set mysql service reference")

	if err = controllerutil.SetControllerReference(dataflow, service, r.Scheme); err != nil {
		logg.Error(err, "c.5 set mysql service reference error")
		return err
	}

	logg.Info("c.6 start create mysql service")

	if err = r.Create(ctx, service); err != nil {
		logg.Error(err, "c.7 create mysql service error")
		return err
	}

	logg.Info("c.8 create mysql service success")
	return nil
}

func createDeploymentIfNotExists(ctx context.Context, r *DataflowEngineReconciler, dataflow *dataflowv1.DataflowEngine, req ctrl.Request) error {
	logg := log.FromContext(ctx)
	logg.Info("d.1 start create deployment for mysql")

	deployment := &appsv1.Deployment{}

	err := r.Get(ctx, req.NamespacedName, deployment)
	if err == nil {
		logg.Info("d.2 mysql deployment exists")
		return nil
	}

	if !errors.IsNotFound(err) {
		logg.Error(err, "d.3 query mysql deployment error")
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

	logg.Info("d.4 set mysql deployment reference")
	if err = controllerutil.SetControllerReference(dataflow, deployment, r.Scheme); err != nil {
		logg.Error(err, "d.5 set mysql deployment reference error")
		return err
	}

	logg.Info("d.6 start create deployment")
	if err = r.Create(ctx, deployment); err != nil {
		logg.Error(err, "d.7 create mysql deployment error")
		return err
	}

	logg.Info("d.8 create mysql deployment success")
	return nil
}

func (r *DataflowEngineReconciler) PVFinalizer(ctx context.Context, de *dataflowv1.DataflowEngine) error {

	de.Finalizers = removeString(de.Finalizers, pvFinalizer)
	return r.Client.Update(ctx, de)
}

// SetupWithManager sets up the controller with the Manager.
func (r *DataflowEngineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dataflowv1.DataflowEngine{}).
		Complete(r)
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

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
