package domain

import (
	"errors"

	"github.com/ananthv26-cog-demo-repos/rh-portfolio-go/internal/money"
)

var ErrNoOpenPosition = errors.New("no open position")

type Lot struct {
	Quantity int64
	Price    money.Decimal
}

func NetQuantity(lots []Lot) int64 {
	var quantity int64
	for _, lot := range lots {
		quantity += lot.Quantity
	}
	return quantity
}

func TotalCost(lots []Lot) money.Decimal {
	total, _ := money.Parse("0")
	for _, lot := range lots {
		total = total.Add(lot.Price.MulInt64(lot.Quantity))
	}
	return total
}

func AverageCost(lots []Lot) (money.Decimal, error) {
	quantity := NetQuantity(lots)
	if quantity <= 0 {
		return money.Decimal{}, ErrNoOpenPosition
	}
	return TotalCost(lots).DivideQuantized(quantity, 4), nil
}

func UnrealizedPnL(lots []Lot, mark money.Decimal) money.Decimal {
	quantity := NetQuantity(lots)
	return mark.MulInt64(quantity).Sub(TotalCost(lots)).Quantize(2)
}
