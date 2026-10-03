# Roadmap

**English** · [Português](ROADMAP.pt-BR.md)

What is open in the app, with what is already known about each item. Curriculum does not
go here: prompts, answer keys and their audit live in the content repository.

Items are ordered by risk to players, not by effort.

## The gates do not force any path

The track has a ceiling and the gates are calibrated in the content repository. The Core's
`view()` unlocks a node **by XP only**, using the XP of the node's track (for the free
track, global XP minus the XP from paid tracks); prerequisites are used only to mark the
parent as conquered — today they do not actually lock anything.

Either the prerequisites start locking for real, or the gate numbers change. This goes
together with the timer calibration: both require playing the whole track.

## The question timer was never calibrated by playing

`seconds_for_template` in `match_engine.rs` gives 150s to DRY_RUN, 90s to SPOT_THE_BUG, 75s
to COMPLEXITY_MATCH and 60s to everything else, and a challenge can override it with
`content.seconds`.

The budgets were estimated, not measured with people. The blind review measured two
extremes at node 7: a DRY_RUN with 109 seconds to spare, and a COMPLEXITY_MATCH where
reading alone takes almost the whole budget for a slow reader. The numbers came from a
words-per-minute model, not from players — that is why this is listed here and was not
changed.

The per-language review (September 2026) strengthened the signal: by the content guide's
formula, seven items use more than half the timer on reading alone, in all three
languages, almost all of them COMPLEXITY_MATCH, where the options are scanned twice.
Spanish comes out longer and is at the limit in more cases. The prompts were already
trimmed where that was possible without changing the meaning; the rest needs cutting an
option or code, or giving the template more time. The list of items is in the content
repository.

Calibrating requires playing all seven tracks and noting where time is left over and where
it runs short.

## Nobody reads the end of a match

Every match with at least one answer closes with `MATCH_END`: one played to the end with
`solved`, an abandoned one with `solved` and `abandoned: true`. Entering and leaving
without answering generates no event. But nothing consumes this event yet — the backend
ignores it in `ProcessEventXP`, and there is no history screen or route. The post-match
report reads the in-memory state, not the event.

Once history exists, it is what decides what an abandoned match counts as: a loss,
neutral, or hidden. The flag is already recorded for all three readings.

## Paid tracks: what is left

