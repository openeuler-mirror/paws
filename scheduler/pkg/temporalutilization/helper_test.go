package temporalutilization

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
				assert.Nil(t, err)
			}

			// we offset by hour, so let's shift the expected usages
			if !tt.usageTemplates[0].Status.IsLongRunning {
				shiftedUsageTemplate := shiftUsageTemplate(tt.expectedUsages, time.Now().UTC().Hour())

				tt.expectedUsages = shiftedUsageTemplate
			}

			assert.Equal(t, tt.expectedUsages, podUsages)
		})
	}
}

func shiftUsageTemplate(old map[string]*UsageTemplate, currentHour int) map[string]*UsageTemplate {
	new := map[string]*UsageTemplate{}
	for resource, template := range old {
		new[resource] = &UsageTemplate{
			resource:    template.resource,
			unit:        template.unit,
			weekDayHour: make(map[int16]float32),
			weekendHour: make(map[int16]float32),
		}

		for k, v := range template.weekDayHour {
			offsetHour := math.Mod(float64(currentHour)+float64(k), NumHoursInADay)

			new[resource].weekDayHour[int16(offsetHour)] = float32(v)
		}

		for k, v := range template.weekendHour {
			offsetHour := math.Mod(float64(currentHour)+float64(k), NumHoursInADay)

			new[resource].weekendHour[int16(offsetHour)] = float32(v)
		}

	}
	return new
}
