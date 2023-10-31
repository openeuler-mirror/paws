package temporalutilization

import (
	"context"
	"testing"

	pluginConfig "gitee.com/openeuler/paws/scheduler/apis/config"
	"gitee.com/openeuler/paws/scheduler/apis/config/v1beta3"
	"gitee.com/openeuler/paws/scheduler/apis/scheduling/v1alpha1"
	fakeclientset "gitee.com/openeuler/paws/scheduler/pkg/generated/clientset/versioned/fake"
	pawsinformers "gitee.com/openeuler/paws/scheduler/pkg/generated/informers/externalversions"
	testutils "gitee.com/openeuler/paws/scheduler/pkg/test/util"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	testClientSet "k8s.io/client-go/kubernetes/fake"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	st "k8s.io/kubernetes/pkg/scheduler/testing"
)

type testSharedLister struct {
	nodes       []*v1.Node
	nodeInfos   []*framework.NodeInfo
	nodeInfoMap map[string]*framework.NodeInfo
}

func (tsl *testSharedLister) NodeInfos() framework.NodeInfoLister {
	return tsl
}

func (tsl *testSharedLister) StorageInfos() framework.StorageInfoLister {
	return nil
}

func (tsl *testSharedLister) Get(nodeName string) (*framework.NodeInfo, error) {
	return tsl.nodeInfoMap[nodeName], nil
}

func (tsl *testSharedLister) HavePodsWithAffinityList() ([]*framework.NodeInfo, error) {
	return nil, nil
}

func (tsl *testSharedLister) HavePodsWithRequiredAntiAffinityList() ([]*framework.NodeInfo, error) {
	return nil, nil
}

func (tsl *testSharedLister) List() ([]*framework.NodeInfo, error) {
	return tsl.nodeInfos, nil
}