PR #2 delivered the whole cycle on iOS and the Android client brought it to Google Play:
grid catalog, one track per tree, free sample, paywall at three points, purchase validated
by the server, restore, refunds (App Store notifications; Play's voided-purchases job), a
30-day offline license with the warning ladder, encrypted package and storage. Design and
threats in ADR 0013; screens in the `LogN Trilhas` and `LogN Validade Offline` designs.
PRD: [`specs/logn_trilhas_pagas_spec.md`](specs/logn_trilhas_pagas_spec.md).

The two tracks are on sale on Play, each with a first-purchase offer. Still open:

- **Offline validity ladder on screen.** Purchase and refund already ran end to end on
  Play, but the ladder has not been seen: the server issues the license with the current
  time, so only the Core tests cover the 27 days.
- **Terms and policy.** v4 is live (2026-09-30), store-neutral: Google as a processor,
  refunds through Google Play and the waitlist section. It shipped without a lawyer's
  review, and these stay open for one: the legal basis for acceptance, acceptances deleted
  on account deletion, teenagers, the XP gates, the retention of access logs, and the
  leaderboard showing every player with no way to opt out (objection by email). With
  acceptances on record, a correction is a new version.
  The text promises something the code does not do yet: delete `store_transactions`,
  `revoked_transactions` and `manual_revocations` 5 years after the transaction (the
  first one comes due in 2031). It is not for now, but it is not to be dropped either: the
  policy says it deletes them.
- **Manual revocation.** `just revoke` and `just appeal` (ADR 0021) revoke the license and
  answer the appeal through the internal route, with the evidence recorded and the email
  notice from section 10.5. They have never been run against production.
- **GitHub sign-in.** Live, with the PostHog flag `sso_github_enabled` on since
  2026-09-30. Still to check: account deletion confirmed by GitHub, which revokes the
  app's authorization there.

Decided and open, each waiting for its moment:

- **Leveler (node zero).** It is in the design and gets its own PR, with a PRD: it is a new
  content system, with video, text, figures, references and the reminder after two
  mistakes.
- **One key per track version.** One buyer's key opens that version's package for anyone.
  This is the threat the spec leaves out of scope; the remedy is to bump
  `content_version`.
- **Device limit.** Today it only records devices. A limit, if one comes, comes after
  measuring how many devices a legitimate account uses.
- **Selector in the header.** The app opens the catalog from the track name with a chevron;
  the design has a separate "Trilhas" (Tracks) button. The design file needs to reflect
  the choice.
- **Spanish.** Content in three languages (ADR 0009) is live in Portuguese, English and
  Spanish. Spanish went through a copy review, but still needs a native technical
  reviewer.

## Educational institution on the profile

The design ("LogN Instituicao") puts the institution on the Perfil (Profile) screen, below
the numbers, as the key for the per-institution leaderboard and for contest
registrations. It is the first step of the two sections below, and nothing of it exists
yet: the Sede (Home base) tab says "UFC" for everyone.

This delivery is choosing from the e-MEC list, on the Profile screen or in a skippable
sign-up step, seeing the acronym on the Profile screen, and proving the affiliation with
a code sent to the institutional email, which does not become a login and is stored
encrypted. e-MEC does not provide email domains: they get curated from the people who try
to verify. It already shows where players come from, which is the data for deciding where
the leaderboard starts. The cooldown for switching and the card's metrics wait for the
leaderboard PRD, because they depend on what it counts; the Sede tab stays as it is until
then.

Before coding: the screen for the sign-up step, which the design does not have, and the
new key `INSTITUTION_EMAIL_KEY` in Secret Manager. The privacy policy gets a new version,
again without legal review.

PRD: [`specs/logn_instituicao_spec.md`](specs/logn_instituicao_spec.md).

## The global leaderboard is built, not deployed

Free-track XP, all-time, opening at 10 players. Everyone shows as "jogador #N" (player #N)
until they pick a permanent nickname on the Profile screen; moderation and objections go
through `just leaderboard-*` (anonymize, hide, unhide). The backend, the Core and both
clients are done and were checked against the "LogN — Placar geral de XP" canvas, on an
Android device and the iOS simulator. The scoreboard and the Sede tab left the screen;
their types and mock stay in the Core for contests.

What is left, in this order:

- **v5 of the policy** in the content repository, non-material, with the notice banner:
  the leaderboard shows number or nickname, XP and position to other players, on
  legitimate interest, with objection by email; a moderated nickname is kept after
  account deletion.
- **Deploy 1:** migration 0070 with the code that draws numbers and keeps free XP. Before
  it, run 0070's guard as a SELECT against production: the migration aborts if global XP
  minus paid-track XP does not match the free-track challenges.
- **Deploy 2:** migration 0071 (recompute and NOT NULL) with the routes and v5.
- **Android release** with the new screens.

PRD: [`specs/logn_placar_spec.md`](specs/logn_placar_spec.md).

The per-institution leaderboard still has no PRD: who counts (student, teacher, alumni),
the institution's switching cooldown and the profile card's metrics ("at the
institution") come with it. Only a verified affiliation counts there.

## Contests do not exist yet

The match already speaks the language of a contest: problems by letter, balloons, the
clock, `CONTEST ENCERRADO` (CONTEST OVER) in the report, and `isFrozen` for the last hour in
the design system. But each match is played alone, offline. There is no contest with
other people: no schedule, registration, shared problem set, or live scoreboard.

The institution spec already sets two rules for it: only a verified affiliation counts in
a registration, and switching institution is never allowed while a contest is running.
The rest needs its own PRD, after the leaderboard's, since the contest scoreboard is the
same screen with real data. A contest is the first feature that needs to be online at a
fixed time, which goes against the offline-first design (ADR 0002): the PRD has to say
what an answer given offline during a contest counts for.

## The iPhone

The public launch moved to Google Play (ADR 0022), live since 2026-10-01. The iPhone comes
in when there are 100 confirmed people on the waitlist or Android revenue that pays for
the Apple Developer account, whichever comes first. The landing page's waitlist is open.

Until then iOS runs only from Xcode, and everything that needs the Apple account waits:

- **Store build.** `just release-ios` packages the track and the documents from production
  and builds the `.ipa`, without uploading it; it regenerates the seed and the documents
  at that moment.
- **App Store Connect.** Paid apps agreement, banking and tax details, and the
  non-consumable product for each track, without Family Sharing.
- **App Store notifications.** The backend is live; the notifications URL still has to be
  registered, and it is the `.run.app` one, not the domain behind Cloudflare.
- **Sign in with Apple.** Dark until the paid account (ADR 0017).
- **End-to-end purchase.** Store purchases only run through Xcode, with the StoreKit
  Testing certificate in `APPLE_XCODE_ROOT_CERT`, and have not been exercised in full yet.
- **Refund reconciliation.** A refund whose notification exhausts Apple's retries never
  arrives. Reconciliation through the App Store Server API is still missing.
- **Sandbox in production.** Whoever tests through TestFlight gets the track for real. If
  that becomes a problem, the way out is to treat Sandbox entitlements as temporary.
- **Chain verified at the current time.** Restoring a JWS whose leaf has expired fails
  closed; Apple verifies at `signedDate`.

## The FFI numbers variants by position

Bincode does not write the variant name, only its position in the enum; the code generated by `codegen` writes that number. Removing or reordering a variant in the middle of a type that crosses the FFI would silently change the protocol if there were a version mismatch.

**Why reordering is safe today:** on each platform both sides ship in the same binary. iOS generates and compiles them together with `build-ios-ffi`; Android with `just android/generate`, and Gradle's `verifyGenerated` refuses the build if the Core changed after the last generate (ADR 0023). What goes to disk to be read later is JSON, not bincode.

**Stability rule (JSON):** in JSON, serialization is based on the name. So you cannot rename a variant or a field of persisted types (`OfflineSnapshot`, `SkillNode`, `Challenge`, `GameEvent`, `NodeStatus` and the stored leaderboard, `LeaderboardCache`). If you need a new field, use `#[serde(default)]`.

**Situations that would require changing this decision (they would require "enums only grow at the end" and hard guards):**
- If some state starts being persisted in bincode.
- If the Core starts being distributed with its own version and the two ends can fall out of date.
- An extension (e.g. a widget) or a watch exchanging bincode with the app.
