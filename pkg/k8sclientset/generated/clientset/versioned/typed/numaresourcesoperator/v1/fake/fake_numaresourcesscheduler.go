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
 * Copyright 2023 Red Hat, Inc.
 */

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	numaresourcesoperatorv1 "github.com/openshift-kni/numaresources-operator/api/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeNUMAResourcesSchedulers implements NUMAResourcesSchedulerInterface
type FakeNUMAResourcesSchedulers struct {
	Fake *FakeNumaresourcesoperatorV1
}

var numaresourcesschedulersResource = schema.GroupVersionResource{Group: "numaresourcesoperator", Version: "v1", Resource: "numaresourcesschedulers"}

var numaresourcesschedulersKind = schema.GroupVersionKind{Group: "numaresourcesoperator", Version: "v1", Kind: "NUMAResourcesScheduler"}

// Get takes name of the nUMAResourcesScheduler, and returns the corresponding nUMAResourcesScheduler object, and an error if there is any.
func (c *FakeNUMAResourcesSchedulers) Get(ctx context.Context, name string, options v1.GetOptions) (result *numaresourcesoperatorv1.NUMAResourcesScheduler, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(numaresourcesschedulersResource, name), &numaresourcesoperatorv1.NUMAResourcesScheduler{})
	if obj == nil {
		return nil, err
	}
	return obj.(*numaresourcesoperatorv1.NUMAResourcesScheduler), err
}

// List takes label and field selectors, and returns the list of NUMAResourcesSchedulers that match those selectors.
func (c *FakeNUMAResourcesSchedulers) List(ctx context.Context, opts v1.ListOptions) (result *numaresourcesoperatorv1.NUMAResourcesSchedulerList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(numaresourcesschedulersResource, numaresourcesschedulersKind, opts), &numaresourcesoperatorv1.NUMAResourcesSchedulerList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &numaresourcesoperatorv1.NUMAResourcesSchedulerList{ListMeta: obj.(*numaresourcesoperatorv1.NUMAResourcesSchedulerList).ListMeta}
	for _, item := range obj.(*numaresourcesoperatorv1.NUMAResourcesSchedulerList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested nUMAResourcesSchedulers.
func (c *FakeNUMAResourcesSchedulers) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(numaresourcesschedulersResource, opts))
}

// Create takes the representation of a nUMAResourcesScheduler and creates it.  Returns the server's representation of the nUMAResourcesScheduler, and an error, if there is any.
func (c *FakeNUMAResourcesSchedulers) Create(ctx context.Context, nUMAResourcesScheduler *numaresourcesoperatorv1.NUMAResourcesScheduler, opts v1.CreateOptions) (result *numaresourcesoperatorv1.NUMAResourcesScheduler, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(numaresourcesschedulersResource, nUMAResourcesScheduler), &numaresourcesoperatorv1.NUMAResourcesScheduler{})
	if obj == nil {
		return nil, err
	}
	return obj.(*numaresourcesoperatorv1.NUMAResourcesScheduler), err
}

// Update takes the representation of a nUMAResourcesScheduler and updates it. Returns the server's representation of the nUMAResourcesScheduler, and an error, if there is any.
func (c *FakeNUMAResourcesSchedulers) Update(ctx context.Context, nUMAResourcesScheduler *numaresourcesoperatorv1.NUMAResourcesScheduler, opts v1.UpdateOptions) (result *numaresourcesoperatorv1.NUMAResourcesScheduler, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(numaresourcesschedulersResource, nUMAResourcesScheduler), &numaresourcesoperatorv1.NUMAResourcesScheduler{})
	if obj == nil {
		return nil, err
	}
	return obj.(*numaresourcesoperatorv1.NUMAResourcesScheduler), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeNUMAResourcesSchedulers) UpdateStatus(ctx context.Context, nUMAResourcesScheduler *numaresourcesoperatorv1.NUMAResourcesScheduler, opts v1.UpdateOptions) (*numaresourcesoperatorv1.NUMAResourcesScheduler, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(numaresourcesschedulersResource, "status", nUMAResourcesScheduler), &numaresourcesoperatorv1.NUMAResourcesScheduler{})
	if obj == nil {
		return nil, err
	}
	return obj.(*numaresourcesoperatorv1.NUMAResourcesScheduler), err
}

// Delete takes name of the nUMAResourcesScheduler and deletes it. Returns an error if one occurs.
func (c *FakeNUMAResourcesSchedulers) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteActionWithOptions(numaresourcesschedulersResource, name, opts), &numaresourcesoperatorv1.NUMAResourcesScheduler{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeNUMAResourcesSchedulers) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(numaresourcesschedulersResource, listOpts)

	_, err := c.Fake.Invokes(action, &numaresourcesoperatorv1.NUMAResourcesSchedulerList{})
	return err
}

// Patch applies the patch and returns the patched nUMAResourcesScheduler.
func (c *FakeNUMAResourcesSchedulers) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *numaresourcesoperatorv1.NUMAResourcesScheduler, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(numaresourcesschedulersResource, name, pt, data, subresources...), &numaresourcesoperatorv1.NUMAResourcesScheduler{})
	if obj == nil {
		return nil, err
	}
	return obj.(*numaresourcesoperatorv1.NUMAResourcesScheduler), err
}
