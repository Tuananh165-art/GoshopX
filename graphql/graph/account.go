package graph

import (
	"context"
	"errors"
	"time"

	"github.com/Tuananh165art/GoshopX/graphql/generated"
	"github.com/Tuananh165art/GoshopX/pkg/auth"
)

type accountResolver struct {
	server *Server
}

func (resolver *accountResolver) Orders(ctx context.Context, obj *generated.Account) ([]*generated.Order, error) {
	accountID, err := auth.GetUserIdInt(ctx, false)
	if err != nil || (accountID != obj.ID && !auth.HasAnyRole(ctx, "support_admin", "operations_admin", "platform_admin")) {
		return nil, errors.New("forbidden")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	orderList, err := resolver.server.orderClient.GetOrdersForAccount(ctx, uint64(obj.ID))
	if err != nil {
		return nil, err
	}

	orders := make([]*generated.Order, 0, len(orderList))
	for _, order := range orderList {
		orderCopy := order
		orders = append(orders, toGeneratedOrder(&orderCopy))
	}
	return orders, nil
}
