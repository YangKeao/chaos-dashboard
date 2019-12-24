// Copyright 2019 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package collector

import (
	"context"
	"fmt"
	"github.com/YangKeao/chaos-dashboard/pkg/api_interface"
	"github.com/go-logr/logr"
	"github.com/pingcap/chaos-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"os"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

var (
	initLog            = ctrl.Log.WithName("setup")
	dashboardNamespace string
	dataSource         string
)

func init() {
	var ok bool

	dashboardNamespace, ok = os.LookupEnv("NAMESPACE")
	if !ok {
		initLog.Error(nil, "cannot find NAMESPACE")
		dashboardNamespace = "chaos"
	}

	dataSource = fmt.Sprintf("root:@tcp(chaos-collector-database.%s:3306)/chaos_operator", dashboardNamespace)
}

type ChaosCollector struct {
	client.Client
	Log            logr.Logger
	apiType        runtime.Object
	databaseClient *DatabaseClient
}

func (r *ChaosCollector) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	if r.apiType == nil {
		r.Log.Error(nil, "apiType has not been initialized")
		return ctrl.Result{}, nil
	}
	ctx := context.Background()

	obj, ok := r.apiType.DeepCopyObject().(api_interface.StatefulObject)
	if !ok {
		r.Log.Error(nil, "it's not a stateful object")
		return ctrl.Result{}, nil
	}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		r.Log.Error(err, "unable to get chaos")
		return ctrl.Result{}, nil
	}

	status := obj.GetStatus()

	affected_namespace := make(map[string]bool)
	for _, pod := range status.Experiment.Pods {
		affected_namespace[pod.Namespace] = true
	}

	for namespace := range affected_namespace {
		err := r.EnsureTidbNamespaceHasGrafana(namespace)
		if err != nil {
			r.Log.Error(err, "check grafana for tidb cluster failed")
		}
	}

	if status.Experiment.Phase == v1alpha1.ExperimentPhaseRunning {
		event := Event{
			Name:              req.Name,
			Namespace:         req.Namespace,
			Type:              reflect.TypeOf(obj).Elem().Name(),
			AffectedNamespace: affected_namespace,
			StartTime:         &status.Experiment.StartTime.Time,
			EndTime:           nil,
		}
		r.Log.Info("event started, save to database", "event", event)

		err := r.databaseClient.WriteEvent(event)
		if err != nil {
			r.Log.Error(err, "write event to database error")
			return ctrl.Result{}, nil
		}
	} else if status.Experiment.Phase == v1alpha1.ExperimentPhaseFinished {
		event := Event{
			Name:              req.Name,
			Namespace:         req.Namespace,
			Type:              reflect.TypeOf(obj).Elem().Name(),
			AffectedNamespace: affected_namespace,
			StartTime:         &status.Experiment.StartTime.Time,
			EndTime:           &status.Experiment.EndTime.Time,
		}
		r.Log.Info("event finished, save to database", "event", event)

		err := r.databaseClient.UpdateEvent(event)
		if err != nil {
			r.Log.Error(err, "write event to database error")
			return ctrl.Result{}, nil
		}
	}
	return ctrl.Result{}, nil
}

func (r *ChaosCollector) Setup(mgr ctrl.Manager, apiType runtime.Object) error {
	r.apiType = apiType

	databaseClient, err := NewDatabaseClient(dataSource)
	if err != nil {
		r.Log.Error(err, "create database client failed")
		return nil
	}

	r.databaseClient = databaseClient
	return ctrl.NewControllerManagedBy(mgr).
		For(apiType).
		Complete(r)
}

func (r *ChaosCollector) EnsureTidbNamespaceHasGrafana(namespace string) error {
	var svcList corev1.ServiceList

	var listOptions = client.ListOptions{}
	listOptions.Namespace = namespace
	err := r.List(context.Background(), &svcList, &listOptions)
	if err != nil {
		r.Log.Error(err, "error while getting all services", "namespace", namespace)
	}

	for _, service := range svcList.Items {
		if strings.Contains(service.Name, "prometheus") {
			ok, err := r.IsGrafanaSetUp(service.Name, service.Namespace)
			if err != nil {
				r.Log.Error(err, "error while getting grafana")
				return err
			}

			if !ok {
				err := r.SetupGrafana(service.Name, service.Namespace, service.Spec.Ports[0].Port) // This zero index is unsafe hack. TODO: use a better way to get port
				if err != nil {
					r.Log.Error(err, "error while creating grafana")
					return err
				}
				r.Log.Info("create grafana successfully", "name", service.Name, "namespace", service.Namespace, "port", service.Spec.Ports[0].Port) // This zero index is unsafe hack TODO: use a better way to get port
			}
			break
		}
	}

	return nil
}

func (r *ChaosCollector) IsGrafanaSetUp(name string, namespace string) (bool, error) {
	var deploymentList v1.DeploymentList

	var listOptions = client.ListOptions{}
	listOptions.Namespace = dashboardNamespace
	err := r.List(context.Background(), &deploymentList, &listOptions)
	if err != nil {
		r.Log.Error(err, "error while getting all deployments", "namespace", dashboardNamespace)
	}

	result := false
	for _, deployment := range deploymentList.Items {
		if strings.Contains(deployment.Name, namespace) && strings.Contains(deployment.Name, "-chaos-grafana") {
			result = true
		}
	}

	return result, nil
}

func (r *ChaosCollector) SetupGrafana(name string, namespace string, port int32) error {
	var deployment v1.Deployment

	deployment.Namespace = dashboardNamespace
	deployment.Name = fmt.Sprintf("%s-%s-chaos-grafana", namespace, name)

	labels := map[string]string{
		"app.kubernetes.io/name": deployment.Name,
		"prometheus/name":        name,
		"prometheus/namespace":   namespace,
	}

	var chaosDashboard v1.Deployment
	err := r.Get(context.Background(), types.NamespacedName{
		Namespace: dashboardNamespace,
		Name:      "chaos-collector",
	}, &chaosDashboard)
	if err != nil {
		return err
	}
	uid := chaosDashboard.UID

	deployment.Labels = labels
	deployment.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: labels,
	}
	deployment.Spec.Template.Labels = labels
	blockOwnerDeletion := true
	deployment.OwnerReferences = append(deployment.OwnerReferences, metav1.OwnerReference{
		BlockOwnerDeletion: &blockOwnerDeletion,
		Name:               "chaos-collector",
		Kind:               "Deployment",
		APIVersion:         "apps/v1beta1",
		UID:                uid,
	})

	var container corev1.Container
	container.Name = "grafana"
	container.Image = "grafana/grafana:master-ubuntu"
	deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, container)

	r.Log.Info("creating grafana deployments")
	return r.Create(context.Background(), &deployment)
}
