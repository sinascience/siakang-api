package domain

// PlatformConfig is the assembled, typed view over market.config's
// key/value rows (fees and timers the marketplace needs).
type PlatformConfig struct {
	BidAutoFeeIDR           int64
	BidManualFeeIDR         int64
	OrderAutoConfirmSeconds int64
}
