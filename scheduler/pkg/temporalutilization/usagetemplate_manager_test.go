package temporalutilization

import (
	"context"
	"fmt"
	"testing"

	fakeclientset "gitee.com/openeuler/paws/scheduler/pkg/generated/clientset/versioned/fake"
	pawsinformers "gitee.com/openeuler/paws/scheduler/pkg/generated/informers/externalversions"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	clientsetfake "k8s.io/client-go/kubernetes/fake"
	st "k8s.io/kubernetes/pkg/scheduler/testing"
	testutil "sigs.k8s.io/scheduler-plugins/test/util"
)

type UpdateItem struct {
	old *v1.Pod
	new *v1.Pod
}

func TestUsageTemplateManagerCacheUpdateLogic(t *testing.T) {
	testNode1 := "node-1"
	testNode2 := "node-2"

	tests := []struct {
		name                     string
		nodePodsMap              map[string][]NamespacedPod
		podsToUpdate             []*UpdateItem
		expectedTotalCacheSize   int
		expectedPerNodeCacheSize map[string]int
		expectedNodeCachePods    map[string][]string
	}{
		{
			name: "OnUpdate doesn't add unassign pods",
			nodePodsMap: map[string][]NamespacedPod{
				testNode1: {
					{"default", "Pod-1"},
				},
			},
			podsToUpdate: []*UpdateItem{
				{
					old: st.MakePod().Name("Pod-4").Obj(),
					new: st.MakePod().Name("Pod-4").Obj(),
				},
			},
			expectedTotalCacheSize: 1,
			expectedPerNodeCacheSize: map[string]int{
				testNode1: 1,
			},
			expectedNodeCachePods: map[string][]string{
				testNode1: {"Pod-1"},
			},
		},
		{
			name: "OnUpdate add pods if assigned to node",
			nodePodsMap: map[string][]NamespacedPod{
				testNode1: {
					{"default", "Pod-1"},
					{"default", "Pod-2"},
				},
				testNode2: {
					{"default", "Pod-3"},
				},
			},
			podsToUpdate: []*UpdateItem{
				{
					old: st.MakePod().Name("Pod-4").Node("").Obj(),
					new: st.MakePod().Name("Pod-4").Node(testNode2).Obj(),
				},
			},
			expectedTotalCacheSize: 4,
			expectedPerNodeCacheSize: map[string]int{
				testNode1: 2,
				testNode2: 2,
			},
			expectedNodeCachePods: map[string][]string{
				testNode1: {"Pod-1", "Pod-2"},
				testNode2: {"Pod-3", "Pod-4"},
			},
		},
		{
			name: "OnUpdate update the cache, if it shifted to another node",
			nodePodsMap: map[string][]NamespacedPod{
				testNode1: {
					{"default", "Pod-1"},
				},
			},
			podsToUpdate: []*UpdateItem{
				{
					old: st.MakePod().Name("Pod-1").Namespace("default").Node(testNode1).Obj(),
					new: st.MakePod().Name("Pod-1").Namespace("default").Node(testNode2).Obj(),
				},
			},
			expectedTotalCacheSize: 1,
			expectedPerNodeCacheSize: map[string]int{
				testNode2: 1,
			},
			expectedNodeCachePods: map[string][]string{
				testNode2: {"Pod-1"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			cs := fakeclientset.NewSimpleClientset()
			pawsInformerFactory := pawsinformers.NewSharedInformerFactory(cs, 0)
			utInformer := pawsInformerFactory.Scheduling().V1alpha1().UsageTemplates()
			pawsInformerFactory.Start(ctx.Done())
			snapshot := testutil.NewFakeSharedLister(nil, nil)

			fakeClient := clientsetfake.NewSimpleClientset()
			informerFactory := informers.NewSharedInformerFactory(fakeClient, 0)
			podInformer := informerFactory.Core().V1().Pods()
			informerFactory.Start(ctx.Done())

			mgr := NewUsageTemplateManager(cs, snapshot, utInformer, podInformer)
			mgr.NodePodsCache = tt.nodePodsMap
			for _, item := range tt.podsToUpdate {
				mgr.OnUpdate(item.old, item.new)
			}

			assert.Equal(t, tt.expectedTotalCacheSize, mgr.CacheSize())
			for node, size := range tt.expectedPerNodeCacheSize {
				assert.Equal(t, size, len(mgr.NodePodsCache[node]))
			}

			for node, pods := range mgr.NodePodsCache {
				for _, expectedPod := range tt.expectedNodeCachePods[node] {
					found := false
					for _, inCache := range pods {
						if inCache.Name == expectedPod {
							found = true
							break
						}
					}
					if !found {
						assert.Fail(t, fmt.Sprintf("Pod %v does not exists in Node %v cache", expectedPod, node))
					}
				}
			}
		})
	}
}
