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
	symbol := strings.ToUpper(request.GetSymbol())
	lots, err := s.Store.Lots(ctx, request.GetAccountId(), symbol)
	if err != nil {
		return nil, status.Error(codes.Internal, "store error")
	}
	if len(lots) == 0 {
		return nil, status.Error(codes.NotFound, "position not found")
	}
	markText := request.GetMark()
	if markText == "" {
		markText = "0"
	}
	mark, err := money.ParseMark(markText)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "mark must be a decimal")
	}
	averageCost, err := domain.AverageCost(lots)
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}
	averageCostText, err := averageCost.StringFixed(4)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid position")
	}
	pnlText, err := unrealizedPnl(lots, mark)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "mark exceeds decimal precision")
	}
	return &portfoliov1.Position{
		Symbol:        symbol,
		Quantity:      domain.NetQuantity(lots),
		AverageCost:   averageCostText,
		UnrealizedPnl: pnlText,
	}, nil
}

func unrealizedPnl(lots []domain.Lot, mark money.Mark) (string, error) {
	if mark.NaN {
		if mark.NegativeNaN {
			return "-NaN", nil
		}
		return "NaN", nil
	}
	value, err := domain.UnrealizedPnL(lots, mark.Value)
	if err != nil {
		return "", err
	}
	return value.StringFixed(2)
}
