package v1alpha1

import (
	"gitee.com/openeuler/paws/scheduler/apis/scheduling"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	UsageTemplateLabelIdentifier    = scheduling.GroupName + "/usage-template"
	ReadyForEvaluationSuccessReason = "UsageTemplateReady"
	DisabledSuccessReason           = "UsageTemplateDisabled"

	// NodeCPUOvercommitRatioAnnotation allows a node to be specified whether it has overcommit set
	// ratio should be starting from 0, i.e. 0.3 = 30% more resources are considered as allocatable
	// it is an annotation because it is not for filtering
	NodeCPUOvercommitRatioAnnotation = scheduling.GroupName + "/cpu-overcommit-ratio"

	// TODO: To evaluate how often
	DefaultEvaluationPeriodHours = 6
	DefaultEvaluationWindowDays  = 14
)

var (
	SupportedResourcesMetricLabel = map[string]string{
		v1.ResourceCPU.String(): "container_cpu_usage_seconds_total",
	}

	// For the supported resources, if they are a counter
	// for each datapoint, what is the time window to look at
	SupportedResourcesRateTimeWindow = map[string]string{
		v1.ResourceCPU.String(): "2m",
	}

	SupportedResourcesRangeMethod = map[string]string{
		v1.ResourceCPU.String(): "rate",
	}

	SupportedMetricLabelFilters = map[string][]string{
		// https://stackoverflow.com/questions/69281327/why-container-memory-usage-is-doubled-in-cadvisor-metrics/69282328#69282328
		// To ignore empty cgroup hierarchy
		"container_cpu_usage_seconds_total": {"container!=\"\""},
	}

	SupportedResourceMetricUnit = map[string]string{
		v1.ResourceCPU.String(): "millicore",
	}

	SupportedResourceMetricScalingFactor = map[string]float64{
		// container_cpu_usage_seconds_total returns core seconds.
		// i.e. 1 = 1000 millicore
		v1.ResourceCPU.String(): 1000.0,
	}

	SupportedOvercommitResourceAnnotation = map[string]string{
		v1.ResourceCPU.String(): NodeCPUOvercommitRatioAnnotation,
	}
)

