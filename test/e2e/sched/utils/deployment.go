/*
Copyright 2022 The Kubernetes Authors.

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

package utils

import (
	"context"
	"fmt"

	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"

	e2eclient "github.com/openshift-kni/numaresources-operator/test/utils/clients"
)

func GetDeploymentByOwnerReference(uid types.UID) (*v1.Deployment, error) {
	deployList := &v1.DeploymentList{}

	if err := e2eclient.Client.List(context.TODO(), deployList); err != nil {
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}

	for _, deploy := range deployList.Items {
		for _, or := range deploy.GetOwnerReferences() {
			if or.UID == uid {
				return &deploy, nil
			}
		}
	}
	return nil, fmt.Errorf("failed to get deployment with uid: %s", uid)
}
