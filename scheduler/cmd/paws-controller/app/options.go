/*
Copyright 2020 The Kubernetes Authors.

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

package app

import (
	"github.com/spf13/pflag"
)

type ServerRunOptions struct {
	MetricsAddr          string
	HealthProbeAddr      string
	ApiServerQPS         int
	ApiServerBurst       int
	Workers              int
	EnableLeaderElection bool

	TimeoutMinutes              int
	EvaluationResolutionSeconds int
	PrometheusAddress           string
}

func NewServerRunOptions() *ServerRunOptions {
	options := &ServerRunOptions{}
	options.addAllFlags()
	return options
}

func (s *ServerRunOptions) addAllFlags() {
	pflag.StringVar(&s.MetricsAddr, "metricsAddr", ":8080", "Metrics server bind listen address.")
	pflag.StringVar(&s.HealthProbeAddr, "probeAddr", ":8081", "Probe endpoint bind address.")

	pflag.IntVar(&s.ApiServerQPS, "qps", 5, "qps of query apiserver.")
	pflag.IntVar(&s.ApiServerBurst, "burst", 10, "burst of query apiserver.")
	pflag.IntVar(&s.Workers, "workers", 1, "workers of paws-controllers.")
	pflag.BoolVar(&s.EnableLeaderElection, "enableLeaderElection", s.EnableLeaderElection, "If EnableLeaderElection for controller.")
	pflag.IntVar(&s.TimeoutMinutes, "timeoutMinutes", 1, "timeout for reconciling and pulling metrics.")
	pflag.IntVar(&s.EvaluationResolutionSeconds, "evaluationResolutionSeconds", 300, "evaluation resolution seconds for prometheus, default to 5 mins resolution.")
	pflag.StringVar(&s.PrometheusAddress, "prometheusAddress", "http://prometheus:9090", "Prometheus API address.")

}