// NOTE: This is preliminary as we are still investigating which score is better
func TestTemporalUtilizationWithTrimaranScoring(t *testing.T) {
	args := pluginConfig.TemporalUtilizationArgs{
		HotSpotThreshold: int32(v1beta3.DefaultHotSpotThreshold),
		HardThreshold:    v1beta3.DefaultHardThresholdValue,
	}

	nodeResources := map[v1.ResourceName]string{
		v1.ResourceCPU:    "1000m",
		v1.ResourceMemory: "1Gi",
	}

	tests := []struct {
		name           string
		pod            *v1.Pod
		usageTemplates []*v1alpha1.UsageTemplate
		scheduledPods  []*v1.Pod
		nodes          []*v1.Node
		expected       framework.NodeScoreList
	}{
		{
			name:           "new node, Trimaran Score, No CRD, using 10 percent (best effort pod) of capacity with target at 60",
			pod:            st.MakePod().Name("Pod-1").Namespace("default").Obj(),
			usageTemplates: []*v1alpha1.UsageTemplate{},
			scheduledPods:  []*v1.Pod{},
			nodes: []*v1.Node{
				st.MakeNode().Name("Node-1").Capacity(nodeResources).Obj(),
			},
			expected: []framework.NodeScore{
				// defaultCPUMilli 100
				// NodeCap 1000
				// 100/1000 = trimaranScore(10%) = 67
				// over entire weekdays (1608) and weekend (1608)
				{Name: "Node-1", Score: 3216},
			},
		},
		{
			name: "new node, Trimaran Score, Has enabled CRD, using 10 percent of capacity with target threshold at 60.",
			pod: st.MakePod().Name("Pod-1").Namespace("default").Labels(map[string]string{
				v1alpha1.UsageTemplateLabelIdentifier: "test-crd",
			}).Obj(),
			usageTemplates: []*v1alpha1.UsageTemplate{
				testutils.MakeUsageTemplate("test-crd", "default", true, "BestEffort",
					map[string]map[int]float32{
						"cpu": testutils.SameUsageADay(100),
					}, map[string]map[int]float32{
						"cpu": testutils.SameUsageADay(100),
					}, true),
			},
			scheduledPods: []*v1.Pod{},
			nodes: []*v1.Node{
				st.MakeNode().Name("Node-1").Capacity(nodeResources).Obj(),
			},
			expected: []framework.NodeScore{
				// defaultCPUMilli 100
				// NodeCap 1000
				// 100/1000 = trimaranScore(10%) over entire weekdays and weekend
				{Name: "Node-1", Score: 3216},
			},
		},
		{
			name: "new node, Trimaran Score, Has disabled CRD, falls back to default resource request, using 10 percent of capacity with target threshold at 60.",
			pod: st.MakePod().Name("Pod-1").Namespace("default").Labels(map[string]string{
				v1alpha1.UsageTemplateLabelIdentifier: "test-crd",
			}).Obj(),
			usageTemplates: []*v1alpha1.UsageTemplate{
				testutils.MakeUsageTemplate("test-crd", "default", false, "BestEffort",
					map[string]map[int]float32{
						"cpu": testutils.SameUsageADay(1000),
					}, map[string]map[int]float32{
						"cpu": testutils.SameUsageADay(1000),
					}, true),
			},
			scheduledPods: []*v1.Pod{},
			nodes: []*v1.Node{
				st.MakeNode().Name("Node-1").Capacity(nodeResources).Obj(),
			},
			expected: []framework.NodeScore{
				// defaultCPUMilli 100
				// NodeCap 1000
				// 100/1000 = trimaranScore(10%) over entire weekdays and weekend
				{Name: "Node-1", Score: 3216},
			},
		},
		{
			name: "hot node, Trimaran Score, using 70 percent of capacity with target at 60",
			pod: st.MakePod().Name("Pod-1").Namespace("default").Labels(map[string]string{
				v1alpha1.UsageTemplateLabelIdentifier: "test-crd",
			}).Obj(),
			nodes: []*v1.Node{
				st.MakeNode().Name("Node-1").Capacity(nodeResources).Obj(),
			},
			usageTemplates: []*v1alpha1.UsageTemplate{
				testutils.MakeUsageTemplate("test-crd", "default", true, "BestEffort",
					map[string]map[int]float32{
						"cpu": testutils.SameUsageADay(700),
					}, map[string]map[int]float32{
						"cpu": testutils.SameUsageADay(700),
					}, true),
			},
			scheduledPods: []*v1.Pod{},
			expected: []framework.NodeScore{
				// defaultCPUMilli 700
				// NodeCap 1000
				// 700/1000 = trimaranScore(70%) over entire weekdays and weekends
				{Name: "Node-1", Score: 2160},
			},
		},
		{
			name: "Packed multiple pods on to node, Trimaran Score, using 80 percent of capacity with target at 60",
			pod: st.MakePod().Name("Pod-2").Namespace("default").Labels(map[string]string{
				v1alpha1.UsageTemplateLabelIdentifier: "test-crd-2",
			}).Obj(),
			scheduledPods: []*v1.Pod{
				st.MakePod().Name("Pod-1").Namespace("default").Labels(map[string]string{
					v1alpha1.UsageTemplateLabelIdentifier: "test-crd-1",
				}).Node("Node-1").Obj(),
			},
			usageTemplates: []*v1alpha1.UsageTemplate{
				testutils.MakeUsageTemplate("test-crd-1", "default", true, "BestEffort",
					map[string]map[int]float32{
						"cpu": testutils.SameUsageADay(100),
					}, map[string]map[int]float32{
						"cpu": testutils.SameUsageADay(100),
					}, true),
				testutils.MakeUsageTemplate("test-crd-2", "default", true, "BestEffort",
					map[string]map[int]float32{
						"cpu": testutils.SameUsageADay(700),
					}, map[string]map[int]float32{
						"cpu": testutils.SameUsageADay(700),
					}, true),
			},
			nodes: []*v1.Node{
				st.MakeNode().Name("Node-1").Capacity(nodeResources).Obj(),
			},
			expected: []framework.NodeScore{
				// Trimaran(80%) => 30*24*2
				{Name: "Node-1", Score: 1440},
			},
		},
		{
			name: "Packed multiple nodes, with different peaks and valleys trimaran score, using 80 percent of capacity with target at 60 in total",
			pod: st.MakePod().Name("Pod-1").Namespace("default").Labels(map[string]string{
				v1alpha1.UsageTemplateLabelIdentifier: "test-crd-1",
			}).Obj(),
			nodes: []*v1.Node{
				st.MakeNode().Name("Node-1").Capacity(nodeResources).Obj(),
			},
			scheduledPods: []*v1.Pod{
				st.MakePod().Name("Pod-2").Namespace("default").Labels(map[string]string{
					v1alpha1.UsageTemplateLabelIdentifier: "test-crd-2",
				}).Node("Node-1").Obj(),
			},
			usageTemplates: []*v1alpha1.UsageTemplate{
				testutils.MakeUsageTemplate("test-crd-1", "default", true, "BestEffort",
					map[string]map[int]float32{
						"cpu": testutils.MakeUsageAcrossPeriods([][]float32{
							{0.0, 6.0, 100},
							{6.0, 12.0, 700},
							{12.0, 18.0, 100},
							{18.0, 24.0, 700},
						}),
					}, map[string]map[int]float32{
						"cpu": testutils.MakeUsageAcrossPeriods([][]float32{
							{0.0, 6.0, 100},
							{6.0, 12.0, 700},
							{12.0, 18.0, 100},
							{18.0, 24.0, 700},
						}),
					}, true),
				testutils.MakeUsageTemplate("test-crd-2", "default", true, "BestEffort",
					map[string]map[int]float32{
						"cpu": testutils.MakeUsageAcrossPeriods([][]float32{
							{0.0, 6.0, 700},
							{6.0, 12.0, 100},
							{12.0, 18.0, 700},
							{18.0, 24.0, 100},
						}),
					}, map[string]map[int]float32{
						"cpu": testutils.MakeUsageAcrossPeriods([][]float32{
							{0.0, 6.0, 700},
							{6.0, 12.0, 100},
							{12.0, 18.0, 700},
							{18.0, 24.0, 100},
						}),
					}, true),
			},
			expected: []framework.NodeScore{
				// 80% weekdays and weekends
				{Name: "Node-1", Score: 1440},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes := append([]*v1.Node{}, tt.nodes...)
			state := framework.NewCycleState()
			mgr := newTestUsageEvaluationManager(nodes, tt.pod, tt.scheduledPods, tt.usageTemplates)

			pl := &TemporalUtilization{
				HotSpotThreshold: args.HotSpotThreshold,
				HardThreshold:    args.HardThreshold,
				utMgr:            mgr,
			}

			for _, sp := range tt.scheduledPods {
				pl.utMgr.OnAdd(sp)
			}

			var actualList framework.NodeScoreList
			for _, n := range tt.nodes {
				nodeName := n.Name
				score, status := pl.Score(context.Background(), state, tt.pod, nodeName)
				assert.True(t, status.IsSuccess())
				actualList = append(actualList, framework.NodeScore{Name: nodeName, Score: score})
			}
			assert.ElementsMatch(t, tt.expected, actualList)
		})
	}
}

