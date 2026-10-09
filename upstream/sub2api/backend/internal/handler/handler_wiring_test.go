package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestProvideHandlersWiresMonitorV4(t *testing.T) {
	monitorV4 := NewMonitorV4Handler(nil)
	handlers := ProvideHandlers(
		nil, nil, nil, nil, nil, nil, nil, nil,
		nil, monitorV4, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
	)
	if handlers == nil {
		t.Fatal("ProvideHandlers() returned nil")
	}
	if handlers.MonitorV4 != monitorV4 {
		t.Fatalf("MonitorV4 handler = %p, want %p", handlers.MonitorV4, monitorV4)
	}
}

func TestProvideHandlersReusesModelPlazaPricingService(t *testing.T) {
	pricing := &service.ModelPlazaService{}
	gateway := &GatewayHandler{}
	plaza := NewModelPlazaHandler(pricing, nil, nil)
	ProvideHandlers(
		nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, gateway,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, plaza,
		nil, nil, nil, nil, nil,
	)
	if gateway.modelPlazaService != pricing {
		t.Fatal("gateway must reuse the existing model plaza service")
	}
}
