package grpcapi

import (
	"context"
	"testing"

	portfoliov1 "github.com/ananthv26-cog-demo-repos/rh-portfolio-go/gen/portfolio/v1"
	"github.com/ananthv26-cog-demo-repos/rh-portfolio-go/internal/domain"
	"github.com/ananthv26-cog-demo-repos/rh-portfolio-go/internal/money"
	"github.com/ananthv26-cog-demo-repos/rh-portfolio-go/internal/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetPosition(t *testing.T) {
	price, _ := money.Parse("19.990000")
	server := &Server{Store: &store.MemoryStore{Positions: map[string][]domain.Lot{
		"acct\x00HOOD": {{Quantity: 10, Price: price}},
	}}}

	position, err := server.GetPosition(context.Background(), &portfoliov1.GetPositionRequest{
		AccountId: "acct",
		Symbol:    "HOOD",
		Mark:      "20",
	})
	if err != nil {
		t.Fatal(err)
	}
	if position.GetSymbol() != "HOOD" || position.GetQuantity() != 10 ||
		position.GetAverageCost() != "19.9900" || position.GetUnrealizedPnl() != "0.10" {
		t.Fatalf("unexpected position: %+v", position)
	}
}

func TestGetPositionPayloadNaN(t *testing.T) {
	price, _ := money.Parse("19.990000")
	server := &Server{Store: &store.MemoryStore{Positions: map[string][]domain.Lot{
		"acct\x00HOOD": {{Quantity: 10, Price: price}},
	}}}
	position, err := server.GetPosition(context.Background(), &portfoliov1.GetPositionRequest{
		AccountId: "acct",
		Symbol:    "HOOD",
		Mark:      "-NaN007",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := position.GetUnrealizedPnl(); got != "-NaN7" {
		t.Fatalf("unrealized_pnl = %q, want %q", got, "-NaN7")
	}
}

func TestGetPositionUnicodeDecimalMark(t *testing.T) {
	price, _ := money.Parse("19.990000")
	server := &Server{Store: &store.MemoryStore{Positions: map[string][]domain.Lot{
		"acct\x00HOOD": {{Quantity: 10, Price: price}},
	}}}
	position, err := server.GetPosition(context.Background(), &portfoliov1.GetPositionRequest{
		AccountId: "acct",
		Symbol:    "HOOD",
		Mark:      "２０",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := position.GetUnrealizedPnl(); got != "0.10" {
		t.Fatalf("unrealized_pnl = %q, want %q", got, "0.10")
	}
}

func TestGetPositionPreservesIdentifierWhitespace(t *testing.T) {
	price, _ := money.Parse("19.990000")
	server := &Server{Store: &store.MemoryStore{Positions: map[string][]domain.Lot{
		"acct\x00HOOD": {{Quantity: 10, Price: price}},
	}}}
	_, err := server.GetPosition(context.Background(), &portfoliov1.GetPositionRequest{
		AccountId: " acct ",
		Symbol:    "HOOD",
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("whitespace-preserving lookup code = %s", status.Code(err))
	}
}

func TestGetPositionRejectsWhitespaceOnlyIdentifiers(t *testing.T) {
	server := &Server{Store: &store.MemoryStore{}}
	_, err := server.GetPosition(context.Background(), &portfoliov1.GetPositionRequest{
		AccountId: " \t",
		Symbol:    "HOOD",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("whitespace-only account code = %s", status.Code(err))
	}
}

func TestStoredPrecisionOverflowIsInternal(t *testing.T) {
	price, err := money.Parse("10000000000000000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{Store: &store.MemoryStore{Positions: map[string][]domain.Lot{
		"acct\x00HOOD": {{Quantity: 1, Price: price}},
	}}}
	_, err = server.GetPosition(context.Background(), &portfoliov1.GetPositionRequest{
		AccountId: "acct",
		Symbol:    "HOOD",
	})
	if status.Code(err) != codes.Internal {
		t.Fatalf("stored precision code = %s", status.Code(err))
	}
	if status.Convert(err).Message() != "invalid stored position" {
		t.Fatalf("stored precision message = %q", status.Convert(err).Message())
	}
}

func TestMarkPrecisionOverflowIsInvalidArgument(t *testing.T) {
	price, _ := money.Parse("19.990000")
	server := &Server{Store: &store.MemoryStore{Positions: map[string][]domain.Lot{
		"acct\x00HOOD": {{Quantity: 10, Price: price}},
	}}}
	_, err := server.GetPosition(context.Background(), &portfoliov1.GetPositionRequest{
		AccountId: "acct",
		Symbol:    "HOOD",
		Mark:      "1e26",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("mark precision code = %s", status.Code(err))
	}
	if status.Convert(err).Message() != "mark exceeds decimal precision" {
		t.Fatalf("mark precision message = %q", status.Convert(err).Message())
	}
}

func TestGetPositionStatusMappings(t *testing.T) {
	price, _ := money.Parse("19.990000")
	server := &Server{Store: &store.MemoryStore{Positions: map[string][]domain.Lot{
		"acct\x00HOOD": {{Quantity: 10, Price: price}},
	}}}
	_, err := server.GetPosition(context.Background(), &portfoliov1.GetPositionRequest{
		AccountId: "acct",
		Symbol:    "NOPE",
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("not found code = %s", status.Code(err))
	}
	_, err = server.GetPosition(context.Background(), &portfoliov1.GetPositionRequest{
		AccountId: "acct",
		Symbol:    "HOOD",
		Mark:      "not-a-number",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("invalid mark code = %s", status.Code(err))
	}
	_, err = server.GetPosition(context.Background(), &portfoliov1.GetPositionRequest{
		AccountId: "acct",
		Symbol:    "NOPE",
		Mark:      "not-a-number",
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("unknown position precedence code = %s", status.Code(err))
	}
}
