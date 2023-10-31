// Copyright (c) Huawei Technologies Co., Ltd. 2023. All rights reserved.
// paws licensed under the Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//     http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR
// PURPOSE.
// See the Mulan PSL v2 for more details.
// Author: Wei Wei
// Create: 2023-09-18

package v1beta3

var (
	// DefaultHotSpotThreshold is the default value for HotSpotThreshold
	DefaultHotSpotThreshold = 60
	// DefaultHardThresholdValue is the default value for HardThreshold
	DefaultHardThresholdValue = false
	// DefaultEnableOvercommitValue is the default value for EnableOvercommit
	DefaultEnableOvercommitValue = true
)

// SetDefaults_TemporalUtilizationArgs
func SetDefaults_TemporalUtilizationArgs(args *TemporalUtilizationArgs) {
	if args.HardThreshold == nil {
		args.HardThreshold = new(bool)
		*args.HardThreshold = DefaultHardThresholdValue
	}

	if args.HotSpotThreshold == nil {
		args.HotSpotThreshold = new(int32)
		*args.HotSpotThreshold = int32(DefaultHotSpotThreshold)
	}

	if args.EnableOvercommit == nil {
		args.EnableOvercommit = new(bool)
		*args.EnableOvercommit = DefaultEnableOvercommitValue
	}
}
