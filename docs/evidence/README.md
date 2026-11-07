# GoshopX evidence screenshots

Screenshots and the capture procedure are documented in [Demo and Evidence Runbook](../demo-and-evidence-gcp.md).

## Current checked-in screenshots

| File | What it shows | What it does not prove |
|---|---|---|
| [Storefront](./screenshots/goshopx-storefront-2026-11-07.png) | GoshopX storefront/catalog UI was open on 2026-11-07. | Current public availability, login, search, checkout, or GKE resource state. |
| [Assistant widget](./screenshots/goshopx-assistant-ready-2026-11-07.png) | The assistant widget was visible and empty. | A successful model response, retrieval, image search, or provider/tool invocation. |

No GCP Console screenshots are checked in yet. Do not label screenshots produced from mock data, source diagrams, Terraform plans, or old browser sessions as live-service evidence. Add dated PNGs only after capture and review for personal data and credentials. Never commit `.env`, token-bearing network captures, state, kubeconfig, Secret Manager values, or customer data.

## Capture record template

Append rows to a private/demo evidence ledger or to the linked runbook after capture:

| Captured at (timezone) | Commit | Test/resource | Result | Screenshot | Notes / request ID |
|---|---|---|---|---|---|
| YYYY-MM-DD HH:mm | short SHA | e.g. GKE cluster / CHAT-01 | pass/fail/not run | `screenshots/name.png` | no credentials or personal data |
