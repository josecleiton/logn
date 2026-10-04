# ADR 0022: Android first for the public launch, and iPhone through a waitlist

**English** · [Português](0022-android-primeiro-no-lancamento.pt-BR.md)

## 1. Overview

The plan so far was to launch on iPhone and bring Android later (ADR 0001, paid tracks spec). The spec put "Android and Play Billing" out of scope, and purchasing, the landing page and the legal documents were written for the App Store only.

That plan assumed the paid Apple Developer Program account, which costs US$ 99 per year, and that account was never created. Without it there is no TestFlight, App Store, Sign in with Apple or APNs. Purchases exist only in the local `.storekit`, and the Xcode signature expires every 7 days. Documents that say the app "was on TestFlight" (ADRs 0008, 0013 and 0020) describe a distribution that never happened: the app only ever ran on the owner's device, through Xcode.

The Google Play Console account already exists and is an organization account. Because it is an organization account, the closed-testing rule of 12 testers for 14 days, which applies to new personal accounts, does not apply. The fee is a one-time US$ 25.

## 2. Decision

**The public launch is on Google Play.** The reason is cost: not paying US$ 99 per year to validate the idea. iOS stays in the repository and runs on the owner's device. The Apple code (StoreKit, Sign in with Apple, JWS validation) stays as it is, unused in production, and none of it is deleted.

**The Apple account comes in when one of two triggers fires**, whichever comes first:

- 100 **confirmed** sign-ups on the iPhone waitlist;
- Android revenue that covers US$ 99 per year.

**The landing page** (`landing/README.md`):

- The buttons point to Google Play (`LOGN_PLAY_STORE_URL`). The App Store is optional (`LOGN_APP_STORE_URL`) and turned off. Each badge only appears when the store has a link.
- The copy says "Android · free · iPhone coming soon".
- **Waitlist for iPhone only**, because Android ships first. It is a `<form>` without JS that posts to `POST /api/v1/waitlist`, and the backend replies with `303` to `/waitlist/{thanks,confirmed,left,error}/`. The CSP `form-action` allows the API domain and no other. The form only appears with `LOGN_API_ORIGIN`, and that is only set after the route exists and the privacy policy covers the list.
- `/account/delete/` is the deletion URL the Google Play listing requires under "Data safety". One URL serves every language, and the page has the language selector. It repeats section 10 of the policy: through the app, immediately; without the app, through `contact@logn.sh` from the registered e-mail address.

**The waitlist, in the backend:**

- Table `waitlist_entries` (`email`, `locale`, `confirmed_at`, `created_at`, `updated_at` with trigger), in Postgres. It does not live in the worker or with a third party, so personal data stays in the same place and under the same purge.
- Double opt-in: a link with an HMAC token, and unconfirmed entries are purged 7 days after creation. The response is the same for a new, pending and confirmed e-mail, so the route does not reveal who is on the list.
- **A pending entry receives one e-mail only.** Asking again does not resend, unless SMTP failed. With resending, someone submitting the form every day would deliver one e-mail per day to an address that is not theirs, and the row would never expire. Someone who comes back after the pending entry expires signs up again, so the worst case is one e-mail every 7 days per address.
- Against abuse:
  - Only a form from the landing page itself is accepted: `Origin` equal to the landing page, or `Sec-Fetch-Site: same-site`. The landing page is served with `no-referrer`, which makes the browser send `Origin: null`. Without this check, any page could post the form through each visitor's browser, one IP per visitor.
  - Per-IP rate limit, `limitBody`, and a global cap of 100 confirmation e-mails per hour, counted in the database.
  - The `website` honeypot field, which bots fill in and people do not see.
  - No Turnstile or captcha: it would be a third-party script, and it only comes in if spam shows up, with its own ADR.
- **The e-mail link changes nothing on GET.** Corporate e-mail filters open every link they receive. A GET that confirmed would sign up people who did not ask, and one that removed from the list would remove people who did. `GET /api/v1/waitlist/{confirm,leave}?t=` shows a page with a button, with hash-based CSP and `Referrer-Policy: no-referrer`, because the token is in the URL. The button sends the `POST` to the same URL, which replies with `303` to the landing page.
- **The token is `id.HMAC(id, action)`**, with the JWT key and its own prefix. The e-mail address never goes in the link, because the URL passes through the hosting logs, and those do not store e-mail addresses. The token does not expire on its own: it dies with the row. Each action has its own signature, so the confirm link does not remove anyone from the list.
- Every e-mail carries the leave link, also in `List-Unsubscribe`, and leaving deletes the row.
- We do not send `List-Unsubscribe-Post` (RFC 8058): one-click unsubscribe is a POST from the mail provider's server, and the edge's Bot Fight Mode challenges servers with JS, with no exceptions (ADR 0013). Unsubscribing would fail silently. The header comes back with a path that does not go through the challenge, and with DKIM signing both headers.
- The confirmation e-mail is sent outside the request, like the OTP one, so response time does not tell who is already on the list. If SMTP fails, the goroutine allows a resend. The error is logged without the e-mail address, because SMTP often repeats the recipient in the rejection.
- `WAITLIST_LANDING_ORIGIN` and `WAITLIST_API_ORIGIN` turn the routes on (`enable_waitlist` in Terraform). Without both, the routes do not exist. Only one, or either one outside the `https://domain` format, and the server does not start.
- A confirmed e-mail is kept until the iPhone launch, with no fixed deadline. The policy says so.
- A single notice, when the iPhone version ships. Sending that notice is deferred until the trigger fires.

