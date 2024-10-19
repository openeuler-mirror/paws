package overcommit

import (
	"testing"

	"gitee.com/openeuler/paws/scheduler/apis/scheduling/v1alpha1"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	st "k8s.io/kubernetes/pkg/scheduler/testing"
)

func TestOvercommitRatioCalculation(t *testing.T) {
	tests := []struct {
		name            string
		pods            []*v1.Pod
		node            *v1.Node
		overcommitRatio string
		Expected        v1.ResourceList
		expectError     bool
	}{
		{
			name: "CPU overcommit ratio 0.3, obtain 300m overcommitable resource",
			node: st.MakeNode().Capacity(map[v1.ResourceName]string{
				v1.ResourceCPU: "1000m",
			}).Obj(),
			overcommitRatio: "0.3",
			Expected: v1.ResourceList{
				v1.ResourceCPU: resource.MustParse("300m"),
			},
			expectError: false,
		},
		{
			name: "Memory overcommit ratio 0.5, obtain 512Mi overcommitable resource",
			node: st.MakeNode().Capacity(map[v1.ResourceName]string{
				v1.ResourceMemory: "1024Mi",
			}).Obj(),
			overcommitRatio: "0.5",
			Expected: v1.ResourceList{
				v1.ResourceMemory: resource.MustParse("512Mi"),
			},
			expectError: false,
		},
		{
			name: "Invalid overcommit ratio (negative)",
			node: st.MakeNode().Capacity(map[v1.ResourceName]string{
				v1.ResourceCPU: "1000m",
			}).Obj(),
			overcommitRatio: "-0.5",
			Expected:        v1.ResourceList{},
			expectError:     true,
		},
		{
			name: "Missing overcommit annotation",
			node: st.MakeNode().Capacity(map[v1.ResourceName]string{
				v1.ResourceCPU: "1000m",
			}).Obj(),
			overcommitRatio: "",
			Expected:        v1.ResourceList{},
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeInfo := framework.NewNodeInfo(tt.pods...)
			tt.node.Annotations = make(map[string]string)
			if tt.overcommitRatio != "" {
				tt.node.Annotations[v1alpha1.NodeCPUOvercommitRatioAnnotation] = tt.overcommitRatio
			}
			nodeInfo.SetNode(tt.node)

			got, err := CalculateOvercommitResources(nodeInfo, v1alpha1.SupportedOvercommitResourceAnnotation)

			if tt.expectError {
				assert.Error(t, err, "Expected an error but got none")
				return
			}

			assert.NoError(t, err, "Unexpected error during calculation")

			for r, v := range got {
				expectedv := tt.Expected[r]
				assert.Equal(t, expectedv.MilliValue(), v.MilliValue(), "Resource mismatch for %v", r)
			}
		})
	}
}

