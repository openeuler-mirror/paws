package temporalutilization

// Copyright (c) Huawei Technologies Co., Ltd. 2023-2024. All rights reserved.
// PAWS licensed under the Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//     http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR
// PURPOSE.
// See the Mulan PSL v2 for more details.
// Author: Wei Wei; Gingfung Yeung
// Date: 2024-10-19

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
			name: "OnUpdate doesn't add unassigned pods",
			nodePodsMap: map[string][]NamespacedPod{
				testNode1: {{"default", "Pod-1"}},
			},
			podsToUpdate: []*UpdateItem{
				{old: st.MakePod().Name("Pod-4").Obj(), new: st.MakePod().Name("Pod-4").Obj()},
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
			name: "OnUpdate adds pods if assigned to node",
			nodePodsMap: map[string][]NamespacedPod{
				testNode1: {{"default", "Pod-1"}, {"default", "Pod-2"}},
				testNode2: {{"default", "Pod-3"}},
			},
			podsToUpdate: []*UpdateItem{
				{old: st.MakePod().Name("Pod-4").Node("").Obj(), new: st.MakePod().Name("Pod-4").Node(testNode2).Obj()},
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
			name: "OnUpdate updates the cache if Pod is moved to another node",
			nodePodsMap: map[string][]NamespacedPod{
				testNode1: {{"default", "Pod-1"}},
			},
			podsToUpdate: []*UpdateItem{
				{old: st.MakePod().Name("Pod-1").Namespace("default").Node(testNode1).Obj(), new: st.MakePod().Name("Pod-1").Namespace("default").Node(testNode2).Obj()},
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
			// 初始化测试环境
			ctx := context.Background()
			cs := fakeclientset.NewSimpleClientset()
			pawsInformerFactory := pawsinformers.NewSharedInformerFactory(cs, 0)
			utInformer := pawsInformerFactory.Scheduling().V1alpha1().UsageTemplates()
			pawsInformerFactory.Start(ctx.Done())

			fakeClient := clientsetfake.NewSimpleClientset()
			informerFactory := informers.NewSharedInformerFactory(fakeClient, 0)
			podInformer := informerFactory.Core().V1().Pods()
			informerFactory.Start(ctx.Done())

			snapshot := testutil.NewFakeSharedLister(nil, nil)
			mgr := NewUsageTemplateManager(cs, snapshot, utInformer, podInformer)
			mgr.NodePodsCache = tt.nodePodsMap

			// 执行 Pod 更新操作
			for _, item := range tt.podsToUpdate {
				mgr.OnUpdate(item.old, item.new)
			}

			// 验证缓存总大小
			assert.Equal(t, tt.expectedTotalCacheSize, mgr.CacheSize())

			// 验证每个节点的缓存大小
			for node, expectedSize := range tt.expectedPerNodeCacheSize {
				assert.Equal(t, expectedSize, len(mgr.NodePodsCache[node]))
			}

			// 验证缓存中 Pod 的存在性
			for node, expectedPods := range tt.expectedNodeCachePods {
				podsInCache := extractPodNames(mgr.NodePodsCache[node])
				assert.ElementsMatch(t, expectedPods, podsInCache)
			}
		})
	}
}

// 提取缓存中 Pod 名称的辅助函数
func extractPodNames(pods []NamespacedPod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}
