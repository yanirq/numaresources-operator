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

package serial

import (
	"context"
	"fmt"
	"time"

	"github.com/ghodss/yaml"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"

	e2efixture "github.com/openshift-kni/numaresources-operator/test/utils/fixture"
	e2enrt "github.com/openshift-kni/numaresources-operator/test/utils/noderesourcetopologies"
	"github.com/openshift-kni/numaresources-operator/test/utils/nrosched"
	"github.com/openshift-kni/numaresources-operator/test/utils/objects"
	e2ewait "github.com/openshift-kni/numaresources-operator/test/utils/objects/wait"

	nrtv1alpha1 "github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/apis/topology/v1alpha1"

	numacellapi "github.com/openshift-kni/numaresources-operator/test/deviceplugin/pkg/numacell/api"
)

var _ = Describe("[serial][disruptive][scheduler] workload placement", func() {
	var fxt *e2efixture.Fixture
	var nrtList nrtv1alpha1.NodeResourceTopologyList
	var nrts []nrtv1alpha1.NodeResourceTopology

	BeforeEach(func() {
		var err error
		fxt, err = e2efixture.Setup("e2e-test-workload-placement")
		Expect(err).ToNot(HaveOccurred(), "unable to setup test fixture")

		err = fxt.Client.List(context.TODO(), &nrtList)
		Expect(err).ToNot(HaveOccurred())

		tmPolicy := nrtv1alpha1.SingleNUMANodeContainerLevel
		nrts = e2enrt.FilterTopologyManagerPolicy(nrtList.Items, tmPolicy)
		if len(nrts) < 2 {
			Skip(fmt.Sprintf("not enough nodes with policy %q - found %d", string(tmPolicy), len(nrts)))
		}

		// Note that this test, being part of "serial", expects NO OTHER POD being scheduled
		// in between, so we consider this information current and valid when the It()s run.
	})

	AfterEach(func() {
		err := e2efixture.Teardown(fxt)
		Expect(err).NotTo(HaveOccurred())
	})

	// note we hardcode the values we need here and when we pad node.
	// This is ugly, but automatically computing the values is not straightforward
	// and will we want to start lean and mean.

	Context("cluster with at least a worker node suitable", func() {
		It("[placement][case:1] should keep the pod pending if not enough resources available, then schedule when resources are freed", func() {
			// make sure this is > 1 and LESS than required Res!
			unsuitableFreeRes := corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("2"),
				corev1.ResourceMemory: resource.MustParse("4Gi"),
			}

			requiredRes := corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4"),
				corev1.ResourceMemory: resource.MustParse("8Gi"),
			}

			By(fmt.Sprintf("creating test pod, total resources required %s", e2enrt.ResourceListToString(requiredRes)))

			// TODO: we need AT LEAST 2 (so 4, 8 is fine...) but we hardcode the padding logic to keep the test simple,
			// so we can't support ATM zones > 2. HW with zones > 2 is rare anyway, so not to big of a deal now.
			By(fmt.Sprintf("filtering available nodes with at least %d NUMA zones", 2))
			nrtCandidates := e2enrt.FilterZoneCountEqual(nrts, 2)
			if len(nrtCandidates) < 1 {
				Skip(fmt.Sprintf("not enough nodes with 2 NUMA Zones: found %d", len(nrtCandidates)))
			}
			By("filtering available nodes with allocatable resources on each NUMA zone that can match request")
			nrtCandidates = e2enrt.FilterAnyZoneMatchingResources(nrtCandidates, requiredRes)
			if len(nrtCandidates) < 1 {
				Skip(fmt.Sprintf("not enough nodes with NUMA zones each of them can match requests: found %d", len(nrtCandidates)))
			}

			candidateNodeNames := e2enrt.AccumulateNames(nrtCandidates)
			// nodes we have now are all equal for our purposes. Pick one at random
			// TODO: make sure we can control this randomness using ginkgo seed or any other way
			targetNodeName, ok := candidateNodeNames.PopAny()
			Expect(ok).To(BeTrue(), "cannot select a target node among %#v", candidateNodeNames.List())
			unsuitableNodeNames := candidateNodeNames.List()

			By(fmt.Sprintf("selecting target node %q and unsuitable nodes %#v (random pick)", targetNodeName, unsuitableNodeNames))
			var targetPaddingPods []*corev1.Pod
			var paddingPods []*corev1.Pod

			By(fmt.Sprintf("preparing target node %q to fit the test case", targetNodeName))
			// first, let's make sure that ONLY the required res can fit in either zone on the target node
			nrtInfo, err := e2enrt.FindFromList(nrtList.Items, targetNodeName)
			Expect(err).ToNot(HaveOccurred(), "missing NRT info for %q", targetNodeName)

			for _, zone := range nrtInfo.Zones {
				By(fmt.Sprintf("padding node %q zone %q", nrtInfo.Name, zone.Name))
				padPod, err := makePaddingPod(fxt.Namespace.Name, "target", zone, requiredRes)
				Expect(err).ToNot(HaveOccurred())

				padPod, err = pinPodTo(padPod, nrtInfo.Name, zone.Name)
				Expect(err).ToNot(HaveOccurred())

				err = fxt.Client.Create(context.TODO(), padPod)
				Expect(err).ToNot(HaveOccurred())
				paddingPods = append(paddingPods, padPod)
			}

			failedPods := e2ewait.ForPodListAllRunning(fxt.Client, paddingPods)
			for _, failedPod := range failedPods {
				// ignore errors intentionally
				_ = objects.LogEventsForPod(fxt.K8sClient, failedPod.Namespace, failedPod.Name)
			}
			Expect(failedPods).To(BeEmpty())

			for _, zone := range nrtInfo.Zones {
				By(fmt.Sprintf("making node %q zone %q unsuitable with a placeholder pod", nrtInfo.Name, zone.Name))
				Expect(err).ToNot(HaveOccurred(), "cannot detect the zone ID from %q", zone.Name)
				// now put a minimal pod (1 cpu 1Gi) on both zones. Now the target node as whole will still have the
				// required resources, but no NUMA zone individually will
				targetedPaddingPod := objects.NewTestPodPause(fxt.Namespace.Name, fmt.Sprintf("tgtpadpod-%s", zone.Name))
				targetedPaddingPod.Spec.NodeName = nrtInfo.Name
				targetedPaddingPod.Spec.Containers[0].Resources.Limits = corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				}

				targetedPaddingPod, err = pinPodTo(targetedPaddingPod, nrtInfo.Name, zone.Name)
				Expect(err).ToNot(HaveOccurred())

				err = fxt.Client.Create(context.TODO(), targetedPaddingPod)
				Expect(err).ToNot(HaveOccurred())
				targetPaddingPods = append(targetPaddingPods, targetedPaddingPod)
			}

			failedPods = e2ewait.ForPodListAllRunning(fxt.Client, targetPaddingPods)
			for _, failedPod := range failedPods {
				// ignore errors intentionally
				_ = objects.LogEventsForPod(fxt.K8sClient, failedPod.Namespace, failedPod.Name)
			}
			Expect(failedPods).To(BeEmpty())

			By("saturating nodes we want to be unsuitable")
			for idx, unsuitableNodeName := range unsuitableNodeNames {
				nrtInfo, err := e2enrt.FindFromList(nrtList.Items, unsuitableNodeName)
				Expect(err).ToNot(HaveOccurred(), "missing NRT info for %q", unsuitableNodeName)

				for _, zone := range nrtInfo.Zones {
					name := fmt.Sprintf("unsuitable%d", idx)
					By(fmt.Sprintf("saturating node %q -> %q zone %q", nrtInfo.Name, name, zone.Name))
					padPod, err := makePaddingPod(fxt.Namespace.Name, name, zone, unsuitableFreeRes)
					Expect(err).ToNot(HaveOccurred())

					padPod, err = pinPodTo(padPod, nrtInfo.Name, zone.Name)
					Expect(err).ToNot(HaveOccurred())

					err = fxt.Client.Create(context.TODO(), padPod)
					Expect(err).ToNot(HaveOccurred())
					paddingPods = append(paddingPods, padPod)
				}
			}

			allPaddingPods := append([]*corev1.Pod{}, paddingPods...)
			allPaddingPods = append(allPaddingPods, targetPaddingPods...)
			By("waiting for ALL padding pods to go running - or fail")
			failedPods = e2ewait.ForPodListAllRunning(fxt.Client, allPaddingPods)
			for _, failedPod := range failedPods {
				// ignore errors intentionally
				_ = objects.LogEventsForPod(fxt.K8sClient, failedPod.Namespace, failedPod.Name)
			}
			Expect(failedPods).To(BeEmpty())

			// TODO: smarter cooldown
			time.Sleep(18 * time.Second)
			dumpNRTForNode(fxt.Client, targetNodeName)

			By(fmt.Sprintf("running the test pod requiring: %s", e2enrt.ResourceListToString(requiredRes)))
			pod := objects.NewTestPodPause(fxt.Namespace.Name, "testpod")
			pod.Spec.SchedulerName = schedulerName
			pod.Spec.Containers[0].Resources.Limits = requiredRes
			pod.Spec.NodeSelector = map[string]string{
				multiNUMALabel: "2",
			}
			err = fxt.Client.Create(context.TODO(), pod)
			Expect(err).ToNot(HaveOccurred())

			By("check the pod is still pending")
			// TODO: lacking better ways, let's monitor the pod "long enough" and let's check it stays Pending
			// if it stays Pending "long enough" it still means little, but OTOH if it goes Running or Failed we
			// can tell for sure something's wrong
			err = e2ewait.WhileInPodPhase(fxt.Client, pod.Namespace, pod.Name, corev1.PodPending, 10*time.Second, 3)
			if err != nil {
				_ = objects.LogEventsForPod(fxt.K8sClient, pod.Namespace, pod.Name)
			}
			Expect(err).ToNot(HaveOccurred())

			By("deleting a placeholder pod pod") // any pod is fine
			targetPaddingPod := targetPaddingPods[0]
			err = fxt.Client.Delete(context.TODO(), targetPaddingPod)
			Expect(err).ToNot(HaveOccurred())

			By("checking the test pod is removed")
			err = e2ewait.ForPodDeleted(fxt.Client, targetPaddingPod.Namespace, targetPaddingPod.Name, 3*time.Minute)
			Expect(err).ToNot(HaveOccurred())

			By("waiting for the pod to be scheduled")
			updatedPod, err := e2ewait.ForPodPhase(fxt.Client, pod.Namespace, pod.Name, corev1.PodRunning, 3*time.Minute)
			Expect(err).ToNot(HaveOccurred())

			By(fmt.Sprintf("checking the pod landed on the target node %q vs %q", updatedPod.Spec.NodeName, targetNodeName))
			Expect(updatedPod.Spec.NodeName).To(Equal(targetNodeName),
				"node landed on %q instead of on %v", updatedPod.Spec.NodeName, targetNodeName)

			By(fmt.Sprintf("checking the pod was scheduled with the topology aware scheduler %q", schedulerName))
			schedOK, err := nrosched.CheckPODWasScheduledWith(fxt.K8sClient, updatedPod.Namespace, updatedPod.Name, schedulerName)
			Expect(err).ToNot(HaveOccurred())
			Expect(schedOK).To(BeTrue(), "pod %s/%s not scheduled with expected scheduler %s", updatedPod.Namespace, updatedPod.Name, schedulerName)
		})
	})

	Context("cluster with multiple worker nodes suitable", func() {
		It("[placement][test_id:47575] should make a pod with two gu containers land on a node with enough resources on a specific NUMA zone, each container on a different zone", func() {
			hostsRequired := 2

			pod := objects.NewTestPodPause(fxt.Namespace.Name, "testpod")
			pod.Spec.SchedulerName = schedulerName
			pod.Spec.NodeSelector = map[string]string{
				multiNUMALabel: "2",
			}
			pod.Spec.Containers = append(pod.Spec.Containers, pod.Spec.Containers[0])
			pod.Spec.Containers[0].Resources.Limits = corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("6"),
				corev1.ResourceMemory: resource.MustParse("6Gi"),
			}
			pod.Spec.Containers[1].Resources.Limits = corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("12"),
				corev1.ResourceMemory: resource.MustParse("8Gi"),
			}
			requiredRes := e2enrt.ResourcesFromGuaranteedPod(*pod)

			// make sure the sum is equal to the sum of the requirement of the test pod,
			// so the *node* total free resources are equal between the target node and
			// the unsuitable nodes
			unsuitableFreeRes := []corev1.ResourceList{
				{
					corev1.ResourceCPU:    resource.MustParse("2"),
					corev1.ResourceMemory: resource.MustParse("2Gi"),
				},
				{
					corev1.ResourceCPU:    resource.MustParse("16"),
					corev1.ResourceMemory: resource.MustParse("12Gi"),
				},
			}

			// TODO: we need AT LEAST 2 (so 4, 8 is fine...) but we hardcode the padding logic to keep the test simple,
			// so we can't support ATM zones > 2. HW with zones > 2 is rare anyway, so not to big of a deal now.
			By(fmt.Sprintf("filtering available nodes with at least %d NUMA zones", 2))
			nrtCandidates := e2enrt.FilterZoneCountEqual(nrts, 2)
			if len(nrtCandidates) < hostsRequired {
				Skip(fmt.Sprintf("not enough nodes with 2 NUMA Zones: found %d", len(nrtCandidates)))
			}
			By("filtering available nodes with allocatable resources on each NUMA zone that can match request")
			nrtCandidates = e2enrt.FilterAnyZoneMatchingResources(nrtCandidates, requiredRes)
			if len(nrtCandidates) < hostsRequired {
				Skip(fmt.Sprintf("not enough nodes with NUMA zones each of them can match requests: found %d", len(nrtCandidates)))
			}

			candidateNodeNames := e2enrt.AccumulateNames(nrtCandidates)
			// nodes we have now are all equal for our purposes. Pick one at random
			// TODO: make sure we can control this randomness using ginkgo seed or any other way
			targetNodeName, ok := candidateNodeNames.PopAny()
			Expect(ok).To(BeTrue(), "cannot select a target node among %#v", candidateNodeNames.List())
			unsuitableNodeNames := candidateNodeNames.List()

			By(fmt.Sprintf("selecting target node %q and unsuitable nodes %#v (random pick)", targetNodeName, unsuitableNodeNames))

			By(fmt.Sprintf("preparing target node %q to fit the test case", targetNodeName))
			// first, let's make sure that ONLY the required res can fit in either zone on the target node
			nrtInfo, err := e2enrt.FindFromList(nrtList.Items, targetNodeName)
			Expect(err).ToNot(HaveOccurred(), "missing NRT info for %q", targetNodeName)

			// if we get this far we can now depend on the fact that len(nrt.Zones) == len(pod.Spec.Containers) == 2
			var paddingPods []*corev1.Pod

			for idx := 0; idx < 2; idx++ {
				zone := nrtInfo.Zones[idx]
				cnt := pod.Spec.Containers[1-idx] // switch requirements intentionally - both couplings are legit anyway

				By(fmt.Sprintf("padding node %q zone %q to fit only %s", nrtInfo.Name, zone.Name, e2enrt.ResourceListToString(cnt.Resources.Limits)))
				padPod, err := makePaddingPod(fxt.Namespace.Name, "target", zone, cnt.Resources.Limits)
				Expect(err).ToNot(HaveOccurred())

				padPod, err = pinPodTo(padPod, nrtInfo.Name, zone.Name)
				Expect(err).ToNot(HaveOccurred())

				err = fxt.Client.Create(context.TODO(), padPod)
				Expect(err).ToNot(HaveOccurred())
				paddingPods = append(paddingPods, padPod)
			}

			// still working under the assumption that len(nrt.Zones) == len(pod.Spec.Containers) == 2
			for nodeIdx, unsuitableNodeName := range unsuitableNodeNames {
				nrtInfo, err := e2enrt.FindFromList(nrtList.Items, unsuitableNodeName)
				Expect(err).ToNot(HaveOccurred(), "missing NRT info for %q", unsuitableNodeName)

				for zoneIdx, zone := range nrtInfo.Zones {
					padRes := unsuitableFreeRes[zoneIdx]

					name := fmt.Sprintf("unsuitable%d", nodeIdx)
					By(fmt.Sprintf("saturating node %q -> %q zone %q to fit only %s", nrtInfo.Name, name, zone.Name, e2enrt.ResourceListToString(padRes)))
					padPod, err := makePaddingPod(fxt.Namespace.Name, name, zone, padRes)
					Expect(err).ToNot(HaveOccurred())

					padPod, err = pinPodTo(padPod, nrtInfo.Name, zone.Name)
					Expect(err).ToNot(HaveOccurred())

					err = fxt.Client.Create(context.TODO(), padPod)
					Expect(err).ToNot(HaveOccurred())
					paddingPods = append(paddingPods, padPod)
				}
			}

			By("waiting for ALL padding pods to go running - or fail")
			failedPods := e2ewait.ForPodListAllRunning(fxt.Client, paddingPods)
			for _, failedPod := range failedPods {
				// ignore errors intentionally
				_ = objects.LogEventsForPod(fxt.K8sClient, failedPod.Namespace, failedPod.Name)
			}
			Expect(failedPods).To(BeEmpty())

			// TODO: smarter cooldown
			time.Sleep(18 * time.Second)
			dumpNRTForNode(fxt.Client, targetNodeName)

			By("checking the resource allocation as the test starts")
			nrtListInitial, err := e2enrt.GetUpdated(fxt.Client, nrtList, 1*time.Minute)
			Expect(err).ToNot(HaveOccurred())

			By("running the test pod")
			err = fxt.Client.Create(context.TODO(), pod)
			Expect(err).ToNot(HaveOccurred())

			By("waiting for the pod to be scheduled")
			updatedPod, err := e2ewait.ForPodPhase(fxt.Client, pod.Namespace, pod.Name, corev1.PodRunning, 2*time.Minute)
			Expect(err).ToNot(HaveOccurred())

			By(fmt.Sprintf("checking the pod landed on the target node %q vs %q", updatedPod.Spec.NodeName, targetNodeName))
			Expect(updatedPod.Spec.NodeName).To(Equal(targetNodeName),
				"node landed on %q instead of on %v", updatedPod.Spec.NodeName, targetNodeName)

			By(fmt.Sprintf("checking the pod was scheduled with the topology aware scheduler %q", schedulerName))
			schedOK, err := nrosched.CheckPODWasScheduledWith(fxt.K8sClient, updatedPod.Namespace, updatedPod.Name, schedulerName)
			Expect(err).ToNot(HaveOccurred())
			Expect(schedOK).To(BeTrue(), "pod %s/%s not scheduled with expected scheduler %s", updatedPod.Namespace, updatedPod.Name, schedulerName)

			By(fmt.Sprintf("checking the resources are accounted as expected on %q", updatedPod.Spec.NodeName))
			nrtListPostCreate, err := e2enrt.GetUpdated(fxt.Client, nrtList, 1*time.Minute)
			Expect(err).ToNot(HaveOccurred())

			nrtInitial, err := e2enrt.FindFromList(nrtListInitial.Items, updatedPod.Spec.NodeName)
			Expect(err).ToNot(HaveOccurred())
			nrtPostCreate, err := e2enrt.FindFromList(nrtListPostCreate.Items, updatedPod.Spec.NodeName)
			Expect(err).ToNot(HaveOccurred())

			// TODO: this is only partially correct. We should check with NUMA zone granularity (not with NODE granularity)
			_, err = e2enrt.CheckZoneConsumedResourcesAtLeast(*nrtInitial, *nrtPostCreate, requiredRes)
			Expect(err).ToNot(HaveOccurred())

			By("deleting the test pod")
			err = fxt.Client.Delete(context.TODO(), updatedPod)
			Expect(err).ToNot(HaveOccurred())

			By("checking the test pod is removed")
			err = e2ewait.ForPodDeleted(fxt.Client, updatedPod.Namespace, updatedPod.Name, 3*time.Minute)
			Expect(err).ToNot(HaveOccurred())

			// the NRT updaters MAY be slow to react for a number of reasons including factors out of our control
			// (kubelet, runtime). This is a known behaviour. We can only tolerate some delay in reporting on pod removal.
			Eventually(func() bool {
				By(fmt.Sprintf("checking the resources are restored as expected on %q", updatedPod.Spec.NodeName))

				nrtListPostDelete, err := e2enrt.GetUpdated(fxt.Client, nrtListPostCreate, 1*time.Minute)
				Expect(err).ToNot(HaveOccurred())

				nrtPostDelete, err := e2enrt.FindFromList(nrtListPostDelete.Items, updatedPod.Spec.NodeName)
				Expect(err).ToNot(HaveOccurred())

				ok, err := e2enrt.CheckEqualAvailableResources(*nrtInitial, *nrtPostDelete)
				Expect(err).ToNot(HaveOccurred())
				return ok
			}, time.Minute, time.Second*5).Should(BeTrue(), "resources not restored on %q", updatedPod.Spec.NodeName)
		})
	})
	Context("with at least two nodes suitable", func() {
		It("[test_id: 47583] a guaranteed pod with one container should be scheduled into one NUMA zone", func() {

			const requiredNUMAZones = 2
			By(fmt.Sprintf("filtering available nodes with at least %d NUMA zones", requiredNUMAZones))
			nrtCandidates := e2enrt.FilterZoneCountEqual(nrts, requiredNUMAZones)

			const neededNodes = 2
			if len(nrtCandidates) < neededNodes {
				Skip(fmt.Sprintf("not enough nodes with 2 NUMA Zones: found %d, needed %d", len(nrtCandidates), neededNodes))
			}

			//TODO: we should calculate requiredRes from NUMA zones in cluster nodes instead.
			requiredRes := corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4"),
				corev1.ResourceMemory: resource.MustParse("4Gi"),
			}

			By("filtering available nodes with allocatable resources on at least one NUMA zone that can match request")
			nrtCandidates = e2enrt.FilterAnyZoneMatchingResources(nrtCandidates, requiredRes)
			if len(nrtCandidates) < neededNodes {
				Skip(fmt.Sprintf("not enough nodes with NUMA zones each of them can match requests: found %d, needed: %d", len(nrtCandidates), neededNodes))
			}
			nrtCandidateNames := e2enrt.AccumulateNames(nrtCandidates)

			targetNodeName, ok := nrtCandidateNames.PopAny()
			Expect(ok).To(BeTrue(), "cannot select a targe node among %#v", nrtCandidateNames.List())
			By(fmt.Sprintf("selecting node to schedule the pod: %q", targetNodeName))
			// need to prepare all the other nodes so they cannot have any one NUMA zone with enough resources
			// but have enough allocatable resources at node level to shedule the pod on it.
			// If we pad each zone with a pod with 3/4 of the required resources, as those nodes have at least
			// 2 NUMA zones, they will have enogh allocatable resources at node level to accomodate the required
			// resources but they won't have enough resources in only one NUMA zone.

			By("Padding all other candidate nodes")
			// TODO This should be calculated as 3/4 of requiredRes
			paddingRes := corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("3"),
				corev1.ResourceMemory: resource.MustParse("3Gi"),
			}

			var paddingPods []*corev1.Pod
			for _, nodeName := range nrtCandidateNames.List() {

				nrtInfo, err := e2enrt.FindFromList(nrtCandidates, nodeName)
				Expect(err).NotTo(HaveOccurred(), "missing NRT info for %q", nodeName)

				for idx, zone := range nrtInfo.Zones {
					podName := fmt.Sprintf("padding%s-%d", nodeName, idx)
					padPod, err := makePaddingPod(fxt.Namespace.Name, podName, zone, paddingRes)
					Expect(err).NotTo(HaveOccurred(), "unable to create padding pod %q on zone", podName, zone.Name)

					padPod, err = pinPodTo(padPod, nodeName, zone.Name)
					Expect(err).NotTo(HaveOccurred(), "unable to pin pod %q to zone", podName, zone.Name)

					err = fxt.Client.Create(context.TODO(), padPod)
					Expect(err).NotTo(HaveOccurred(), "unable to create pod %q on zone", podName, zone.Name)

					paddingPods = append(paddingPods, padPod)
				}
			}

			// wait for all padding pods to be up&running ( or fail)
			failedPods := e2ewait.ForPodListAllRunning(fxt.Client, paddingPods)
			for _, failedPod := range failedPods {
				// no need to check for errors here
				_ = objects.LogEventsForPod(fxt.K8sClient, failedPod.Namespace, failedPod.Name)
			}
			Expect(failedPods).To(BeEmpty(), "some padding pods have failed to run")

			By("Scheduling the testing pod")
			pod := objects.NewTestPodPause(fxt.Namespace.Name, "testPod")
			pod.Spec.SchedulerName = schedulerName
			pod.Spec.Containers[0].Resources.Limits = requiredRes

			err := fxt.Client.Create(context.TODO(), pod)
			Expect(err).NotTo(HaveOccurred(), "unable to create pod %q", pod.Name)

			By("waiting for node to be up&running")
			podRunningTimeout := 1 * time.Minute
			updatedPod, err := e2ewait.ForPodPhase(fxt.Client, pod.Namespace, pod.Name, corev1.PodRunning, podRunningTimeout)
			Expect(err).NotTo(HaveOccurred(), "Pod %q not up&running after %v", pod.Name, podRunningTimeout)

			By("checking the pod has been scheduled in the proper node")
			Expect(updatedPod.Spec.NodeName).To(Equal(targetNodeName))

			By(fmt.Sprintf("checking the pod was scheduled with the topology aware scheduler %q", schedulerName))
			schedOK, err := nrosched.CheckPODWasScheduledWith(fxt.K8sClient, updatedPod.Namespace, updatedPod.Name, schedulerName)
			Expect(err).ToNot(HaveOccurred())
			Expect(schedOK).To(BeTrue(), "pod %s/%s not scheduled with expected scheduler %s", updatedPod.Namespace, updatedPod.Name, schedulerName)
		})
	})
})

