package internal

import (
	"context"
	"encoding/json"
	"github.com/Tuananh165art/GoshopX/payment/models"
	"net/http"
	"time"
)

type WebhookServer struct {
	service Service
	effects func(context.Context, *models.Transaction)
}

func (s *WebhookServer) HandlePaymentWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeIPN(w, "99")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, first, code := s.service.HandleVNPAYIPN(ctx, r.URL.Query())
	if code == "00" && first && s.effects != nil {
		s.effects(ctx, tx)
	}
	writeIPN(w, code)
}
func writeIPN(w http.ResponseWriter, code string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"RspCode": code, "Message": "Confirm Success"})
}
