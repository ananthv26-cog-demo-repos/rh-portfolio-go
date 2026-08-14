package domain

import (
	"testing"

	"github.com/ananthv26-cog-demo-repos/rh-portfolio-go/internal/money"
)

func TestUnrealizedPnLSignedZero(t *testing.T) {
	price, err := money.Parse("1.000000")
	if err != nil {
		t.Fatal(err)
	}
	otherPrice, err := money.Parse("2.000000")
	if err != nil {
		t.Fatal(err)
	}
	mark, err := money.Parse("1.5714")
	if err != nil {
		t.Fatal(err)
	}
	value, err := UnrealizedPnL([]Lot{
		{Quantity: 3, Price: price},
		{Quantity: 4, Price: otherPrice},
	}, mark)
	if err != nil {
		t.Fatal(err)
	}
	got, err := value.StringFixed(2)
	if err != nil {
		t.Fatal(err)
	}
	if got != "-0.00" {
		t.Fatalf("pnl = %q, want -0.00", got)
	}
}
