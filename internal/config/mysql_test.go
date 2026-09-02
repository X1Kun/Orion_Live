package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadMySQLPoolDefaults(t *testing.T) {
	for _, name := range []string{"DB_MAX_OPEN_CONNS", "DB_MAX_IDLE_CONNS", "DB_CONN_MAX_LIFETIME"} {
		t.Setenv(name, "")
	}

	cfg, err := loadMySQL()
	if err != nil {
		t.Fatalf("loadMySQL() error = %v", err)
	}
	if cfg.MaxOpenConns != 30 {
		t.Errorf("MaxOpenConns = %d, want 30", cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns != 10 {
		t.Errorf("MaxIdleConns = %d, want 10", cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime != 30*time.Minute {
		t.Errorf("ConnMaxLifetime = %s, want 30m", cfg.ConnMaxLifetime)
	}
}

func TestLoadMySQLRejectsInvalidPoolValues(t *testing.T) {
	t.Setenv("DB_MAX_OPEN_CONNS", "many")
	_, err := loadMySQL()
	if err == nil || !strings.Contains(err.Error(), "DB_MAX_OPEN_CONNS must be an integer") {
		t.Fatalf("loadMySQL() error = %v, want invalid DB_MAX_OPEN_CONNS error", err)
	}
}

func TestLoadMigrationIgnoresAPIPoolSettings(t *testing.T) {
	t.Setenv("DB_USER", "orion")
	t.Setenv("DB_PASSWORD", "secret")
	t.Setenv("DB_NAME", "orion")
	t.Setenv("DB_MAX_OPEN_CONNS", "not-used-by-migrations")

	if _, err := LoadMigration(); err != nil {
		t.Fatalf("LoadMigration() error = %v", err)
	}
}

func TestMySQLValidatePool(t *testing.T) {
	valid := MySQL{MaxOpenConns: 30, MaxIdleConns: 10, ConnMaxLifetime: 30 * time.Minute}
	if err := valid.validate(); err != nil {
		t.Fatalf("valid MySQL pool configuration rejected: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*MySQL)
	}{
		{name: "non-positive max open", mutate: func(c *MySQL) { c.MaxOpenConns = 0 }},
		{name: "negative max idle", mutate: func(c *MySQL) { c.MaxIdleConns = -1 }},
		{name: "max idle above max open", mutate: func(c *MySQL) { c.MaxIdleConns = 31 }},
		{name: "non-positive lifetime", mutate: func(c *MySQL) { c.ConnMaxLifetime = 0 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := valid
			tt.mutate(&cfg)
			if err := cfg.validate(); err == nil {
				t.Fatal("invalid MySQL pool configuration was accepted")
			}
		})
	}
}
