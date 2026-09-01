package domain

// FeatureKey enumerates the feature slots that can be gated by approval flow.
// Keep values lowercase snake_case to match the CHECK constraint in migration
// core/000007_approval_system.up.sql.
const (
	FeatureJournalEntry = "journal_entry"
	FeatureCashIn       = "cash_in"
	FeatureCashOut      = "cash_out"
	FeatureTransfer     = "transfer"
	FeatureExpense      = "expense"
	FeatureInvoice      = "invoice"
	FeatureA1Budget     = "a1_budget"
	FeatureA2Budget     = "a2_budget"
	FeatureA3Budget     = "a3_budget"
)

// knownFeatureKeys is the authoritative set used by IsKnownFeatureKey. Adding
// a new feature means registering it here AND calling approval.Submit with the
// same constant.
var knownFeatureKeys = map[string]struct{}{
	FeatureJournalEntry: {},
	FeatureCashIn:       {},
	FeatureCashOut:      {},
	FeatureTransfer:     {},
	FeatureExpense:      {},
	FeatureInvoice:      {},
	FeatureA1Budget:     {},
	FeatureA2Budget:     {},
	FeatureA3Budget:     {},
}

// IsKnownFeatureKey reports whether the given key is registered.
func IsKnownFeatureKey(key string) bool {
	_, ok := knownFeatureKeys[key]
	return ok
}
