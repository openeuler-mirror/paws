/*
Copyright (c) Huawei Technologies Co., Ltd. 2023. All rights reserved.
paws licensed under the Mulan PSL v2.
You can use this software according to the terms and conditions of the Mulan PSL v2.
You may obtain a copy of Mulan PSL v2 at:
   http://license.coscl.org.cn/MulanPSL2
THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR
PURPOSE.
See the Mulan PSL v2 for more details.
Author: Ging Fung Yeung
Create: 2023-10-27
*/

// Package v1alpha1 contains API Schema definitions for the scheduling.x-k8s.io v1alpha1 API group
// +kubebuilder:object:generate=true
// +groupName=scheduling.x-k8s.io

package v1alpha1

import (
	"gitee.com/openeuler/paws/scheduler/apis/scheduling"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// TODO: This is currently in scheduling x, maybe change it to highlander ?
	SchemeGroupVersion = schema.GroupVersion{Group: scheduling.GroupName, Version: "v1alpha1"}
	// localSchemeBuilder and AddToScheme will stay in k8s.io/kubernetes.
	SchemeBuilder      runtime.SchemeBuilder
	localSchemeBuilder = &SchemeBuilder
	AddToScheme        = localSchemeBuilder.AddToScheme
)

// Resource is required by pkg/client/listers/...
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

func init() {
	// We only register manually written functions here. The registration of the
	// generated functions takes place in the generated files. The separation
	// makes the code compile even when the generated files are missing.
	localSchemeBuilder.Register(addKnownTypes)
}

// Adds the list of known types to Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&UsageTemplate{}, &UsageTemplateList{})
	// AddToGroupVersion allows the serialization of client types like ListOptions.
	v1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
