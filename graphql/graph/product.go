package graph

import (
	"context"
	"time"

	"github.com/Tuananh165art/GoshopX/graphql/generated"
)

type productResolver struct {
	server *Server
}

func (resolver *productResolver) Availability(ctx context.Context, obj *generated.Product) (*generated.InventoryAvailability, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	availability, err := resolver.server.inventoryClient.GetAvailability(ctx, obj.ID)
	if err != nil {
		return nil, err
	}
	return toGeneratedAvailability(availability), nil
}
