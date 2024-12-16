/*
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
 *
 * Copyright 2021 Red Hat, Inc.
 */

package rte

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	rtemanifests "github.com/k8stopologyawareschedwg/deployer/pkg/manifests/rte"
	machineconfigv1 "github.com/openshift/machine-config-operator/pkg/apis/machineconfiguration.openshift.io/v1"

	nropv1 "github.com/openshift-kni/numaresources-operator/api/numaresourcesoperator/v1"
	nodegroupv1 "github.com/openshift-kni/numaresources-operator/api/numaresourcesoperator/v1/helper/nodegroup"
	"github.com/openshift-kni/numaresources-operator/pkg/objectnames"
	"github.com/openshift-kni/numaresources-operator/pkg/objectstate"
	"github.com/openshift-kni/numaresources-operator/pkg/objectstate/compare"
	"github.com/openshift-kni/numaresources-operator/pkg/objectstate/merge"
)

func updateFromClientTreeMachineConfigPool(ret *ExistingManifests, ctx context.Context, cli client.Client, instance *nropv1.NUMAResourcesOperator, tree nodegroupv1.Tree, namespace string) {
	for _, mcp := range tree.MachineConfigPools {
		generatedName := objectnames.GetComponentName(instance.Name, mcp.Name)
		key := client.ObjectKey{
			Name:      generatedName,
			Namespace: namespace,
		}
		ds := &appsv1.DaemonSet{}
		dsm := daemonSetManifest{}
		if dsm.daemonSetError = cli.Get(ctx, key, ds); dsm.daemonSetError == nil {
			dsm.daemonSet = ds
		}
		ret.daemonSets[generatedName] = dsm

		mcName := objectnames.GetMachineConfigName(instance.Name, mcp.Name)
		mckey := client.ObjectKey{
			Name: mcName,
		}
		mc := &machineconfigv1.MachineConfig{}
		mcm := machineConfigManifest{}
		if mcm.machineConfigError = cli.Get(ctx, mckey, mc); mcm.machineConfigError == nil {
			mcm.machineConfig = mc
		}
		ret.machineConfigs[mcName] = mcm
	}
}

func stateFromMachineConfigPools(em *ExistingManifests, mf rtemanifests.Manifests, tree nodegroupv1.Tree) []objectstate.ObjectState {
	var ret []objectstate.ObjectState
	for _, mcp := range tree.MachineConfigPools {
		var existingDs client.Object
		var loadError error

		generatedName := objectnames.GetComponentName(em.instance.Name, mcp.Name)
		existingDaemonSet, ok := em.daemonSets[generatedName]
		if ok {
			existingDs = existingDaemonSet.daemonSet
			loadError = existingDaemonSet.daemonSetError
		} else {
			loadError = fmt.Errorf("failed to find daemon set %s/%s", mf.DaemonSet.Namespace, mf.DaemonSet.Name)
		}

		desiredDaemonSet := mf.DaemonSet.DeepCopy()
		desiredDaemonSet.Name = generatedName

		var updateError error
		if mcp.Spec.NodeSelector != nil {
			desiredDaemonSet.Spec.Template.Spec.NodeSelector = mcp.Spec.NodeSelector.MatchLabels
		} else {
			updateError = fmt.Errorf("the machine config pool %q does not have node selector", mcp.Name)
		}

		gdm := GeneratedDesiredManifest{
			ClusterPlatform:       em.plat,
			MachineConfigPool:     mcp.DeepCopy(),
			NodeGroup:             tree.NodeGroup.DeepCopy(),
			DaemonSet:             desiredDaemonSet,
			IsCustomPolicyEnabled: em.customPolicyEnabled,
		}

		err := em.updater(mcp.Name, &gdm)
		if err != nil {
			updateError = fmt.Errorf("daemonset for MCP %q: update failed: %w", mcp.Name, err)
		}

		ret = append(ret, objectstate.ObjectState{
			Existing:    existingDs,
			Error:       loadError,
			UpdateError: updateError,
			Desired:     desiredDaemonSet,
			Compare:     compare.Object,
			Merge:       merge.ObjectForUpdate,
		})
	}
	return ret
}
