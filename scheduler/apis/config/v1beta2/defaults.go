package v1beta2

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
