package controllers

import (
	"context"
	dataflowv1 "github.com/StepOnce7/dataflow-operator/api/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *DataflowEngineReconciler) ReconcileMaster(ctx context.Context, instance *dataflowv1.DataflowEngine, req ctrl.Request) (ctrl.Result, error) {

	return ctrl.Result{}, nil

}
