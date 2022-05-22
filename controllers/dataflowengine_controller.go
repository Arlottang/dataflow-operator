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
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dataflowv1 "github.com/StepOnce7/dataflow-operator/api/v1"
)

const (
	FRAME_STANDALONE          = "mysql-standalone"
	USER_STANDALONE           = "etcd-sample"
	CONTAINER_PORT            = 3306
	pvFinalizer               = "kubernetes.io/pv-protection"
	EtcdClusterCommonLabelKey = "storage"
	EtcdDataDirName           = "datadir"
	EtcdClusterLabelKey       = "etcd-standalone"
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

	logg.Info("2.3 get dataflow engine instance success : " + instance.String())

	logg.Info("3 start create pv process")
	if err := createPVIfNotExists(ctx, r, instance, req); err != nil {
		logg.Error(err, "3.7 handle all pv error")
		return ctrl.Result{}, err
	}

	logg.Info("4 start frame standalone reconcile logic")
	result, err := r.ReconcileFrameStandalone(ctx, instance, req)

	if err != nil {
		logg.Error(err, "4.4 frame standalone reconcile error")
		return result, err
	}

	logg.Info("5 start user standalone reconcile logic")
	result, err = r.ReconcileUserStandalone(ctx, instance, req)

	if err != nil {
		logg.Error(err, "5.3 user standalone reconcile error")
		return result, err
	}

	logg.Info(fmt.Sprintf("6. Finalizers info : [%v]", instance.Finalizers))

	if !instance.DeletionTimestamp.IsZero() {
		logg.Info("Start delete Finalizers for PV")
		return ctrl.Result{}, r.PVFinalizer(ctx, instance)
	}

	logg.Info("7 dataflow engine reconcile success")

	return ctrl.Result{}, nil
}

func (r *DataflowEngineReconciler) PVFinalizer(ctx context.Context, de *dataflowv1.DataflowEngine) error {

	de.Finalizers = removeString(de.Finalizers, pvFinalizer)
	return r.Client.Update(ctx, de)
}

// SetupWithManager sets up the controller with the Manager.
func (r *DataflowEngineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dataflowv1.DataflowEngine{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
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
