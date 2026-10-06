package routes

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRetiredSchedulerRoutesAreNotRegistered(t *testing.T) {
	content, err := os.ReadFile("admin.go")
	require.NoError(t, err)
	source := string(content)
	require.NotContains(t, source, `Group("/scheduler")`)
	require.NotContains(t, source, `GET("/openai-scheduler-experience"`)
}