func TestTemporalUtilizationWithOvercommitFiltering(t *testing.T) {
	args := pluginConfig.TemporalUtilizationArgs{
		HotSpotThreshold: int32(v1beta3.DefaultHotSpotThreshold),
		HardThreshold:    v1beta3.DefaultHardThresholdValue,
		EnableOvercommit: v1beta3.DefaultEnableOvercommitValue,
	}
	tests := []struct {
		name            string
		pod             *v1.Pod
		scheduledPods   []*v1.Pod
		node            *v1.Node
		overcommitRatio map[string]string
		expected        *framework.Status
	}{
		{
			name: "Filter allow overcommit on empty node",
			pod: st.MakePod().Containers([]v1.Container{
				st.MakeContainer().Resources(map[v1.ResourceName]string{
					v1.ResourceCPU: "1100m",
				}).Obj(),
			}).Obj(),
			node: st.MakeNode().Capacity(map[v1.ResourceName]string{
				v1.ResourceCPU: "1000m",
			}).Obj(),
			overcommitRatio: map[string]string{
				v1alpha1.NodeCPUOvercommitRatioAnnotation: "0.3",
			},
			expected: nil,
		},
		{
			name: "Filter node does not overcommit, pod exceed node",
			pod: st.MakePod().Containers([]v1.Container{
				st.MakeContainer().Resources(map[v1.ResourceName]string{
					v1.ResourceCPU: "1100m",
				}).Obj(),
			}).Obj(),
			node: st.MakeNode().Capacity(map[v1.ResourceName]string{
				v1.ResourceCPU: "1000m",
			}).Obj(),
			expected: framework.NewStatus(framework.Unschedulable, "Insufficient cpu"),
		},
		{
			name: "Filter node overcommit, has schedule pods, pod still fits",
			pod: st.MakePod().Containers([]v1.Container{
				st.MakeContainer().Resources(map[v1.ResourceName]string{
					v1.ResourceCPU: "100m",
				}).Obj(),
			}).Obj(),
			node: st.MakeNode().Name("Node-1").Capacity(map[v1.ResourceName]string{
				v1.ResourceCPU: "1000m",
			}).Obj(),
			scheduledPods: []*v1.Pod{
				st.MakePod().Name("Pod-1").Containers([]v1.Container{
					st.MakeContainer().Resources(map[v1.ResourceName]string{
						v1.ResourceCPU: "100m",
					}).Obj(),
				}).Node("Node-1").Obj(),
				st.MakePod().Name("Pod-2").Containers([]v1.Container{
					st.MakeContainer().Resources(map[v1.ResourceName]string{
						v1.ResourceCPU: "100m",
					}).Obj(),
				}).Node("Node-1").Obj(),
			},
			overcommitRatio: map[string]string{
				v1alpha1.NodeCPUOvercommitRatioAnnotation: "0.3",
			},
			expected: nil,
		},
		{
			name: "Filter node overcommit, has schedule pods, pod does not fits",
			pod: st.MakePod().Containers([]v1.Container{
				st.MakeContainer().Resources(map[v1.ResourceName]string{
					v1.ResourceCPU: "1000m",
				}).Obj(),
			}).Obj(),
			node: st.MakeNode().Name("Node-1").Capacity(map[v1.ResourceName]string{
				v1.ResourceCPU: "1000m",
			}).Obj(),
			scheduledPods: []*v1.Pod{
				st.MakePod().Name("Pod-1").Containers([]v1.Container{
					st.MakeContainer().Resources(map[v1.ResourceName]string{
						v1.ResourceCPU: "200m",
					}).Obj(),
				}).Node("Node-1").Obj(),
				st.MakePod().Name("Pod-2").Containers([]v1.Container{
					st.MakeContainer().Resources(map[v1.ResourceName]string{
						v1.ResourceCPU: "200m",
					}).Obj(),
				}).Node("Node-1").Obj(),
			},
			overcommitRatio: map[string]string{
				v1alpha1.NodeCPUOvercommitRatioAnnotation: "0.3",
			},
			expected: framework.NewStatus(framework.Unschedulable, "Insufficient cpu"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := framework.NewCycleState()
			mgr := newTestUsageEvaluationManager([]*v1.Node{tt.node}, tt.pod, tt.scheduledPods, nil)

			fit, err := NewFitPlugin(nil)
			assert.NoError(t, err)

			pl := &TemporalUtilization{
				HotSpotThreshold: args.HotSpotThreshold,
				HardThreshold:    args.HardThreshold,
				EnableOvercommit: true,
				utMgr:            mgr,
				FitPlugin:        fit,
			}

			for _, sp := range tt.scheduledPods {
				pl.utMgr.OnAdd(sp)
			}
			ctx := context.Background()
			result, status := pl.PreFilter(ctx, state, tt.pod)

			assert.Nil(t, result)
			assert.Nil(t, status)
			nodeInfo := framework.NewNodeInfo(tt.scheduledPods...)

			tt.node.Annotations = make(map[string]string)

			for k, v := range tt.overcommitRatio {
				tt.node.Annotations[k] = v
			}

			nodeInfo.SetNode(tt.node)

			status = pl.Filter(ctx, state, tt.pod, nodeInfo)

			if tt.expected == nil {
				assert.Nil(t, status)
			} else {
				assert.NotNil(t, status)
				assert.Equal(t, tt.expected.Code(), status.Code())
				assert.ElementsMatch(t, tt.expected.Reasons(), status.Reasons())
			}
		})
	}
}

