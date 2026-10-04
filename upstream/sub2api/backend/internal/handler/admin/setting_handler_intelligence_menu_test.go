package admin

import (
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
)

func TestSettingsIntelligenceInternalMenu(t *testing.T) {
	for _, tc := range []struct {
		url    string
		status int
	}{{"/intelligence-test", http.StatusOK}, {"/admin/settings", http.StatusBadRequest}, {"//evil.example", http.StatusBadRequest}, {"javascript:alert(1)", http.StatusBadRequest}} {
		t.Run(tc.url, func(t *testing.T) {
			h, _ := newStepUpSwitchTestHandler(t, map[string]string{})
			rec := doUpdateSettings(t, h, map[string]any{"custom_menu_items": []map[string]any{{"id": "intelligence-test", "label": "智商检测", "url": tc.url, "visibility": "user", "sort_order": 0}}}, nil)
			require.Equal(t, tc.status, rec.Code, rec.Body.String())
		})
	}
}
