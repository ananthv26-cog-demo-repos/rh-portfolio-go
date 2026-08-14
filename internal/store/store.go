package store

import (
	"context"
	"errors"
	"strings"

	"github.com/ananthv26-cog-demo-repos/rh-portfolio-go/internal/domain"
	"github.com/ananthv26-cog-demo-repos/rh-portfolio-go/internal/money"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PositionStore interface {
	Lots(ctx context.Context, accountID, symbol string) ([]domain.Lot, error)
}

type MemoryStore struct {
	Positions map[string][]domain.Lot
}

func (s *MemoryStore) Lots(_ context.Context, accountID, symbol string) ([]domain.Lot, error) {
	lots := s.Positions[accountID+"\x00"+strings.ToUpper(symbol)]
	return append([]domain.Lot(nil), lots...), nil
}

type PostgresStore struct {
	Pool *pgxpool.Pool
}

func (s *PostgresStore) Lots(ctx context.Context, accountID, symbol string) ([]domain.Lot, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT quantity, price::text
		FROM portfolio_lot
		WHERE account_id = $1 AND symbol = $2
		ORDER BY acquired_at, id
	`, accountID, strings.ToUpper(symbol))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lots []domain.Lot
	for rows.Next() {
		var quantity int64
		var priceText string
		if err := rows.Scan(&quantity, &priceText); err != nil {
			return nil, err
		}
		price, err := money.Parse(priceText)
		if err != nil {
			return nil, err
		}
		lots = append(lots, domain.Lot{Quantity: quantity, Price: price})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return lots, nil
}

var ErrStoreUnavailable = errors.New("store unavailable")
