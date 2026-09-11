# Booking Link Credential Specification

## Purpose

Three public routes — `GET /api/v1/book/status`, `GET /api/v1/book/cancel-info`,
`POST /api/v1/book/cancel` (`internal/bookings/bookings.go:222-224`) — let an
unauthenticated client read or cancel their own booking. Today the only input
they require is `booking_id`, and `GetByID` (`internal/data/bookings.go:208-227`,
`WHERE b.id = $1 LIMIT 1`) is the entire authorization. The booking's primary
key is doing double duty as a bearer credential — why `cmd/api/sentry.go`'s
scrubber list (`:76,83,88,96-99,106,113,120`) has no rule for a bare UUID: a
booking id is correctly not a secret anywhere else it is used.

This capability pins what authorizes those three routes, how long that
authorization lives, what an expired holder is told, and what the public
response bodies may return. It does not choose the token's algorithm, storage
shape, or exact buffer duration — those are design's.

## Requirements

### Requirement: The booking's primary key authorizes nothing on the public routes

`GET /api/v1/book/status`, `GET /api/v1/book/cancel-info`, and
`POST /api/v1/book/cancel` MUST NOT accept, resolve, or authorize on
`booking_id`. Each route MUST resolve the booking exclusively through a
separate access token. A `booking_id` present in the request (query string or
body) MUST NOT be used to look up or act on any booking.

#### Scenario: A syntactically valid booking id alone is refused

- GIVEN a caller knows a real booking's UUID but holds no token for it
- WHEN it calls any of the three routes passing only `booking_id`
- THEN the request is refused and no booking data or cancellation happens
- AND the response does not distinguish this from an unknown identifier

### Requirement: The access token is opaque, expiring, and hashed at rest

The system MUST mint a single-purpose access token per booking, unguessable
by inspection (not derived from the booking id or any other public value),
MUST store only its hash, and MUST reject a presented token whose hash is not
found or whose stored expiry has passed. The token MUST be minted no later
than the booking's own insert commits, in the same transaction, so a crash
between the two cannot leave a booking with no way to reach its own routes.

#### Scenario: A booking always has a usable token immediately after creation

- GIVEN a booking is inserted — staff with immediate payment
  (`internal/bookings/create.go:262-263`) or the MercadoPago webhook
  confirming a public booking (`internal/payments/process.go:222-223`)
- WHEN the insert transaction commits
- THEN a token for that booking already exists and resolves to it

#### Scenario: A guessed or fabricated token is rejected

- GIVEN a value never minted by this system
- WHEN it is presented to any of the three routes as the access token
- THEN the request is refused as if the token were unknown

### Requirement: Token expiry can never block a refund-eligible cancellation

The token's expiry MUST be set no earlier than the booking's end time. This is
a structural invariant, not a tuned number: both deadlines `CanRefund`
evaluates (`internal/pricing/refund.go:44` grace-from-creation, `:54`
`cancellationHours`-before-start) fall strictly before the booking's start,
hence strictly before its end. A future change to either `CanRefund` deadline
or to the expiry offset MUST preserve `token_expiry >= booking_end`.

#### Scenario: A token is still valid at both refund deadlines

