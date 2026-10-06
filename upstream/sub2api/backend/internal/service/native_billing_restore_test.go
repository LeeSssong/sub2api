package service

import (
	"github.com/stretchr/testify/require"
	"testing"
)

// Account-stat reporting must not override the native account quota charge.
func TestNativeAccountQuotaIgnoresStatsOverride(t *testing.T) {
	for _, tc := range []struct {
		name     string
		override float64
		explicit bool
	}{
		{"nonzero", 99, true}, {"explicit_zero", 0, true}, {"legacy_nonzero", 99, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &postUsageBillingParams{Cost: &CostBreakdown{TotalCost: 2}, AccountRateMultiplier: 0.15, AccountCost: tc.override, AccountCostSet: tc.explicit}
			require.InDelta(t, 0.3, accountCostForBilling(p), 1e-12)
		})
	}
}
