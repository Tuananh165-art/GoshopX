# Google Login và Gmail notification

## Kiến trúc

- Web dùng Google Identity Services để lấy Google ID token.
- `graphql` chỉ chuyển credential qua gRPC; `account` validate token bằng `GOOGLE_CLIENT_ID`, kiểm tra `email_verified`, upsert account theo email và phát hành JWT cookie hiện có.
- `notification` vẫn lưu in-app notification từ Kafka trước. Nếu event payload có `recipient_email` hoặc `email` và Gmail SMTP đã cấu hình, service gửi thêm email. SMTP lỗi không làm fail business event sau khi in-app row đã được lưu.

Gmail App Password không phải API key. Không dùng mật khẩu Gmail chính và không commit secret.

## 1. Tạo Google OAuth client

1. Mở Google Cloud Console: https://console.cloud.google.com/
2. Tạo hoặc chọn project.
3. Vào **Google Auth Platform → Branding** và hoàn thiện app name/support email nếu Google yêu cầu.
4. Vào **Google Auth Platform → Clients → Create client**.
5. Chọn application type **Web application**.
6. Thêm JavaScript origin cho local:
   - `http://localhost:5173` khi chạy Vite trực tiếp.
   - Origin domain thật khi deploy production.
7. Copy **Client ID**. Không đưa Client Secret vào frontend.
8. Nếu consent screen ở chế độ Testing, thêm tài khoản developer vào **Audience → Test users**.

## 2. Tạo Gmail App Password

1. Dùng Gmail account chuyên gửi notification, không nên dùng account cá nhân chính.
2. Bật 2-Step Verification tại https://myaccount.google.com/security.
3. Mở https://myaccount.google.com/apppasswords.
4. Tạo app password tên `GoshopX Notification`.
5. Copy chuỗi 16 ký tự một lần; đây là `GMAIL_PASSWORD`.

Nếu tài khoản thuộc Google Workspace và không thấy App Passwords, admin có thể đã tắt tính năng này. Khi đó dùng SMTP provider transactional hoặc Gmail API OAuth/service account thay vì dùng password thường.

## 3. Cấu hình `.env`

```dotenv
GOOGLE_CLIENT_ID=1234567890-xxxxx.apps.googleusercontent.com

GMAIL_SMTP_HOST=smtp.gmail.com
GMAIL_SMTP_PORT=587
GMAIL_USERNAME=notifications@example.com
GMAIL_PASSWORD=xxxx xxxx xxxx xxxx
GMAIL_FROM=notifications@example.com
```

`GOOGLE_CLIENT_ID` là public identifier và được dùng khi build web. `GMAIL_PASSWORD`, database password, JWT secret chỉ để trong `.env` local hoặc secret manager; không điền vào `.env.example`.

## 4. Chạy local

### Vite trực tiếp

Tạo `web/.env.local` (không commit):

```dotenv
VITE_GOOGLE_CLIENT_ID=1234567890-xxxxx.apps.googleusercontent.com
VITE_GRAPHQL_URL=http://localhost:8080/graphql
```

Chạy backend/Compose rồi chạy web:

```bash
docker compose config --quiet
docker compose up --build -d account notification graphql kong
cd web && npm run dev
```

### Docker web

Root `.env` dùng `GOOGLE_CLIENT_ID`; Compose truyền giá trị này thành build arg `VITE_GOOGLE_CLIENT_ID` cho web image:

```bash
docker compose config --quiet
docker compose up --build -d
```

## 5. Event recipient contract

Email notification chỉ gửi khi event data có một trong hai key:

```json
{"recipient_email":"customer@example.com"}
```

hoặc:

```json
{"email":"customer@example.com"}
```

Producer phải lấy email từ service sở hữu account/business context và đưa vào event payload; `notification` không đọc trực tiếp `account_db`. Các event không có email vẫn tạo in-app notification bình thường.

## Troubleshooting

- `google login is not configured`: kiểm tra `GOOGLE_CLIENT_ID` trong service `account`, rồi rebuild/recreate container.
- Google `origin not allowed`: thêm đúng origin, gồm scheme và port, trong OAuth client.
- `invalid google identity`: token hết hạn, sai audience, issuer không phải Google, hoặc email chưa verified.
- Gmail `535 Authentication failed`: dùng App Password 16 ký tự, không dùng mật khẩu Gmail thường; kiểm tra 2-Step Verification và username.
- Không có email nhưng có in-app notification: kiểm tra event có `recipient_email`/`email` và log notification container.
