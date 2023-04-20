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
	"fmt"
	"strings"

	b64 "encoding/base64"

	dbv1alpha1 "github.com/c-t-n/postgres-dev-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// PostgresDatabaseReconciler reconciles a PostgresDatabase object
type PostgresDatabaseReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

var (
	defaultPostgresVersion string = "12-alpine"
	defaultReplicaCount    int32  = 1
)

//+kubebuilder:rbac:groups=db.c-t-n,resources=postgresdatabases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=db.c-t-n,resources=postgresdatabases/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=db.c-t-n,resources=postgresdatabases/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the PostgresDatabase object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *PostgresDatabaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the PostgresDatabase Resource
	postgresDatabase := &dbv1alpha1.PostgresDatabase{}
	err := r.Get(ctx, req.NamespacedName, postgresDatabase)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Create the deployment if it doesn't exists
	found := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: postgresDatabase.Name, Namespace: postgresDatabase.Namespace}, found)
	if err != nil {
		if errors.IsNotFound(err) {
			// Define and create a new deployment.
			sec := r.generatePsqlSecrets(postgresDatabase)
			if err = r.Create(ctx, sec); err != nil {
				return ctrl.Result{}, err
			}

			svc := r.generatePsqlService(postgresDatabase)
			if err = r.Create(ctx, svc); err != nil {
				return ctrl.Result{}, err
			}

			dep := r.generatePsqlDatabaseDeploymentObject(postgresDatabase, sec)
			if err = r.Create(ctx, dep); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		} else {
			return ctrl.Result{}, err
		}
	}

	// Verify Replicas
	size := postgresDatabase.Spec.Replicas
	if *found.Spec.Replicas != size {
		found.Spec.Replicas = &size
		if err = r.Update(ctx, found); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	version := postgresDatabase.Spec.Version
	currentImageName := found.Spec.Template.Spec.Containers[0].Image
	if !strings.Contains(currentImageName, fmt.Sprintf(":%s", version)) {
		logger.Info(fmt.Sprintf("Version of %s changed to %s", found.ObjectMeta.Name, version))
		found.Spec.Template.Spec.Containers[0].Image = fmt.Sprintf("postgres:%s", version)
		if err = r.Update(ctx, found); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PostgresDatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dbv1alpha1.PostgresDatabase{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}

func (r *PostgresDatabaseReconciler) generatePsqlSecrets(p *dbv1alpha1.PostgresDatabase) *corev1.Secret {
	user := generatePassword(16)
	password := generatePassword(32)

	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.Name,
			Namespace: p.Namespace,
		},
		Data: map[string][]byte{
			"POSTGRES_USER":     []byte(b64.StdEncoding.EncodeToString(user)),
			"POSTGRES_PASSWORD": []byte(b64.StdEncoding.EncodeToString(password)),
		},
	}
	controllerutil.SetControllerReference(p, sec, r.Scheme)
	return sec
}

func (r *PostgresDatabaseReconciler) generatePsqlDatabaseDeploymentObject(p *dbv1alpha1.PostgresDatabase, sec *corev1.Secret) *appsv1.Deployment {
	lbls := generateLabelsForDeployment(p.Name)
	version := p.Spec.Version
	if version == "" {
		version = defaultPostgresVersion
	}

	replicas := p.Spec.Replicas
	if replicas == 0 {
		replicas = defaultReplicaCount
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.Name,
			Namespace: p.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: lbls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: lbls,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "postgres",
						Image: fmt.Sprintf("postgres:%s", version),
						Ports: []corev1.ContainerPort{{
							ContainerPort: 5678,
							Name:          "postgresql",
						}},
						EnvFrom: []corev1.EnvFromSource{{
							SecretRef: &corev1.SecretEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: sec.Name,
								},
							},
						}},
					}},
				},
			},
		},
	}

	controllerutil.SetControllerReference(p, dep, r.Scheme)
	return dep
}

func (r *PostgresDatabaseReconciler) generatePsqlService(p *dbv1alpha1.PostgresDatabase) *corev1.Service {
	lbls := generateLabelsForDeployment(p.Name)

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.Name,
			Namespace: p.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: lbls,
			Type:     corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       "postgres",
					Protocol:   "TCP",
					Port:       5678,
					TargetPort: intstr.FromString("postgresql"),
				},
			},
		},
	}

	controllerutil.SetControllerReference(p, svc, r.Scheme)
	return svc
}

func generateLabelsForDeployment(name string) map[string]string {
	return map[string]string{"cr_name": name}
}
