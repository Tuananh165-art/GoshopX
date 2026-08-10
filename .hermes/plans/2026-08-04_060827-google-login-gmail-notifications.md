# Google Login and Gmail Notification Implementation Plan

> **For Hermes:** Implement this plan task-by-task with focused verification.

**Goal:** Add Google Sign-In to the existing GraphQL authentication flow and send user-facing notification emails through Gmail SMTP while preserving the existing in-app notification feed.

**Architecture:** The web client uses Google Identity Services to obtain a Google ID token. GraphQL forwards that token through the account gRPC service, where the backend validates issuer, audience, email verification, and account status before issuing the existing HttpOnly JWT cookie. Notification continues consuming Kafka events and persists in-app notifications; after persistence it sends best-effort email using Gmail SMTP. Email recipient is carried in event data by the producer boundary or passed through a resolver-owned account lookup where available; notification never reads the account database directly.

**Tech Stack:** Go 1.24, gRPC/protobuf, GraphQL/gqlgen, PostgreSQL/GORM, Kafka/Sarama, React/Vite, Google Identity Services, `google.golang.org/api/idtoken`, Gmail SMTP with an App Password.

## Acceptance Criteria

- A user can sign in with Google from `/login`; backend rejects invalid audience, issuer, expired, or unverified-email ID tokens.
- Existing password login/register/logout behavior remains unchanged.
- A Google user is created once by verified email and subsequent login is idempotent.
- Existing notification rows are still created idempotently from Kafka events.
- When SMTP is configured, supported notification events send one email to the event recipient; SMTP failure is logged and does not fail the business event consumer.
- When SMTP is not configured, the service still starts and in-app notifications still work.
- `.env.example`, README, and a setup guide document Google Cloud OAuth client configuration and Gmail App Password setup without committing secrets.
- Focused tests, `go test`, GraphQL generation, TypeScript build, and `docker compose config --quiet` pass.

## Tasks

1. Add Google identity fields and account-service Google login contract.
2. Implement backend ID-token validation, account upsert, and JWT issuance.
3. Expose `loginWithGoogle` through GraphQL and add the web Google button.
4. Add Gmail SMTP email sender behind configuration and integrate it into notification consumption after persistence.
5. Propagate recipient email in user-scoped event payloads where needed.
6. Update Compose, `.env.example`, docs, and setup commands.
7. Regenerate protobuf/GraphQL artifacts and run focused/full verification.

## Required external setup

- Google Cloud project, OAuth 2.0 Web client ID, authorized JavaScript origin (`http://localhost:5173` for Vite and production origin later).
- Enable the Google Identity Services client; backend only needs the client ID as audience.
- Gmail sender account with 2-Step Verification enabled and a 16-character App Password. `GMAIL_PASSWORD` is a secret and must stay only in `.env`/runtime secret storage.
- SMTP defaults: `smtp.gmail.com:587`, STARTTLS, sender address equal to the Gmail account.

## Risks and decisions

- Gmail App Password is not an API key. Gmail API OAuth/service-account delivery is a separate option and is not required for transactional email v1.
- Email delivery is best-effort and must not block order/payment state transitions; operational retries can be added later with an outbox.
- Existing working-tree changes are pre-existing and must not be reset or overwritten.