func TestTemporalUtilizationWithTemporalFiltering(t *testing.T) {
	args := pluginConfig.TemporalUtilizationArgs{
		HotSpotThreshold:       int32(v1beta3.DefaultHotSpotThreshold),
		HardThreshold:          v1beta3.DefaultHardThresholdValue,
		EnableOvercommit:       v1beta3.DefaultEnableOvercommitValue,
		FilterByTemporalUsages: true,
	}

	tests := []struct {
		name            string
		pod             *v1.Pod
		scheduledPods   []*v1.Pod
		node            *v1.Node
		usageTemplates  []*v1alpha1.UsageTemplate
		overcommitRatio map[string]string
		expected        *framework.Status
	}{
		{
			name: "Filter use resource requests if usage template is empty, ok",
			pod: st.MakePod().Namespace("default").Name("pod-1").Labels(map[string]string{
				v1alpha1.UsageTemplateLabelIdentifier: "test-crd-1",
			}).Containers([]v1.Container{
				st.MakeContainer().Resources(map[v1.ResourceName]string{
					v1.ResourceCPU: "900m",
				}).Obj(),
			}).Obj(),
			node: st.MakeNode().Capacity(map[v1.ResourceName]string{
				v1.ResourceCPU: "1000m",
			}).Obj(),
			expected:       nil,
			usageTemplates: make([]*v1alpha1.UsageTemplate, 0),
		},
		{
			name: "Filter use resource requests if usage template is empty, Unschedulable",
			pod: st.MakePod().Namespace("default").Name("pod-1").Labels(map[string]string{
				v1alpha1.UsageTemplateLabelIdentifier: "test-crd-1",
			}).Containers([]v1.Container{
				st.MakeContainer().Resources(map[v1.ResourceName]string{
					v1.ResourceCPU: "1100m",
				}).Obj(),
			}).Obj(),
			node: st.MakeNode().Capacity(map[v1.ResourceName]string{
				v1.ResourceCPU: "1000m",
			}).Obj(),
			expected:       framework.NewStatus(framework.Unschedulable, "Insufficient cpu"),
			usageTemplates: make([]*v1alpha1.UsageTemplate, 0),
		},
		{
			name: "Filter ignore resource requests if usage template is present",
			pod: st.MakePod().Namespace("default").Name("pod-1").Labels(map[string]string{
				v1alpha1.UsageTemplateLabelIdentifier: "test-crd-1",
			}).Containers([]v1.Container{
				st.MakeContainer().Resources(map[v1.ResourceName]string{
					v1.ResourceCPU: "1100m",
				}).Obj(),
			}).Obj(),
			node: st.MakeNode().Capacity(map[v1.ResourceName]string{
				v1.ResourceCPU: "1000m",
			}).Obj(),
			expected: nil,
			usageTemplates: []*v1alpha1.UsageTemplate{
				testutils.MakeUsageTemplate("test-crd-1", "default", true, "BestEffort",
					map[string]map[int]float32{
						"cpu": testutils.SameUsageADay(100),
					}, map[string]map[int]float32{
						"cpu": testutils.SameUsageADay(100),
					}, true),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := framework.NewCycleState()
			mgr := newTestUsageEvaluationManager([]*v1.Node{tt.node}, tt.pod, tt.scheduledPods, tt.usageTemplates)

			fit, err := NewFitPlugin(nil)
			assert.NoError(t, err)

			pl := &TemporalUtilization{
				HotSpotThreshold:       args.HotSpotThreshold,
				HardThreshold:          args.HardThreshold,
				EnableOvercommit:       args.EnableOvercommit,
				FilterByTemporalUsages: args.FilterByTemporalUsages,
				utMgr:                  mgr,
				FitPlugin:              fit,
			}

			for _, sp := range tt.scheduledPods {
				pl.utMgr.OnAdd(sp)
			}
			ctx := context.Background()
			result, status := pl.PreFilter(ctx, state, tt.pod)

			assert.Nil(t, result)
			assert.Nil(t, status)
			nodeInfo := framework.NewNodeInfo(tt.scheduledPods...)

			tt.node.Annotations = make(map[string]string)

			for k, v := range tt.overcommitRatio {
				tt.node.Annotations[k] = v
			}

			nodeInfo.SetNode(tt.node)

			status = pl.Filter(ctx, state, tt.pod, nodeInfo)

			if tt.expected == nil {
				assert.Nil(t, status)
			} else {
				assert.NotNil(t, status)
				assert.Equal(t, tt.expected.Code(), status.Code())
				// assert.ElementsMatch(t, tt.expected.Reasons(), status.Reasons())
			}
		})
	}
}

