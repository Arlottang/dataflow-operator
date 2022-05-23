package controllers

import (
	"context"
	dataflowv1 "github.com/StepOnce7/dataflow-operator/api/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *DataflowEngineReconciler) ReconcileMaster(ctx context.Context, instance *dataflowv1.DataflowEngine, req ctrl.Request) (ctrl.Result, error) {

	logg := log.FromContext(ctx)

	logg.Info("find dataflow engine master ")

	//todo

	logg.Info("dataflow engine master reconcile success")

	return ctrl.Result{}, nil
}
