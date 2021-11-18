/*


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
	"time"

	fluxksv1beta2 "github.com/fluxcd/kustomize-controller/api/v1beta2"
	"github.com/go-logr/logr"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	v1beta2 "github.com/raffis/ceasing-kustomize-controller/api/v1beta2"
)

const (
	// secretIndexKey is the key used for indexing CeasingKustomizations based on
	// their secrets.
	kustoimizationIndexKey string = ".metadata.name"
)

// CeasingKustomization reconciles a CeasingKustomization object
type CeasingKustomizationReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

type CeasingKustomizationReconcilerOptions struct {
	MaxConcurrentReconciles int
}

// SetupWithManager adding controllers
func (r *CeasingKustomizationReconciler) SetupWithManager(mgr ctrl.Manager, opts CeasingKustomizationReconcilerOptions) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta2.CeasingKustomization{}).
		Watches(
			&source.Kind{Type: &fluxksv1beta2.Kustomization{}},
			&handler.EnqueueRequestForOwner{
				IsController: true,
				OwnerType:    &v1beta2.CeasingKustomization{},
			},
		).
		WithOptions(controller.Options{MaxConcurrentReconciles: opts.MaxConcurrentReconciles}).
		Complete(r)
}

// +kubebuilder:rbac:groups=kustomize.raffis.github.io,resources=CeasingKustomizations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kustomize.raffis.github.io,resources=CeasingKustomizations/status,verbs=get;update;patch

// Reconcile CeasingKustomizations
func (r *CeasingKustomizationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("Namespace", req.Namespace, "Name", req.NamespacedName)
	logger.Info("reconciling CeasingKustomization")

	// Fetch the CeasingKustomization instance
	ck := v1beta2.CeasingKustomization{}
	err := r.Client.Get(ctx, req.NamespacedName, &ck)

	if err != nil {
		if kerrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// examine DeletionTimestamp to determine if object is under deletion
	if ck.ObjectMeta.DeletionTimestamp.IsZero() {
		if !containsString(ck.GetFinalizers(), v1beta2.FinalizerName) {
			controllerutil.AddFinalizer(&ck, v1beta2.FinalizerName)
			if err := r.Update(ctx, &ck); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	ck, result, reconcileErr := r.reconcile(ctx, ck)

	// Update status after reconciliation.
	if err = r.patchStatus(ctx, &ck); err != nil {
		logger.Error(err, "unable to update status after reconciliation")
		return ctrl.Result{Requeue: true}, err
	}

	return result, reconcileErr
}

func (r *CeasingKustomizationReconciler) reconcile(ctx context.Context, ck v1beta2.CeasingKustomization) (v1beta2.CeasingKustomization, ctrl.Result, error) {
	// Fetch referencing secret
	ks := &fluxksv1beta2.Kustomization{}
	ksName := types.NamespacedName{
		Namespace: ck.GetNamespace(),
		Name:      ck.GetName(),
	}

	err := r.Client.Get(context.TODO(), ksName, ks)
	expired := (ck.GetCreationTimestamp().Unix() + int64(ck.Spec.TTL.Seconds())) <= time.Now().Unix()

	switch {
	// CeasingKustomization is being deleted, cleanup and remove our finalizer
	case !ck.ObjectMeta.DeletionTimestamp.IsZero():
		return r.finalize(ctx, ck, ks)

	// CeasingKustomization is expired and Kustomization is already removed, don't requeue
	case err != nil && expired == true:
		return ck, ctrl.Result{Requeue: false}, nil

	// CeasingKustomization is expired and Kustomization is not yet removed, remove it
	case err == nil && expired == true:
		return r.cleanup(ctx, ck, ks)

	// Kustomization does not exists, create from template
	case err != nil && expired == false:
		ck.Spec.KustomizationTemplate.Spec.Prune = true
		tpl := fluxksv1beta2.Kustomization{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ck.GetName(),
				Namespace: ck.GetNamespace(),
			},
			Spec: ck.Spec.KustomizationTemplate.Spec,
		}

		err := r.Client.Create(context.TODO(), &tpl)
		if err != nil {
			msg := fmt.Sprintf("failed to create kustomization: %s", err.Error())
			r.Recorder.Event(&ck, "Normal", "error", msg)
			return v1beta2.NotReady(ck, v1beta2.FailedCreateKustomizationReason, msg), ctrl.Result{Requeue: true}, err
		}

		ck.Status.LastReconciliationRunTime = &metav1.Time{Time: time.Now()}

		msg := "kustomization created successfully"
		r.Recorder.Event(&ck, "Normal", "info", msg)

		//Reconcile after ttl is reached
		return v1beta2.Ready(ck, v1beta2.KustomizationCreatedReason, msg), ctrl.Result{
			RequeueAfter: ck.ObjectMeta.CreationTimestamp.Add(ck.Spec.TTL.Duration).Sub(time.Now()),
		}, err

		// CeasingKustomization wants to manage a kustomization which is not owned, abort
	case err != nil && ck.Status.LastReconciliationRunTime == nil:
		msg := fmt.Sprintf("targeting kustomization does already exist")
		r.Recorder.Event(&ck, "Normal", "error", msg)
		return v1beta2.NotReady(ck, v1beta2.KustomizationAlreadyExistsReason, msg), ctrl.Result{Requeue: true}, err

	// Kustomization exists, update from template (if we have changes)
	default:
		ck.Spec.KustomizationTemplate.Spec.Prune = true
		tpl := fluxksv1beta2.Kustomization{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ck.GetName(),
				Namespace: ck.GetNamespace(),
			},
			Spec: ck.Spec.KustomizationTemplate.Spec,
		}

		err = r.Client.Patch(ctx, ks, client.MergeFrom(&tpl))
	}

	// Reconcile after ttl is reached
	return ck, ctrl.Result{
		RequeueAfter: ck.ObjectMeta.CreationTimestamp.Add(ck.Spec.TTL.Duration).Sub(time.Now()),
	}, err
}

func (r *CeasingKustomizationReconciler) cleanup(ctx context.Context, ck v1beta2.CeasingKustomization, ks *fluxksv1beta2.Kustomization) (v1beta2.CeasingKustomization, ctrl.Result, error) {
	err := r.Client.Delete(context.TODO(), ks)

	if err != nil && !kerrors.IsNotFound(err) {
		msg := fmt.Sprintf("failed to remove kustomization: %s", err.Error())
		r.Recorder.Event(&ck, "Normal", "error", msg)
		return v1beta2.NotReady(ck, v1beta2.FailedDeleteKustomizationReason, msg), ctrl.Result{Requeue: false}, err
	}

	msg := fmt.Sprintf("removed ceased kustomization")
	r.Recorder.Event(&ck, "Normal", "error", msg)
	return v1beta2.NotReady(ck, v1beta2.KustomizationExpiredReason, msg), ctrl.Result{Requeue: false}, err
}

func (r *CeasingKustomizationReconciler) finalize(ctx context.Context, ck v1beta2.CeasingKustomization, ks *fluxksv1beta2.Kustomization) (v1beta2.CeasingKustomization, ctrl.Result, error) {
	if !containsString(ck.GetFinalizers(), v1beta2.FinalizerName) {
		return ck, ctrl.Result{}, nil
	}

	var err error
	// Only attempt a delete if the kustomization is a valid reference
	if ks.GetName() != "" {
		err = r.Client.Delete(context.TODO(), ks)
	}

	if err != nil {
		return ck, ctrl.Result{}, err
	}

	controllerutil.RemoveFinalizer(&ck, v1beta2.FinalizerName)

	err = r.Update(ctx, &ck)
	return ck, ctrl.Result{}, err
}

func (r *CeasingKustomizationReconciler) patchStatus(ctx context.Context, ck *v1beta2.CeasingKustomization) error {
	key := client.ObjectKeyFromObject(ck)
	latest := &v1beta2.CeasingKustomization{}
	if err := r.Client.Get(ctx, key, latest); err != nil {
		return err
	}

	return r.Client.Status().Patch(ctx, ck, client.MergeFrom(latest))
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
