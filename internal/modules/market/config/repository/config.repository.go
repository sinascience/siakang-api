package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"siakang-api/internal/modules/market/config/domain"
	"siakang-api/pkg/logger"
)

// The three market.config keys sprint 1's PlatformConfig assembles.
const (
	keyBidAutoFeeIDR           = "bid_auto_fee_idr"
	keyBidManualFeeIDR         = "bid_manual_fee_idr"
	keyOrderAutoConfirmSeconds = "order_auto_confirm_seconds"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Get reads the seeded market.config rows and assembles the typed
// PlatformConfig. A missing key is a server error, not a zero value — a
// silently-zero fee would be a money bug, not a corner case.
func (r *Repository) Get(ctx context.Context) (*domain.PlatformConfig, error) {
	rows, err := r.db.Query(ctx,
		`SELECT key, value FROM market.config WHERE key IN ($1, $2, $3)`,
		keyBidAutoFeeIDR, keyBidManualFeeIDR, keyOrderAutoConfirmSeconds,
	)
	if err != nil {
		logger.Error("Failed to query market config", logger.Err(err))
		return nil, err
	}
	defer rows.Close()

	values := make(map[string]int64, 3)
	for rows.Next() {
		var key string
		var value int64
		if err := rows.Scan(&key, &value); err != nil {
			logger.Error("Failed to scan market config row", logger.Err(err))
			return nil, err
		}
		values[key] = value
	}
	if err := rows.Err(); err != nil {
		logger.Error("Failed to read market config rows", logger.Err(err))
		return nil, err
	}

	cfg := &domain.PlatformConfig{}
	for _, want := range []struct {
		key string
		dst *int64
	}{
		{keyBidAutoFeeIDR, &cfg.BidAutoFeeIDR},
		{keyBidManualFeeIDR, &cfg.BidManualFeeIDR},
		{keyOrderAutoConfirmSeconds, &cfg.OrderAutoConfirmSeconds},
	} {
		v, ok := values[want.key]
		if !ok {
			err := fmt.Errorf("market.config missing required key %q", want.key)
			logger.Error("Missing market config key", logger.Err(err))
			return nil, err
		}
		*want.dst = v
	}
	return cfg, nil
}