**Purchases through Google Play**, alongside the App Store:

- **One product id in both stores.** The current ids are valid on Play. A new migration renames `tracks.app_store_product_id` to `store_product_id`, and `logn-conteudo` follows.
- **`entitlements` gains `provider`**, and the active-owner index becomes `(provider, original_transaction_id)`. The `environment` CHECK in `store_transactions` starts accepting the Play environments. The other purchase and revocation tables already accept `google_play` (migrations 0050 and 0063).
- **Play's `original_transaction_id` is the hex SHA-256 of the `purchaseToken`.** The token has no documented maximum length, and `orderId` is not present on license tester purchases. The raw token goes to `raw_payload` and to the API call.
- **Server-side verification, through the Google Play Developer API**:
  - `purchaseState` must be purchased; pending does not unlock.
  - `obfuscatedAccountId` must be the `user_id`, with the same rule as `appAccountToken` in ADR 0013 (on restore, only from an account that no longer exists).
  - The server acknowledges right after recording the entitlement. Without an acknowledgement within 3 days, Play refunds on its own.
- **A license tester purchase unlocks the track**, like a Sandbox one in ADR 0013. The difference is that only the accounts we register in the Play Console can make it.
- **Keyless credential.** The Cloud Run service account is invited into the Play Console, with permission only to view financial data and manage orders. The token comes from the metadata server, with the `androidpublisher` scope, and the call is REST over `net/http`, with no SDK, as in ADR 0016. No new secret and no new dependency.
- **The purchase arrives through the same route.** `POST /api/v1/purchases` and `/restore` accept `{"jws"}`, as always, or `{"provider":"google_play","product_id","purchase_token"}`. Before any request to the store, the product is checked in the database (`IsPaidProduct`) and the token goes through a closed format, because both go into the API URL path. There are two new codes: `purchase_pending` (409), for a purchase not yet paid, and `store_unavailable` (503), for Play turned off. A store or acknowledgement failure returns 502, and the app sends again: recording the entitlement is idempotent.
- **Refunds through a daily poll.** A Cloud Scheduler job calls `POST /api/v1/internal/play/voided`, which reads the Voided Purchases API (29 days back) and revokes through the same `revoked_transactions` path. A void for fraud or chargeback (`voidedReason` 5, 6 and 7) becomes `fraud`; the rest becomes `refund`. The route checks issuer, audience and the signing account. The account is the Scheduler's, the same as the purge's: the same job calls both, and both only read the store and revoke, with nothing to return to the caller. *Update (ADR 0026): the separate route and job are gone; the poll runs inside the daily purge (`POST /api/v1/internal/purge`), to free a Scheduler slot on the free tier.*
- **The provider now lives on the entitlement** (`entitlements.provider`, 0066). `GrantEntitlement` uses the purchase's provider, and manual revocation reads the provider from the entitlement instead of the purchase record. This closes the open item from ADR 0021. `RevokeTransaction` and `ReinstateRefund` only touch the entitlement from the same store.

**Sign-in on Android:**

- Google through Credential Manager. The ID token comes back with `aud` = web client (the `serverClientId`) and `azp` = Android client. Accepting only the web audience would let through a token requested by any client in the project, so the backend accepts **pairs** `(aud, azp)`: `(iOS, iOS)` with `GOOGLE_IOS_CLIENT_ID`, and `(web, Android)` with `GOOGLE_WEB_CLIENT_ID` and `GOOGLE_ANDROID_CLIENT_IDS`, which come together, or the server does not start. There is one Android client per key that signs the app: the debug key, the upload key (`~/.android/keystores/logn-upload.jks`, outside the repository) and the Play App Signing key, which Google generates and the Play Console shows after the first upload. All of them sit under the same web client, and `azp` must be one of them. Each audience goes through full validation; nothing in the token is read before the signature checks out. A missing `azp` is only accepted for the iOS pair.
- GitHub works as it is.
- No "Sign in with Apple": on Android it requires a Services ID, which requires the paid account. The `logn-apple-signin-key` secret stays in Terraform, reserved.

