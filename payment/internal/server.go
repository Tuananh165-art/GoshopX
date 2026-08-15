package internal

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/IBM/sarama"
	cart "github.com/Tuananh165art/GoshopX/cart/client"
	inventory "github.com/Tuananh165art/GoshopX/inventory/client"
	order "github.com/Tuananh165art/GoshopX/order/client"
	"github.com/Tuananh165art/GoshopX/payment/models"
	"github.com/Tuananh165art/GoshopX/payment/proto/pb"
	"github.com/Tuananh165art/GoshopX/pkg/observability"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func StartServers(service Service, _ sarama.Consumer, orderURL, inventoryURL, cartURL string, grpcPort, webhookPort int) error {
	go func() { _ = listenWebhook(service, orderURL, inventoryURL, cartURL, webhookPort) }()
	return ListenGRPC(service, grpcPort)
}
func ListenGRPC(service Service, port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	server := grpc.NewServer(observability.GRPCServerOptions("payment")...)
	pb.RegisterPaymentServiceServer(server, &grpcServer{service: service})
	reflection.Register(server)
	return server.Serve(lis)
}
func listenWebhook(service Service, orderURL, inventoryURL, cartURL string, port int) error {
	orders, err := order.NewClient(orderURL)
	if err != nil {
		return err
	}
	defer orders.Close()
	stock, err := inventory.NewClient(inventoryURL)
	if err != nil {
		return err
	}
	defer stock.Close()
	carts, err := cart.NewClient(cartURL)
	if err != nil {
		return err
	}
	defer carts.Close()
	apply := func(ctx context.Context, tx *models.Transaction) {
		commitOK := true
		for _, reservationID := range reservationIDs(tx.ReservationIDs) {
			var err error
			for attempt := 0; attempt < 3; attempt++ {
				if tx.Status == models.Success.String() {
					_, _, err = stock.CommitReservation(ctx, reservationID)
				} else {
					_, _, err = stock.ReleaseReservation(ctx, reservationID)
				}
				if err == nil {
					break
				}
				time.Sleep(time.Duration(attempt+1) * 200 * time.Millisecond)
			}
			if err != nil {
				commitOK = false
				log.Printf("payment effects: reservation %s transition failed: %v", reservationID, err)
			}
		}
		if !commitOK {
			return
		}
		if err := orders.UpdateOrderStatus(ctx, tx.OrderId, tx.Status); err != nil {
			log.Printf("payment effects: order %d status update failed: %v", tx.OrderId, err)
			return
		}
		_ = carts.ClearCart(ctx, tx.UserId)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook/payment", (&WebhookServer{service: service, effects: apply}).HandlePaymentWebhook)
	return http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
}
func reservationIDs(csv string) []string {
	var out []string
	for _, part := range strings.Split(csv, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