func GetSupportedResources() []string {
	results := []string{}
	for k := range SupportedResourcesMetricLabel {
		results = append(results, k)
	}
	return results
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName={ut,uts}
// +kubebuilder:subresource:status
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// UsageTemplate is the configuration for requesting a evaluation of a usage template based on real time resource usages
type UsageTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Spec              UsageTemplateSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status            UsageTemplateStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// UsageTemplateSpec is the specification for UT
type UsageTemplateSpec struct {
	// Enabled allow scheduler to interpret whether to use the evaluated values for scheduling
	Enabled bool `json:"enabled,omitempty" protobuf:"bytes,1,name=enabled"`
	// EvaluatePeriodHours specify the desire evaluation period minutes for this specific UT, default to 6 hours
	EvaluatePeriodHours *int32 `json:"evaluatePeriodHours,omitempty" protobuf:"bytes,2,name=evaluatePeriodHours"`
	// EvaluationWindow specify the desire time window in days for this specific UT, default to 14 days
	EvaluationWindowDays *int16 `json:"evaluationWindowDays,omitempty" protobuf:"bytes,3,name=evaluationWindowDays"`
	// Resources specify the desire resource to evaluate for, currently supports CPU
	Resources []string `json:"resources,omitempty" protobuf:"bytes,3,rep,name=resources"`
	// Filters to specify how to look for an application pods, i.e. "k=v,k!=v,k~=v"
	// we are not using the k8s labelSelector because a few labelExpression are not supported in prometheus
	Filters []string `json:"filters" protobuf:"bytes,5,name=filters"`
	// JoinLabels to specify when the metric require joining its own timeseries to acquire more labels,
	// an example of this is when using cAdvisor with white_listed_labels, the whitelistlabels only appear on the top level container 'pause'
	//  The following query shows such usecase:
	// rate(container_cpu_usage_seconds_total{container!=""}[2m])
	// * on (namespace, pod) group_left (part_of, managed_by)
	// container_cpu_usage_seconds_total{namespace="monitoring",part_of!=""}
	JoinLabels []string `json:"joinLabels,omitempty" protobuf:"bytes,6,name=joinLabels"`
	// JoinFilters to specify when the joining the metric, the right handside operation
	// should also contain filters
	JoinFilters []string `json:"joinFilters,omitempty" protobuf:"bytes,7,name=joinFilters"`
	// PriorityClass specify whether the priority of the application
	// follow the kubernetes convention. i.e. Guaranteed, Burstable, BestEffort
	QualityOfServiceClass string `json:"qualityOfServiceClass,omitempty" protobuf:"bytes,8,name=qualityOfServiceClass"`
}

// Sample contains the actual usage for the particular hour
type Sample struct {
	Hour int32 `json:"hour" protobuf:"bytes,1,name=hour"`
	// the actual value represented as a string
	Value string `json:"value" protobuf:"bytes,2,name=value"`
	// which percentile was calculated from
	Percentile string `json:"percentile" protobuf:"bytes,3,name=percentile"`
	// what unit, e.g. millicore
	Unit string `json:"unit" protobuf:"bytes,4,name=unit"`
	// whether this is a weekday value
	IsWeekday bool `json:"isWeekday,omitempty" protobuf:"bytes,5,opt,name=isWeekday"`
}

// ResourceUsage is the historical usage of a resource
type ResourceUsage struct {
	// Name of the resource
	Resource string `json:"name" protobuf:"bytes,1,name=resource"`
	// Usages contains the samples for the resource
	Usages []Sample `json:"usages" protobuf:"bytes,2,rep,name=usages"`
}

// ResourceUsages is the evaluated historical usage per resource
// It contains a set of samples for each resource
type ResourceUsages struct {
	// Items contains historical usage per resource
	// currently only support CPU
	// +optional
	Items []ResourceUsage `json:"items,omitempty" protobuf:"bytes,1,rep,name=items"`
}

// UsageTemplateStatus describes the runtime state of the UT
type UsageTemplateStatus struct {
	// HistoricalUsage is the most recent evaluation conducted by the evaluator for the controlled pods
	// +optional
	HistoricalUsage *ResourceUsages `json:"sample,omitempty" protobuf:"bytes,1,name=historicalUsage"`
	// Conditions is the set of conditions required for this UT and indicates whether or not those conditions are met.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions Conditions `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,2,rep,name=conditions"`
	// IsLongRunning indicates whether this application is long running, defined as longer than 24 hours
	// +optional
	IsLongRunning bool `json:"isLongRunning,omitempty" protobuf:"bytes,3,name=isLongRunning"`
}

// Conditions maintains a list of condition
type Conditions []UsageTemplateCondition

func (c *Conditions) AreReady() bool {
	if *c != nil {
		for _, condition := range *c {
			if condition.Type == ReadyForEvaluation {
				return true
			}
		}
	}
	return false
}

func GetReadyCondtions() *Conditions {
	return &Conditions{
		UsageTemplateCondition{
			Type:   ReadyForEvaluation,
			Status: metav1.ConditionUnknown,
		},
	}
}

func (c *Conditions) SetReadyCondition(status metav1.ConditionStatus, reason string, message string) {
	if *c == nil {
		c = GetReadyCondtions()
	}
	done := c.setCondition(ReadyForEvaluation, status, reason, message)
	if !done {
		// probably don't have that condition
		*c = append(*c, UsageTemplateCondition{
			Type:               ReadyForEvaluation,
			Status:             status,
			Reason:             reason,
			Message:            message,
			LastTransitionTime: metav1.Now(),
		})
	}
}

func (c *Conditions) GetReadyCondtion() UsageTemplateCondition {
	if *c == nil {
		c = GetReadyCondtions()
	}
	cond, ok := c.getCondition(ReadyForEvaluation)
	if !ok {
		cond = UsageTemplateCondition{
			Type:   ReadyForEvaluation,
			Status: metav1.ConditionUnknown,
		}
		*c = append(*c, cond)
	}
	return cond
}

func (c Conditions) getCondition(conditionType UsageTemplateConditionType) (UsageTemplateCondition, bool) {
	for i := range c {
		if c[i].Type == conditionType {
			return c[i], true
		}
	}
	return UsageTemplateCondition{}, false
}

func (c Conditions) setCondition(conditionType UsageTemplateConditionType, status metav1.ConditionStatus, reason string, msg string) bool {
	for i := range c {
		if c[i].Type == conditionType {
			c[i].Status = status
			c[i].Reason = reason
			c[i].Message = msg
			c[i].LastTransitionTime = metav1.Now()
			return true
		}
	}

	return false
}

func (uc *UsageTemplateCondition) IsFalse() bool {
	if uc == nil {
		return false
	}
	return uc.Status == metav1.ConditionFalse
}

func (uc *UsageTemplateCondition) IsUnknown() bool {
	if uc == nil {
		return true
	}
	return uc.Status == metav1.ConditionUnknown
}

// UsageTemplateConditionType are the valid conditions of a UsageTemplate.
type UsageTemplateConditionType string

var (
	// ReadyForEvaluation indicates whether the UsageTemplate was ready for the evaluator to evaluate
	ReadyForEvaluation UsageTemplateConditionType = "ReadyForEvaluation"
	// UsageEvaluated indicates whether the UsageTemplate was able to evaluate an application's usage
	UsageEvaluated UsageTemplateConditionType = "UsageEvaluated"
	// ConfigUnsupported indicates that the UT configuration is unsupported and evaluate will not be conducted
	ConfigUnsupported UsageTemplateConditionType = "ConfigUnsupported"
)

// UsageTemplateCondition describes the state of a UsageTemplate at a certain point
type UsageTemplateCondition struct {
	// type describe the current condition
	Type UsageTemplateConditionType `json:"type" protobuf:"bytes,1,name=type"`
	// Status of the condition
	Status metav1.ConditionStatus `json:"status" description:"status of the condition, one of True, False, Unknown" protobuf:"bytes,2,name=status"`
	// lastTransitionTime is the last time the condition transitioned from one status to another
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty" protobuf:"bytes,3,opt,name=lastTransitionTime"`
	// reason is the reason for the condition's last transition
	// +optional
	Reason string `json:"reason,omitempty" protobuf:"bytes,4,opt,name=reason"`
	// message is a human-readable explanation containing details about
	// the transition
	// +optional
	Message string `json:"message,omitempty" protobuf:"bytes,5,opt,name=message"`
}

// +kubebuilder:object:root=true

// UsageTemplateList is a collection of UsageTemplates.
type UsageTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is the list of UsageTemplate
	Items []UsageTemplate `json:"items"`
}