**Legal documents:** v4 is replaced in place (`gen_documentos_legais.py --substitui`), because only the owner has accepted it. The text becomes store-neutral, with Google as the payment processor, refunds through Google Play, and the waitlist section.

## 3. Rejected alternatives

- **Pay for the Apple account and launch in both stores.** Rejected on cost before the idea proves itself.
- **Waitlist for both platforms.** Android ships first, and the list would only matter for launch day.
- **List in the worker (D1/KV) or in a third-party service.** It would take personal data out of Postgres, out of the purge and out of the current policy.
- **`fetch` with JS in the form.** It would open `connect-src` and keep a script around for something HTML already handles.
- **Play Real-time Developer Notifications (RTDN) through Pub/Sub.** It would be a new public route, with a topic and a subscription, to gain less than 24 h on a track revocation.
- **Service account JSON key.** It would be one more secret, at the free tier limit, when the Cloud Run account is already enough.
- **Separate `play_product_id` column.** Both catalogs are ours, and a single id removes the need for a mapping.

## 4. Consequences

- **The store says which product the purchase is for.** The product id goes in the query path, and the store rejects a token from another product. But the documentation does not promise this, and a token for a cheap item from the same app must not unlock a track. When the response carries `productId`, it must be the one requested, and the quantity must be one.
- **A failed acknowledgement does not become a silent refund.** The entitlement is recorded before the acknowledgement. If it fails and the app does not send again, the daily voided job acknowledges purchases from the last 4 days that are still unacknowledged. An acknowledgement that failed but reached the store counts as done.
- **Our error is not an invalid purchase.** Only the response for a token that is not valid (410, or 400 and 404 with the token reason) becomes `purchase_invalid`. Wrong package, account without permission or API turned off return 502, and the app retries. API errors are logged without the URL, which carries the token.
- **A purchase from a promo code redeemed in the store** arrives with no account inside, and only comes in through restore. The Android client must call restore on launch.
- **Limits of the voided job:** if it goes more than 29 days without running, earlier refunds are lost, because the API keeps 30 days. More than 20 thousand voided purchases in the window make the job fail on every run. A Scheduler failure alert is missing.
- **Deploy order for 0066:** it renames `app_store_product_id`, and the old Cloud Run revision reads the old name. Between the migration and the traffic switch, `/tracks` and `/purchases` return 500. With no users, this is accepted. `gen_conteudo.py` and `tracks.json` in `logn-conteudo` change together, or `content-check` and the next content migration break.
- **The Android client depends on the Play Billing Library** (`com.android.billingclient:billing`, 9.1.0 in the `android/` skeleton). It is its first dependency, and it is not optional. The Play Console decides from its version whether the app can sell, and a hand-written `BILLING` permission, without the library, counts as the old AIDL API and is rejected (the floor is 8.0). It moves up in version with whatever the Console requires.
- **The Core still sends only `{"jws"}`.** `SubmitPurchase` in `app.rs` has to gain the Play shape once the Android client exists, and the Core needs to map `purchase_pending` and `store_unavailable` to `StatusKey`.
- **No AAB, no end-to-end test.** The Play Console only creates products after receiving an AAB with the billing permission, and a `purchaseToken` only comes from a real client. Until then, Play verification is tested against responses built from the documentation.
- The `/account/delete/` page and section 10 of the policy say the same thing. Whoever changes one changes the other. The Android app must use the same labels, "Gerenciar conta → Excluir minha conta" (Manage account → Delete my account).
- `LOGN_API_ORIGIN` on the landing page only after the route and the policy are live, in that order.
- **Accepted limits of the list:**
  - The token is in the URL, so it passes through the edge and Cloud Run logs. It carries no personal data and only confirms or removes that one sign-up.
  - Rotating the JWT key kills every link already sent, including the leave link. Anyone confirmed leaves through `contact@logn.sh`.
  - With `cpu_idle`, the sending goroutine may get no CPU after the response, like the OTP one. If sending dies there, the pending entry has no e-mail until it expires.
  - A browser without `Sec-Fetch-Site` that still sends `Origin: null` cannot sign up.
  - Before turning it on, check the form and the e-mail links going through the real edge, with Bot Fight Mode on.
- **The list may sit idle for years.** The "until launch" retention was chosen knowing that. If the trigger never fires, deleting the list is a decision to revisit here.
- The Android client, push (FCM on Android, APNs on iOS once there is an account), `assetlinks.json` and sending the launch notice are outside this decision.
- ADR 0001 and the paid tracks spec remain valid for what they describe of the architecture. The launch order is now this one.
