package platform

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB membungkus dua pool koneksi (PRD 18.6 keputusan #6):
//   - App   : role flowcbt_app, tunduk RLS; setiap transaksi tenant menjalankan
//     SET LOCAL app.tenant_id sehingga Postgres mengisolasi baris.
//   - System: role flowcbt_system (BYPASSRLS), hanya untuk login lintas tenant,
//     webhook, worker retensi, dan Super Admin.
type DB struct {
	App    *pgxpool.Pool
	System *pgxpool.Pool
}

// NewDB membuka kedua pool. SystemURL opsional: default memakai App
// (membantu dev tanpa role terpisah; production wajib terpisah).
func NewDB(ctx context.Context, appURL, systemURL string) (*DB, error) {
	appPool, err := pgxpool.New(ctx, appURL)
	if err != nil {
		return nil, fmt.Errorf("pool app: %w", err)
	}
	if err := appPool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping app: %w", err)
	}
	sysURL := systemURL
	if sysURL == "" {
		sysURL = appURL
	}
	sysPool, err := pgxpool.New(ctx, sysURL)
	if err != nil {
		appPool.Close()
		return nil, fmt.Errorf("pool system: %w", err)
	}
	if err := sysPool.Ping(ctx); err != nil {
		appPool.Close()
		sysPool.Close()
		return nil, fmt.Errorf("ping system: %w", err)
	}
	return &DB{App: appPool, System: sysPool}, nil
}

func (d *DB) Close() {
	d.App.Close()
	d.System.Close()
}

// WithTenantTx menjalankan fn dalam satu transaksi yang ter-scope ke tenant.
// tenantID selalu dari JWT (PRD 9), tidak pernah dari input klien.
func (d *DB) WithTenantTx(ctx context.Context, tenantID string, fn func(pgx.Tx) error) error {
	tx, err := d.App.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// SET LOCAL berlaku hanya selama transaksi ini (RLS current_tenant_id()).
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		return fmt.Errorf("set tenant scope: %w", err)
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// WithSystemTx menjalankan fn sebagai role sistem (BYPASSRLS).
// Hanya boleh dipakai auth, billing/webhook, retention, dan admin (depguard).
func (d *DB) WithSystemTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := d.System.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
