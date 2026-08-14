package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/Tuananh165art/GoshopX/notification/models"
	sharedevents "github.com/Tuananh165art/GoshopX/pkg/events"
)

type Service interface {
	ListNotifications(ctx context.Context, accountID uint64, skip, take uint64) ([]*models.Notification, error)
	CountUnread(ctx context.Context, accountID uint64) (int64, error)
	MarkRead(ctx context.Context, accountID uint64, notificationID uint64) (bool, error)
	MarkAllRead(ctx context.Context, accountID uint64) (int64, error)
	CreateFromEvent(ctx context.Context, event *sharedevents.Envelope) error
}

type notificationService struct {
	repository    Repository
	emailSender   EmailSender
	emailResolver AccountEmailResolver
}

type AccountEmailResolver interface {
	ResolveEmail(ctx context.Context, accountID uint64) (string, error)
}

func NewNotificationService(repository Repository, emailSenders ...EmailSender) Service {
	var emailSender EmailSender
	if len(emailSenders) > 0 {
		emailSender = emailSenders[0]
	}
	return &notificationService{repository: repository, emailSender: emailSender}
}

func NewNotificationServiceWithEmailResolver(repository Repository, emailSender EmailSender, emailResolver AccountEmailResolver) Service {
	return &notificationService{repository: repository, emailSender: emailSender, emailResolver: emailResolver}
}

func (service *notificationService) ListNotifications(ctx context.Context, accountID uint64, skip, take uint64) ([]*models.Notification, error) {
	return service.repository.ListNotifications(ctx, accountID, skip, take)
}

func (service *notificationService) CountUnread(ctx context.Context, accountID uint64) (int64, error) {
	return service.repository.CountUnread(ctx, accountID)
}

func (service *notificationService) MarkRead(ctx context.Context, accountID uint64, notificationID uint64) (bool, error) {
	return service.repository.MarkRead(ctx, accountID, notificationID)
}

func (service *notificationService) MarkAllRead(ctx context.Context, accountID uint64) (int64, error) {
	return service.repository.MarkAllRead(ctx, accountID)
}

func (service *notificationService) CreateFromEvent(ctx context.Context, event *sharedevents.Envelope) error {
	if event == nil || event.AccountID == 0 {
		return nil
	}

	title, message := notificationCopy(event.EventType)
	if title == "" && message == "" {
		return nil
	}

	body, err := json.Marshal(event.Data)
	if err != nil {
		return err
	}
	emailSubject, emailBody := notificationEmail(event.EventType, title, message, event.Data)

	notification := &models.Notification{
		AccountID:    event.AccountID,
		EventID:      event.EventID,
		EventType:    event.EventType,
		Title:        title,
		Message:      message,
		MetadataJSON: string(body),
		IsRead:       false,
		CreatedAt:    time.Now().UTC(),
	}
	if err := service.repository.CreateNotification(ctx, notification); err != nil {
		return err
	}
	if service.emailSender != nil && shouldSendEmail(event) {
		if recipient := event.RecipientEmail; recipient != "" {
			if err := service.emailSender.Send(ctx, recipient, emailSubject, emailBody); err != nil {
				log.Printf("notification email delivery failed for %s: %v", recipient, err)
				return nil
			}
		} else if recipient := eventRecipient(event.Data); recipient != "" {
			if err := service.emailSender.Send(ctx, recipient, emailSubject, emailBody); err != nil {
				// Email delivery is best-effort; in-app notification is already durable.
				log.Printf("notification email delivery failed for %s: %v", recipient, err)
				return nil
			}
		} else if service.emailResolver != nil && event.AccountID != 0 {
			if recipient, err := service.emailResolver.ResolveEmail(ctx, event.AccountID); err == nil && recipient != "" {
				if err := service.emailSender.Send(ctx, recipient, emailSubject, emailBody); err != nil {
					log.Printf("notification email delivery failed for %s: %v", recipient, err)
				}
			}
		}
	}
	return nil
}

func eventRecipient(data any) string {
	values, ok := data.(map[string]any)
	if !ok {
		return ""
	}
	for _, key := range []string{"recipient_email", "email"} {
		if value, ok := values[key].(string); ok {
			return value
		}
	}
	return ""
}

func shouldSendEmail(event *sharedevents.Envelope) bool {
	if event == nil {
		return false
	}
	switch event.EventType {
	case "payment.succeeded", "auth.password_reset_otp", "order.cod_created":
		return true
	case "order.payment_status_updated":
		values, ok := event.Data.(map[string]any)
		if !ok {
			return false
		}
		status := strings.ToLower(stringValue(values["payment_status"]))
		return status == "paid" || status == "succeeded" || status == "success" || status == "completed"
	default:
		return false
	}
}

