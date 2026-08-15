# Google Sign-In and Gmail Notifications

## Architecture

- The web client uses Google Identity Services to obtain a Google ID token.
- `graphql` forwards the credential only through gRPC. `account` validates the token with `GOOGLE_CLIENT_ID`, checks `email_verified`, upserts the account by email, and issues the existing JWT cookie.
- `notification` first persists the Kafka-driven in-app notification. If the event payload contains `recipient_email` or `email` and Gmail SMTP is configured, the service also sends email. SMTP delivery failure does not fail the business event after the in-app row has been saved.

A Gmail App Password is not an API key. Do not use the primary Gmail password or commit secrets.

## 1. Create a Google OAuth client

1. Open the [Google Cloud Console](https://console.cloud.google.com/).
2. Create or select a project.
3. Go to **Google Auth Platform → Branding** and complete the application name and support email if Google requests them.
4. Go to **Google Auth Platform → Clients → Create client**.
5. Select the **Web application** application type.
6. Add a JavaScript origin for local development:
   - `http://localhost:5173` when running Vite directly.
   - The real domain origin when deploying to production.
7. Copy the **Client ID**. Never put the Client Secret in the frontend.
8. If the consent screen is in Testing mode, add developer accounts under **Audience → Test users**.

## 2. Create a Gmail App Password

1. Use a dedicated Gmail account for notification delivery rather than a primary personal account.
2. Enable [2-Step Verification](https://myaccount.google.com/security).
3. Open [App Passwords](https://myaccount.google.com/apppasswords).
4. Create an app password named `GoshopX Notification`.
5. Copy the 16-character value once; it is `GMAIL_PASSWORD`.

If the account belongs to Google Workspace and App Passwords are unavailable, an administrator may have disabled the feature. Use a transactional SMTP provider or Gmail API OAuth/service account instead of a normal password.

## 3. Configure `.env`

```dotenv
GOOGLE_CLIENT_ID=1234567890-xxxxx.apps.googleusercontent.com

GMAIL_SMTP_HOST=smtp.gmail.com
GMAIL_SMTP_PORT=587
GMAIL_USERNAME=notifications@example.com
GMAIL_PASSWORD=xxxx xxxx xxxx xxxx
GMAIL_FROM=notifications@example.com
```

`GOOGLE_CLIENT_ID` is a public identifier and is used when building the web client. `GMAIL_PASSWORD`, database passwords, and JWT secrets belong only in local `.env` files or a secret manager; never place them in `.env.example`.

## 4. Run locally

### Run Vite directly

Create `web/.env.local` without committing it:

```dotenv
VITE_GOOGLE_CLIENT_ID=1234567890-xxxxx.apps.googleusercontent.com
VITE_GRAPHQL_URL=http://localhost:8080/graphql
```

Start the backend/Compose stack and then start the web client:

```bash
docker compose config --quiet
docker compose up --build -d account notification graphql kong
cd web && npm run dev
```

### Run the Docker web image

The root `.env` uses `GOOGLE_CLIENT_ID`; Compose passes it as the `VITE_GOOGLE_CLIENT_ID` build argument to the web image:

```bash
docker compose config --quiet
docker compose up --build -d
```

## 5. Event recipient contract

Email notifications are sent only when event data contains either key:

```json
{"recipient_email":"customer@example.com"}
```

or:

```json
{"email":"customer@example.com"}
```

The producer must obtain the email from the service that owns the account or business context and include it in the event payload. `notification` does not read `account_db` directly. Events without an email still create normal in-app notifications.

## Troubleshooting

- `google login is not configured`: check `GOOGLE_CLIENT_ID` in `account`, then rebuild and recreate the container.
- Google `origin not allowed`: add the exact origin, including scheme and port, to the OAuth client.
- `invalid google identity`: the token is expired, has the wrong audience, has a non-Google issuer, or the email is not verified.
- Gmail `535 Authentication failed`: use a 16-character App Password rather than a normal Gmail password; verify 2-Step Verification and the username.
- In-app notification exists but email is absent: verify the event includes `recipient_email` or `email`, then inspect the notification container logs.
