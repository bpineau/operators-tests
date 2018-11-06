/*
Copyright 2018 Datadog Inc..

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

package monitor

import (
	"context"
	"reflect"
	"strconv"
	"time"

	datadoghqv1alpha1 "github.com/DataDog/monop/pkg/apis/datadoghq/v1alpha1"
	"github.com/DataDog/monop/pkg/monitor"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// annotationIDKey is the name of the annotation where we save the monitor id.
	annotationIDKey = "monitor.datadoghq.com/id"

	// finalizerKey is well, the name of our Monitor finalizer
	finalizerKey = "finalizer.monitor.datadoghq.com"
)

// Add creates a new Monitor Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this datadoghq.Add(mgr) to install this Controller
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileMonitor{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("monitor-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Monitor
	err = c.Watch(&source.Kind{Type: &datadoghqv1alpha1.Monitor{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileMonitor{}

// ReconcileMonitor reconciles a Monitor object
type ReconcileMonitor struct {
	client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Monitor object and makes changes based on the state read
// and what is in the Monitor.Spec
// +kubebuilder:rbac:groups=datadoghq.datadoghq.com,resources=monitors,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileMonitor) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the Monitor instance
	instance := &datadoghqv1alpha1.Monitor{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// The object was deleted (fine, already dealt with via finalizer logic)
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// The object is being deleted; cleanup dd monitor and handle finalizer
	if !instance.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.finalize(instance)
	}

	// Create or update
	mon := datadogMonitorFromSpec(instance)
	if mon.ID == 0 {
		return r.create(instance, mon)
	}
	return r.update(instance, mon)
}

func (r *ReconcileMonitor) create(instance *datadoghqv1alpha1.Monitor, mon *monitor.DatadogMonitor) (reconcile.Result, error) {
	log := logf.Log.WithName("reconcile")

	// Create the real Datadog monitor, server side
	id, err := monitor.Save(mon)
	if err != nil {
		log.Info("Failed to create new monitor, will retry", "ns",
			instance.GetNamespace(), "name", instance.GetName())
		return reconcile.Result{Requeue: true, RequeueAfter: 30 * time.Second}, err
	}
	log.Info("New monitor", "ns",
		instance.GetNamespace(), "name", instance.GetName(), "id", id)

	// Store Datadog monitor ID as CRD annotation
	annotations := instance.GetAnnotations()
	annotations[annotationIDKey] = strconv.FormatInt(id, 10)
	instance.SetAnnotations(annotations)

	instance.Status.Phase = "Created"
	instance.Status.ID = id

	// Inject our finalizer
	if !containsString(instance.ObjectMeta.Finalizers, finalizerKey) {
		instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, finalizerKey)
	}

	err = r.Update(context.TODO(), instance)
	if err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileMonitor) update(instance *datadoghqv1alpha1.Monitor, mon *monitor.DatadogMonitor) (reconcile.Result, error) {
	log := logf.Log.WithName("reconcile")

	rmon, err := monitor.Get(mon.ID)
	if err != nil {
		switch err.(type) {
		case *monitor.ErrNotFound:
			// Monitor was deleted server side, reset id and we'll re-create it
			mon.ID = 0
			return r.create(instance, mon)
		default:
			return reconcile.Result{Requeue: true, RequeueAfter: 30 * time.Second}, err
		}
	}

	if reflect.DeepEqual(mon, rmon) {
		return reconcile.Result{}, nil
	}

	log.Info("Updating monitor", "ns",
		instance.GetNamespace(), "name", instance.GetName(), "id", mon.ID)
	if _, err = monitor.Save(mon); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (r *ReconcileMonitor) finalize(instance *datadoghqv1alpha1.Monitor) (reconcile.Result, error) {
	log := logf.Log.WithName("reconcile")

	// "You should implement the pre-delete logic in such a way
	// that it is safe to invoke it multiple times for the same object.
	if !containsString(instance.ObjectMeta.Finalizers, finalizerKey) {
		return reconcile.Result{}, nil
	}

	// delete the monitor, server side
	if id, ok := getID(instance); ok {
		log.Info("Deleting monitor", "ns", instance.GetNamespace(), "name", instance.GetName(), "id", id)
		err := monitor.Delete(id)
		if err != nil {
			// deletion failed, we'll retry later
			return reconcile.Result{Requeue: true, RequeueAfter: 30 * time.Second}, err
		}
	}

	// remove our finalizer from the list and update it.
	instance.ObjectMeta.Finalizers = removeString(instance.ObjectMeta.Finalizers, finalizerKey)
	if err := r.Update(context.Background(), instance); err != nil {
		return reconcile.Result{Requeue: true}, nil
	}

	return reconcile.Result{}, nil
}

func datadogMonitorFromSpec(instance *datadoghqv1alpha1.Monitor) *monitor.DatadogMonitor {
	m := &monitor.DatadogMonitor{
		Type:    instance.Spec.Type,
		Query:   instance.Spec.Query,
		Message: instance.Spec.Message,
		Name:    instance.Spec.Name,
	}

	m.Tags = make([]string, len(instance.Spec.Tags))
	copy(m.Tags, instance.Spec.Tags)

	if id, ok := getID(instance); ok {
		m.ID = id
	}

	m.Options = getOptions(instance)

	return m
}

func getOptions(instance *datadoghqv1alpha1.Monitor) *monitor.Options {
	// default values
	opts := &monitor.Options{
		NotifyAudit:       false,
		Locked:            false,
		NewHostDelay:      300,
		RequireFullWindow: true,
		NoDataTimeFrame:   20, // XXX in reality, this default depends on monitor type
		NotifyNoData:      false,
		RenotifyInterval:  0,
		EscalationMessage: "",
		IncludeTags:       true,
	}

	if instance.Spec.Options == nil {
		return opts
	}

	if instance.Spec.Options.NotifyAudit != nil {
		opts.NotifyAudit = *instance.Spec.Options.NotifyAudit
	}

	if instance.Spec.Options.Locked != nil {
		opts.Locked = *instance.Spec.Options.Locked
	}

	if instance.Spec.Options.NewHostDelay != nil {
		opts.NewHostDelay = *instance.Spec.Options.NewHostDelay
	}

	if instance.Spec.Options.RequireFullWindow != nil {
		opts.RequireFullWindow = *instance.Spec.Options.RequireFullWindow
	}

	if instance.Spec.Options.NoDataTimeFrame != nil {
		opts.NoDataTimeFrame = *instance.Spec.Options.NoDataTimeFrame
	}

	if instance.Spec.Options.NotifyNoData != nil {
		opts.NotifyNoData = *instance.Spec.Options.NotifyNoData
	}

	if instance.Spec.Options.RenotifyInterval != nil {
		opts.RenotifyInterval = *instance.Spec.Options.RenotifyInterval
	}

	if instance.Spec.Options.EscalationMessage != nil {
		opts.EscalationMessage = *instance.Spec.Options.EscalationMessage
	}

	if instance.Spec.Options.IncludeTags != nil {
		opts.IncludeTags = *instance.Spec.Options.IncludeTags
	}

	return opts
}

func getID(instance *datadoghqv1alpha1.Monitor) (int64, bool) {
	annot := instance.GetAnnotations()
	if sid, ok := annot[annotationIDKey]; ok {
		if id, err := strconv.ParseInt(sid, 10, 64); err == nil {
			return id, true
		}
	}
	return 0, false
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
