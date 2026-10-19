package config_test

import (
	"testing"
	"github.com/paulo-rocha/sdkman-ui/internal/config"
)

func TestLoad(t *testing.T) {
	cfg, err := config.Load("../../config.yaml")
	if err != nil { t.Fatal(err) }
	if cfg.Server.Port != 8080 { t.Errorf("want 8080 got %d", cfg.Server.Port) }
	if cfg.Server.Host != "127.0.0.1" { t.Errorf("want 127.0.0.1 got %s", cfg.Server.Host) }
	if cfg.Server.Addr() != "127.0.0.1:8080" { t.Errorf("wrong addr: %s", cfg.Server.Addr()) }
	t.Logf("OK: addr=%s timeout=%s queue=%d", cfg.Server.Addr(), cfg.Jobs.Timeout, cfg.Jobs.QueueSize)

	cfg2, err := config.Load("nao-existe.yaml")
	if err != nil { t.Fatal(err) }
	if cfg2.Server.Addr() != "127.0.0.1:8080" { t.Error("defaults wrong") }
	t.Logf("defaults OK: addr=%s", cfg2.Server.Addr())
}