- GIVEN a booking whose token expiry derives from its end time
- WHEN evaluated at the grace-period deadline and separately at the
  cancellation-hours deadline (`CanRefund`'s two branches)
- THEN the token has not expired at either, since both fall strictly before
  the booking's end time

#### Scenario: A cancellation eligible for refund is never rejected as expired-token

- GIVEN a booking is still before its `CanRefund` deadline
- WHEN its holder presents the token to `POST /api/v1/book/cancel`
- THEN the request is not refused for token expiry

### Requirement: An expired token answers 410, distinguishably from an unknown one

A token resolving to a real, previously-minted token whose expiry has passed
MUST receive `410 Gone` on all three routes, with a body stating the client's
recourse. A token matching no minted token, malformed, or absent MUST keep
each route's current shape: `404` for the two GET routes, `400`/`422` for
missing or malformed input on the POST route. The `410` case is reachable
only by a caller already holding a syntactically valid, previously-issued
token, so distinguishing it from `404` reveals nothing to anyone else.

#### Scenario: An expired token gets a distinguishable, actionable response

- GIVEN a token was minted for a real booking and its expiry has passed
- WHEN presented to any of the three routes
- THEN the response is `410 Gone`
- AND the response body states what the client can do next

#### Scenario: An unknown token keeps today's response

- GIVEN a token never minted, malformed, or absent
- WHEN presented to any of the three routes
- THEN the response is the route's existing `404` or `400`/`422` — never `410`

#### Scenario: A cancelled booking's token still answers status

- GIVEN a booking was already cancelled through `PublicCancel`
- WHEN its still-unexpired token is presented to `GET /api/v1/book/status`
- THEN the current status and payment status are returned, not `410`/`404`

### Requirement: Public response bodies never return the booking's primary key

`PublicBook`, `PublicStatus`, `PublicCancelInfo`, and `PublicCancel`
(`internal/bookings/public.go:304,409,468,633`) MUST NOT include the
booking's UUID in any response body. `PublicBook` currently returns the
entire booking struct (`:304`); the other three return `"id": booking.ID`
explicitly (`:409,468,633`). This is a public response contract change: a
frontend consumer reading the booking id from these bodies must be updated
before this lands, since the field will no longer be present.

#### Scenario: A successful public booking response carries no booking id

- GIVEN a client completes `POST /api/v1/book`
- WHEN the response is returned
- THEN no field in the body is the booking's primary key

#### Scenario: Status, cancel-info, and cancel responses carry no booking id

- GIVEN a valid, unexpired token is presented to `PublicStatus`,
  `PublicCancelInfo`, or `PublicCancel`
- WHEN each returns its success response
- THEN no field in any of the three bodies is the booking's primary key

### Requirement: The token's query parameter is named exactly `token`

Wherever the access token is presented as a URL query parameter, the name
MUST be exactly `token`. `cmd/api/sentry.go`'s "named secret" rule (`:96-99`)
already redacts `?token=<value>` in captured URLs and query strings
(`scrubRequest`, `:188-189`); its own comment records why: `\b` cannot match
`token` inside `booking_token`, because an underscore is a word character.
Naming the parameter anything else — including `booking_token` — silently
loses this redaction with no visible failure at the call site. Only a
regression test pins this; a comment would not survive a refactor.

#### Scenario: The token query parameter is redacted by the existing named-secret rule

- GIVEN a captured URL `.../api/v1/book/status?token=<64-char-value>`
- WHEN passed through `scrubText` (applied to `request.URL` and
  `request.QueryString`)
- THEN the value after `token=` is replaced with the redaction placeholder

#### Scenario: A differently-named token parameter would not be redacted

- GIVEN a captured URL `.../api/v1/book/status?booking_token=<64-char-value>`
- WHEN passed through `scrubText`
- THEN the value after `booking_token=` is NOT redacted — proving why the
  parameter must be named `token`, not a longer variant

### Requirement: A resolved booking id never re-enters an error report on these routes

Once a token resolves to a booking on any of the three routes, no error path
on that route (`ServerError`, a captured message, or a breadcrumb) may put
the booking's UUID into a value that reaches Sentry. An error raised on any
of the three routes MUST produce a Sentry event containing neither the
booking's UUID nor the token's plaintext.

#### Scenario: A server error after resolution does not leak the booking id to Sentry

- GIVEN a token has resolved to a booking on any of the three routes
- WHEN a later step on that request raises a captured error
- THEN the captured event's message, exception values, URL, and query
  string contain neither the booking's UUID nor the token's plaintext

## Out of Scope

- Rate limiting the three routes beyond what already applies.
- A Sentry rule matching bare UUIDs generally — rejected, it would redact
  every legitimate diagnostic booking id elsewhere in captured errors.
- `cmd/api/cron.go:185`'s captured message, on a path that never sees a
  token; harmless once the id stops being a credential anywhere else.
- Dual-accept windows, migration, or backfill. Nothing is deployed.
- A self-service token reissue path — would be a new capability.
