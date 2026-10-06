package mihomo

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestRegularSpeedPriorityKeepsAffinity(t *testing.T) {
	m := bpsTestManager(t)
	now := time.Now()
	ids := []string{}
	for id := range m.bpsPorts {
		ids = append(ids, id)
		m.bpsHealthLocked(id).verifiedUntil = now.Add(time.Minute)
	}
	m.bpsHealthLocked(ids[0]).proxyLatency = 100 * time.Millisecond
	m.bpsHealthLocked(ids[1]).proxyLatency = time.Second
	l, err := m.acquireBPSLease(context.Background(), "regular:one", nil)
	require.NoError(t, err)
	require.Equal(t, ids[0], l.node)
	l.Release()
	m.bpsHealthLocked(ids[1]).proxyLatency = time.Millisecond
	l, err = m.acquireBPSLease(context.Background(), "regular:one", nil)
	require.NoError(t, err)
	require.Equal(t, ids[0], l.node)
	l.Release()
	l, err = m.acquireBPSLease(context.Background(), "regular:two", nil)
	require.NoError(t, err)
	require.Equal(t, ids[1], l.node)
	l.Release()
	m.bpsHealthLocked(ids[1]).retryAfter = now.Add(time.Minute)
	l, err = m.acquireBPSLease(context.Background(), "regular:three", nil)
	require.NoError(t, err)
	require.Equal(t, ids[0], l.node)
	l.Release()
}
