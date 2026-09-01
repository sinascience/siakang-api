package dto

// PlatformConfigResponse is the contract's PlatformConfig schema. Every
// amount is a JSON integer number of rupiah — never a string, never a
// float; IDR has no minor unit.
type PlatformConfigResponse struct {
	BidAutoFeeIDR           int64 `json:"bid_auto_fee_idr"`
	BidManualFeeIDR         int64 `json:"bid_manual_fee_idr"`
	OrderAutoConfirmSeconds int64 `json:"order_auto_confirm_seconds"`
}
