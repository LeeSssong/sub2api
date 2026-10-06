package repository

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestRequestTimingCloseBeforeFirstWrite(t *testing.T) {
	r := &usageLogRepository{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, r.CloseRequestTiming(ctx))
	require.NoError(t, r.CloseRequestTiming(ctx))
}