func makePaddingPod(namespace, nodeName string, zone nrtv1alpha1.Zone, podReqs corev1.ResourceList) (*corev1.Pod, error) {
	klog.Infof("want to have zone %q with allocatable: %s", zone.Name, e2enrt.ResourceListToString(podReqs))

	paddingReqs, err := e2enrt.SaturateZoneUntilLeft(zone, podReqs)
	if err != nil {
		return nil, err
	}

	klog.Infof("padding resource to saturate %q: %s", nodeName, e2enrt.ResourceListToString(paddingReqs))

	padPod := objects.NewTestPodPause(namespace, fmt.Sprintf("padpod-%s-%s", nodeName, zone.Name))
	padPod.Spec.Containers[0].Resources.Limits = paddingReqs
	return padPod, nil
}

func pinPodTo(pod *corev1.Pod, nodeName, zoneName string) (*corev1.Pod, error) {
	zoneID, err := e2enrt.GetZoneIDFromName(zoneName)
	if err != nil {
		return nil, err
	}
	klog.Infof("creating padding pod for node %q zone %d", nodeName, zoneID)

	klog.Infof("forcing affinity to [kubernetes.io/hostname: %s]", nodeName)
	pod.Spec.NodeSelector = map[string]string{
		"kubernetes.io/hostname": nodeName,
	}
	cnt := &pod.Spec.Containers[0] // shortcut
	cnt.Resources.Limits[numacellapi.MakeResourceName(zoneID)] = resource.MustParse("1")
	return pod, nil
}

func dumpNRTForNode(cli client.Client, nodeName string) {
	nrt := nrtv1alpha1.NodeResourceTopology{}
	err := cli.Get(context.TODO(), client.ObjectKey{Name: nodeName}, &nrt)
	Expect(err).ToNot(HaveOccurred())
	data, err := yaml.Marshal(nrt)
	Expect(err).ToNot(HaveOccurred())
	klog.Infof("NRT for node %q:\n%s", nodeName, data)
}