func newTestUsageEvaluationManager(tnodes []*v1.Node, pod *v1.Pod, schedulePods []*v1.Pod, uts []*v1alpha1.UsageTemplate) *UsageTemplateManager {
	nodes := append([]*v1.Node{}, tnodes...)
	snapshot := newTestSharedLister(nil, nodes)
	ctx := context.Background()
	tCS := testClientSet.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(tCS, 0)
	podInformer := informerFactory.Core().V1().Pods()
	informerFactory.Start(ctx.Done())
	// add incoming pod to the informer
	podInformer.Informer().GetStore().Add(pod)
	// add existing pods to the informer
	for _, schedulePod := range schedulePods {
		if schedulePod != nil {
			podInformer.Informer().GetStore().Add(schedulePod)
		}
	}

	pawsCS := fakeclientset.NewSimpleClientset()
	pawsInformerFactory := pawsinformers.NewSharedInformerFactory(pawsCS, 0)
	utInformer := pawsInformerFactory.Scheduling().V1alpha1().UsageTemplates()
	pawsInformerFactory.Start(ctx.Done())
	for _, ut := range uts {
		if ut != nil {
			utInformer.Informer().GetStore().Add(ut)
		}
	}

	mgr := NewUsageTemplateManager(pawsCS, snapshot, utInformer, podInformer)
	return mgr
}

func newTestSharedLister(pods []*v1.Pod, nodes []*v1.Node) *testSharedLister {
	nodeInfoMap := make(map[string]*framework.NodeInfo)
	nodeInfos := make([]*framework.NodeInfo, 0)
	for _, pod := range pods {
		nodeName := pod.Spec.NodeName
		if _, ok := nodeInfoMap[nodeName]; !ok {
			nodeInfoMap[nodeName] = framework.NewNodeInfo()
		}
		nodeInfoMap[nodeName].AddPod(pod)
	}

	for _, node := range nodes {
		if _, ok := nodeInfoMap[node.Name]; !ok {
			nodeInfoMap[node.Name] = framework.NewNodeInfo()
		}
		nodeInfoMap[node.Name].SetNode(node)
	}

	for _, v := range nodeInfoMap {
		nodeInfos = append(nodeInfos, v)
	}

	return &testSharedLister{
		nodes:       nodes,
		nodeInfos:   nodeInfos,
		nodeInfoMap: nodeInfoMap,
	}
}
