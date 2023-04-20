/*
Copyright 2023.

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

package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dbv1alpha1 "github.com/c-t-n/postgres-dev-operator/api/v1alpha1"
	"github.com/go-logr/logr"
)

// PostgresTableReconciler reconciles a PostgresTable object
type PostgresTableReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const postgresTableFinalizer = "db.c-t-n/deletionFinalizer"

//+kubebuilder:rbac:groups=db.c-t-n,resources=postgrestables,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=db.c-t-n,resources=postgrestables/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=db.c-t-n,resources=postgrestables/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the PostgresTable object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *PostgresTableReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the PostgresTable Resource
	psqlTable := &dbv1alpha1.PostgresTable{}
	// psqlDbList := &dbv1alpha1.PostgresDatabaseList{}

	// >> Fetch Table
	err := r.Get(ctx, req.NamespacedName, psqlTable)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// // >> Get Database Info
	// err = r.List(ctx, psqlDbList)
	// if err != nil {
	// 	return ctrl.Result{}, err
	// }
	// if len(psqlDbList.Items) == 0 {
	// 	logger.Info("No Postgres Database Found, retrying in 10 seconds")
	// 	// return ctrl.Result{RequeueAfter: time.Duration(time.Duration.Seconds(10))}, err
	// }

	isMarkedForDeletion := psqlTable.GetDeletionTimestamp() != nil
	if isMarkedForDeletion {
		if controllerutil.ContainsFinalizer(psqlTable, postgresTableFinalizer) {
			// Run finalization logic for psqlTableFinalizer. If the
			// finalization logic fails, don't remove the finalizer so
			// that we can retry during the next reconciliation.
			if err := r.FinalizePostgresTable(logger, psqlTable); err != nil {
				return ctrl.Result{}, err
			}

			// Remove psqlTableFinalizer. Once all finalizers have been
			// removed, the object will be deleted.
			controllerutil.RemoveFinalizer(psqlTable, postgresTableFinalizer)
			err := r.Update(ctx, psqlTable)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// add Finalizer
	if !controllerutil.ContainsFinalizer(psqlTable, postgresTableFinalizer) {
		controllerutil.AddFinalizer(psqlTable, postgresTableFinalizer)
		err = r.Update(ctx, psqlTable)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil

}

// CleanUp Finalizer
func (r *PostgresTableReconciler) FinalizePostgresTable(logger logr.Logger, p *dbv1alpha1.PostgresTable) error {
	logger.Info("Finalized")
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PostgresTableReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dbv1alpha1.PostgresTable{}).
		Complete(r)
}
