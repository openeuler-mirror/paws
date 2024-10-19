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
// Date: 2024-10-18

import (
	"math"
	"testing"
	"time"

	"gitee.com/openeuler/paws/scheduler/apis/scheduling/v1alpha1"
	testutils "gitee.com/openeuler/paws/scheduler/pkg/test/util"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	st "k8s.io/kubernetes/pkg/scheduler/testing"
)

func TestUsageTemplateLogic(t *testing.T) {
	tests := []struct {
		name           string
		pod            *v1.Pod
		usageTemplates []*v1alpha1.UsageTemplate
		expectedUsages map[string]*UsageTemplate
		expectedErr    bool
	}{
		{
			name: "Expected Usages should match CRD when not long running",
			pod: st.MakePod().Namespace("default").Name("pod-1").Labels(map[string]string{
				v1alpha1.UsageTemplateLabelIdentifier: "test-crd-1",
			}).Containers([]v1.Container{
				st.MakeContainer().Resources(map[v1.ResourceName]string{
					v1.ResourceCPU: "900m",
				}).Obj(),
			}).Obj(),
			expectedUsages: map[string]*UsageTemplate{
				"cpu": {
					resource: "cpu",
					weekDayHour: map[int16]float32{
						0:  100,
						1:  100,
						2:  100,
						3:  100,
						4:  100,
						5:  100,
						6:  700,
						7:  700,
						8:  700,
						9:  700,
						10: 700,
						11: 700,
						12: 200,
						13: 200,
						14: 200,
						15: 200,
						16: 200,
						17: 200,
					},
					weekendHour: map[int16]float32{
						0:  100,
						1:  100,
						2:  100,
						3:  100,
						4:  100,
						5:  100,
						6:  700,
						7:  700,
						8:  700,
						9:  700,
						10: 700,
						11: 700,
						12: 200,
						13: 200,
						14: 200,
						15: 200,
						16: 200,
						17: 200,
					},
				},
			},
			expectedErr: false,
			usageTemplates: []*v1alpha1.UsageTemplate{
				testutils.MakeUsageTemplate("test-crd-1", "default", true, "BestEffort",
					map[string]map[int]float32{
						"cpu": testutils.MakeUsageAcrossPeriods([][]float32{
							{0.0, 6.0, 100},
							{6.0, 12.0, 700},
							{12.0, 18.0, 200},
						}),
					}, map[string]map[int]float32{}, false),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := newTestUsageEvaluationManager(nil, tt.pod, nil, tt.usageTemplates)
			podUsages, err := getUsageTemplatesByPod(mgr, tt.pod.Namespace, tt.pod.Name, []string{"cpu"})

			if tt.expectedErr {
				assert.NotNil(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Shift expected usages only if not long running
			if !tt.usageTemplates[0].Status.IsLongRunning {
				shiftedUsageTemplate := shiftUsageTemplate(tt.expectedUsages, time.Now().UTC().Hour())
				tt.expectedUsages = shiftedUsageTemplate
			}

			assert.Equal(t, tt.expectedUsages, podUsages)
		})
	}
}

func shiftUsageTemplate(old map[string]*UsageTemplate, currentHour int) map[string]*UsageTemplate {
	new := make(map[string]*UsageTemplate)
	for resource, template := range old {
		newTemplate := &UsageTemplate{
			resource:    template.resource,
			unit:        template.unit,
			weekDayHour: make(map[int16]float32, NumHoursInADay),
			weekendHour: make(map[int16]float32, NumHoursInADay),
		}

		for k, v := range template.weekDayHour {
			offsetHour := int16((currentHour + int(k)) % NumHoursInADay)
			newTemplate.weekDayHour[offsetHour] = v
		}

		for k, v := range template.weekendHour {
			offsetHour := int16((currentHour + int(k)) % NumHoursInADay)
			newTemplate.weekendHour[offsetHour] = v
		}

		new[resource] = newTemplate
	}
	return new
}
