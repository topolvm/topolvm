package controllers

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/topolvm/topolvm"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// NodeReconciler reconciles a Node object
type NodeReconciler struct {
	client.Client
	APIReader client.Reader
}

func (r *NodeReconciler) RunOnce(ctx context.Context) error {
	nodes := &corev1.NodeList{}
	err := r.List(ctx, nodes)
	switch {
	case err == nil:
	case apierrors.IsNotFound(err):
		return nil
	default:
		return err
	}

	for _, node := range nodes.Items {
		_, err := r.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{
				Namespace: node.Namespace,
				Name:      node.Name,
			},
		})

		if err != nil {
			return err
		}
	}

	return nil
}

// Reconcile finalize Node
func (r *NodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := crlog.FromContext(ctx)
	node := &corev1.Node{}
	err := r.Get(ctx, req.NamespacedName, node)
	switch {
	case err == nil:
	case apierrors.IsNotFound(err):
		return ctrl.Result{}, nil
	default:
		return ctrl.Result{}, err
	}

	if node.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	var changed bool
	newNode := node.DeepCopy()
	for _, m := range []migrator{migrateFinalizer, migrateCapacity} {
		n, err := m(r, newNode)
		if err != nil {
			return ctrl.Result{}, err
		}

		if n != nil {
			changed = true
			newNode = n
		}
	}

	if !changed {
		log.Info("skip migration", "name", node.Name)
		return ctrl.Result{}, nil
	}

	log.Info("migrate node", "name", node.Name)
	if err := r.Update(ctx, newNode); err != nil {
		log.Error(err, "failed to migrate finalizer", "name", node.Name)
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

type migrator func(*NodeReconciler, *corev1.Node) (*corev1.Node, error)

func migrateFinalizer(r *NodeReconciler, node *corev1.Node) (*corev1.Node, error) {
	finalizers := []string{}
	var found bool
	for _, f := range node.Finalizers {
		if f != topolvm.LegacyNodeFinalizer {
			finalizers = append(finalizers, f)
		}

		if f == topolvm.NodeFinalizer {
			found = true
		}
	}

	if !found {
		finalizers = append(finalizers, topolvm.NodeFinalizer)
	}
	if reflect.DeepEqual(finalizers, node.Finalizers) {
		return nil, nil
	}

	newNode := node.DeepCopy()
	newNode.Finalizers = finalizers
	return newNode, nil
}

func migrateCapacity(r *NodeReconciler, node *corev1.Node) (*corev1.Node, error) {
	var changed bool
	annotations := map[string]string{}

	for key, val := range node.Annotations {
		dc := strings.SplitAfter(key, topolvm.LegacyCapacityKeyPrefix)
		if len(dc) == 1 && dc[0] != "" {
			changed = true
			annotations[fmt.Sprintf("%s%s", topolvm.CapacityKeyPrefix, dc)] = val
		} else {
			annotations[key] = val
		}
	}

	if changed {
		newNode := node.DeepCopy()
		newNode.Annotations = annotations
		return newNode, nil
	}

	return nil, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	pred := predicate.Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return true },
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		UpdateFunc:  func(event.UpdateEvent) bool { return true },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(pred).
		For(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}
