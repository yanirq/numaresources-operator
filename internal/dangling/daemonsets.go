/*
 * Copyright 2021 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package dangling

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	"sigs.k8s.io/controller-runtime/pkg/client"

	nropv1 "github.com/openshift-kni/numaresources-operator/api/numaresourcesoperator/v1"
	nodegroupv1 "github.com/openshift-kni/numaresources-operator/api/numaresourcesoperator/v1/helper/nodegroup"
	"github.com/openshift-kni/numaresources-operator/pkg/objectnames"
)

func DaemonSetKeys(cli client.Client, ctx context.Context, instance *nropv1.NUMAResourcesOperator) ([]client.ObjectKey, error) {
	trees, err := getTreesByNodeGroup(ctx, cli, instance.Spec.NodeGroups)
	if err != nil {
		return nil, err
	}
	objs, err := DaemonSets(cli, ctx, instance, trees)
	if err != nil {
		return nil, err
	}
	keys := make([]client.ObjectKey, 0, len(objs))
	for _, obj := range objs {
		obj2 := obj
		keys = append(keys, client.ObjectKeyFromObject(&obj2))
	}
	return keys, nil
}

func DaemonSets(cli client.Client, ctx context.Context, instance *nropv1.NUMAResourcesOperator, trees []nodegroupv1.Tree) ([]appsv1.DaemonSet, error) {
	var daemonSetList appsv1.DaemonSetList
	if err := cli.List(ctx, &daemonSetList, &client.ListOptions{Namespace: instance.Namespace}); err != nil {
		return nil, err
	}

	expectedDaemonSetNames := sets.NewString()
	for _, tree := range trees {
		for _, mcp := range tree.MachineConfigPools {
			expectedDaemonSetNames = expectedDaemonSetNames.Insert(objectnames.GetComponentName(instance.Name, mcp.Name))
		}
	}

	var dangling []appsv1.DaemonSet
	for _, ds := range daemonSetList.Items {
		if expectedDaemonSetNames.Has(ds.Name) {
			continue
		}
		if !isOwnedBy(ds.GetObjectMeta(), instance) {
			continue
		}
		dangling = append(dangling, ds)
	}
	return dangling, nil
}