func notificationEmail(eventType, title, message string, data any) (string, string) {
	values, ok := data.(map[string]any)
	if !ok {
		return title, message
	}

	orderID := numberText(values["order_id"])
	emailTitle := title
	if orderID != "" {
		emailTitle = fmt.Sprintf("%s - Đơn hàng #%s", title, orderID)
	}

	var body strings.Builder
	body.WriteString(message)
	if eventType == "auth.password_reset_otp" {
		if otp := stringValue(values["otp"]); otp != "" {
			body.WriteString("\n\nMã OTP đổi mật khẩu: ")
			body.WriteString(otp)
			body.WriteString("\nMã OTP chỉ dùng một lần và có thời hạn ngắn.")
		}
	}
	if orderID != "" {
		body.WriteString("\n\nĐơn hàng #")
		body.WriteString(orderID)
	}
	if status := stringValue(values["status"]); status != "" {
		body.WriteString("\nTrạng thái đơn hàng: ")
		body.WriteString(vietnameseStatus(status))
	}
	if status := stringValue(values["payment_status"]); status != "" {
		body.WriteString("\nTrạng thái thanh toán: ")
		body.WriteString(vietnameseStatus(status))
	}
	if total := formatVND(values["total_price"]); total != "" {
		body.WriteString("\nTổng tiền: ")
		body.WriteString(total)
	}

	if rawProducts, ok := values["products"].([]any); ok && len(rawProducts) > 0 {
		body.WriteString("\n\nChi tiết sản phẩm:")
		for index, rawProduct := range rawProducts {
			product, ok := rawProduct.(map[string]any)
			if !ok {
				continue
			}
			body.WriteString(fmt.Sprintf("\n%d. %s", index+1, stringValue(product["name"])))
			body.WriteString("\n   Mã sản phẩm: ")
			body.WriteString(stringValue(product["id"]))
			body.WriteString("\n   Mô tả: ")
			body.WriteString(stringValue(product["description"]))
			body.WriteString("\n   Đơn giá: ")
			body.WriteString(formatVND(product["price"]))
			body.WriteString("\n   Số lượng: ")
			body.WriteString(numberText(product["quantity"]))
		}
	}
	return emailTitle, body.String()
}

func formatVND(value any) string {
	raw := strings.TrimSpace(numberText(value))
	if raw == "" {
		return ""
	}
	amount, err := strconv.ParseInt(strings.ReplaceAll(raw, ",", ""), 10, 64)
	if err != nil {
		return raw + " ₫"
	}
	text := strconv.FormatInt(amount, 10)
	for index := len(text) - 3; index > 0; index -= 3 {
		text = text[:index] + "." + text[index:]
	}
	return text + " ₫"
}

func vietnameseStatus(status string) string {
	switch strings.ToLower(status) {
	case "pending":
		return "Đang chờ xử lý"
	case "paid", "succeeded", "success", "completed":
		return "Đã thanh toán"
	case "cod_pending":
		return "Thanh toán khi nhận hàng"
	case "failed":
		return "Thanh toán thất bại"
	default:
		return status
	}
}

func stringValue(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func numberText(value any) string {
	switch number := value.(type) {
	case float64:
		return fmt.Sprintf("%.0f", number)
	case float32:
		return fmt.Sprintf("%.0f", number)
	case int:
		return fmt.Sprintf("%d", number)
	case int64:
		return fmt.Sprintf("%d", number)
	case uint:
		return fmt.Sprintf("%d", number)
	case uint64:
		return fmt.Sprintf("%d", number)
	case string:
		return number
	default:
		return ""
	}
}

func notificationCopy(eventType string) (string, string) {
	switch eventType {
	case "auth.password_reset_otp":
		return "Mã OTP đổi mật khẩu", "Bạn vừa yêu cầu đổi mật khẩu. Dùng mã OTP bên dưới để tiếp tục."
	case "payment.succeeded":
		return "Thanh toán thành công", "Thanh toán đơn hàng của bạn đã được xác nhận thành công."
	case "payment.failed":
		return "Thanh toán thất bại", "Thanh toán chưa thành công. Vui lòng thử lại hoặc chọn phương thức khác."
	case "order.created":
		return "Đơn hàng đã được tạo", "Đơn hàng của bạn đã được tạo và đang chờ thanh toán."
	case "order.cod_created":
		return "Đặt đơn COD thành công", "Đơn COD của bạn đã được tiếp nhận và đang chờ thu tiền."
	case "order.payment_status_updated":
		return "Cập nhật thanh toán đơn hàng", "Trạng thái thanh toán đơn hàng của bạn vừa được cập nhật."
	case "inventory.released":
		return "Reservation released", "A reserved item was released back to stock."
	case "inventory.committed":
		return "Stock committed", "Your reserved stock was committed after payment."
	case "cart.checkout_prepared":
		return "Checkout ready", "Your reserved cart is ready for checkout."
	default:
		if strings.HasPrefix(eventType, "cart.") {
			return "Cart updated", "Your shopping cart was updated."
		}
		if strings.HasPrefix(eventType, "inventory.") {
			return "Inventory updated", fmt.Sprintf("Inventory event received: %s", eventType)
		}
		return "", ""
	}
}
