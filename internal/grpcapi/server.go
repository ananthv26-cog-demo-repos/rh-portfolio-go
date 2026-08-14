package grpcapi

import (
	"context"
	"strings"

	portfoliov1 "github.com/ananthv26-cog-demo-repos/rh-portfolio-go/gen/portfolio/v1"
	"github.com/ananthv26-cog-demo-repos/rh-portfolio-go/internal/domain"
	"github.com/ananthv26-cog-demo-repos/rh-portfolio-go/internal/money"
	"github.com/ananthv26-cog-demo-repos/rh-portfolio-go/internal/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	portfoliov1.UnimplementedPortfolioServiceServer
	Store store.PositionStore
}

func (s *Server) GetPosition(ctx context.Context, request *portfoliov1.GetPositionRequest) (*portfoliov1.Position, error) {
	if request == nil || strings.TrimSpace(request.GetAccountId()) == "" || strings.TrimSpace(request.GetSymbol()) == "" {
		return nil, status.Error(codes.InvalidArgument, "account_id and symbol are required")
	}
	markText := request.GetMark()
	if markText == "" {
		markText = "0"
	}
	mark, err := money.ParseMark(markText)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "mark must be a decimal")
	}
	symbol := strings.ToUpper(request.GetSymbol())
	lots, err := s.Store.Lots(ctx, request.GetAccountId(), symbol)
	if err != nil {
		return nil, status.Error(codes.Internal, "store error")
	}
	if len(lots) == 0 {
		return nil, status.Error(codes.NotFound, "position not found")
	}
	averageCost, err := domain.AverageCost(lots)
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}
	return &portfoliov1.Position{
		Symbol:        symbol,
		Quantity:      domain.NetQuantity(lots),
		AverageCost:   averageCost.StringFixed(4),
		UnrealizedPnl: unrealizedPnl(lots, mark),
	}, nil
}

func unrealizedPnl(lots []domain.Lot, mark money.Mark) string {
	if mark.NaN {
		if mark.NegativeNaN {
			return "-NaN"
		}
		return "NaN"
	}
	return domain.UnrealizedPnL(lots, mark.Value).StringFixed(2)
}
