//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestOfficialRechargeBonusPreservesQuotaAttribution(t *testing.T) {
	for _, bonus := range []float64{0, 20} {
		t.Run(map[bool]string{true: "bonus", false: "disabled"}[bonus > 0], func(t *testing.T) {
			ctx := context.Background()
			client := newPaymentConfigServiceTestClient(t)
			user, err := client.User.Create().SetEmail("bonus@example.invalid").SetPasswordHash("fixture").SetUsername("bonus").Save(ctx)
			require.NoError(t, err)
			svc := &PaymentService{entClient: client}
			order, err := svc.createOrderInTx(ctx, CreateOrderRequest{UserID: user.ID, OrderType: payment.OrderTypeBalance, PaymentType: payment.TypeAlipay}, &User{ID: user.ID, Email: user.Email}, nil, &PaymentConfig{MaxPendingOrders: 3}, 100+bonus, 100, 0, 100, bonus, nil)
			require.NoError(t, err)
			reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
			require.NoError(t, err)
			require.Equal(t, 100+bonus, reloaded.Amount)
			require.Equal(t, bonus, reloaded.BonusAmount)
			require.Equal(t, "100", reloaded.PaidQuotaUsd.String())
			require.Equal(t, bonus, reloaded.GiftQuotaUsd.InexactFloat64())
			require.Equal(t, 100+bonus, reloaded.TotalQuotaUsd.InexactFloat64())
			require.Equal(t, "confirmed", reloaded.QuotaAccountingStatus)
		})
	}
}
