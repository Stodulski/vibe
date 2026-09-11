# WhatsApp Templates

The four templates Vibe sends, exactly as they must exist in WhatsApp
Manager.

**The text in this document is the source of truth.** The Go builders in
`internal/whatsapp` send positional parameters that fit into these
sentences. Meta matches `{{1}}`, `{{2}}`, … by position and does not warn
when they do not match: a template with a different parameter count or
order comes out as `Cancha: 15/03`, with a 200 OK and no error anywhere.

**Changing the parameter count or order requires a code change.** You have
to edit the builder in `internal/whatsapp/whatsapp.go` and its test in
`internal/whatsapp/whatsapp_test.go` in the same commit as the template.

All templates are:

- **Category**: UTILITY. They are transactional. Meta rejects a UTILITY
  template that sounds like marketing, and approves faster the more literal
  the text is. That is why none of them have emojis, exclamation marks, or
  invitations to come back.
- **Language**: `es_AR`. It has to be exactly that code. A template created
  as `es` fails on send with error 132001.

## Text rules

- One idea per line. No emojis or exclamation marks.
- Vocabulary: **seña** for the deposit, **devolución** for money that comes
  back (never "reembolso"), **complejo** for the venue (never "club"),
  **link** for a URL (never "enlace").
- There cannot be two `{{n}}` in a row, and the body cannot start or end
  with one. Meta's validator rejects it. The fixed words between variables
  exist to satisfy this. Do not delete them: this is why, for example, the
  deposit amount and the pending balance travel as two separate parameters
  instead of one sentence already assembled: "Seña abonada: {{5}}. Resta
  pagar en el complejo: {{6}}." has fixed words before, between, and after
  the two.
- Parameter **values** cannot have line breaks, tabs, or four or more
  spaces in a row. Meta rejects the whole message if any of them do. The
  code strips those in `SendTemplate` (`sanitizeParam`). The template's
  fixed text can have line breaks.

## Buttons

All buttons are dynamic "Visitar sitio web" buttons: a fixed base with a
variable at the end. The value the API sends is appended as-is, with no
encoding on Meta's side.

| Base URL | Buttons |
|---|---|
| `https://www.google.com/maps/search/?api=1&query=` | Ver ubicación, Cómo llegar |
| `https://app.vibe.com.ar/` | Cancelar reserva, Nueva reserva |

`https://app.vibe.com.ar/` represents the deploy's frontend origin, the
value of `FRONTEND_URL`. It has to end in a slash: the code sends the
suffix without a leading slash.

---

## 1. `booking_confirmation`

Sent when a booking gets confirmed: MercadoPago approves a public client's
payment, or staff loads a booking from the dashboard.

**Body**, 7 variables:

```
Su reserva está confirmada.

{{1}} en {{2}}, el {{3}} de {{4}}.

Seña abonada: {{5}}. Resta pagar en el complejo: {{6}}.
Cancelación {{7}}

Gracias por su reserva.
```

| # | Value | Example |
|---|---|---|
| 1 | court name | `Cancha 1` |
| 2 | complex name | `Vibe Palermo` |
| 3 | date, `dd/mm` | `15/03` |
| 4 | time, with `(día sig.)` if it ends the next day | `09:00 a 10:30` |
| 5 | amount paid as deposit, `$0` if nothing was charged | `$5.000` |
| 6 | balance pending at the complex, `$0` if everything is already paid | `$15.000` |
| 7 | cancellation rule | `con devolución hasta 24 horas antes del turno.` |

Parameters 5 and 6 come from `notifications.PaymentAmounts`, which no
longer builds a sentence: it returns the two amounts separately, because
the template cannot leave two variables next to each other and "nothing
pending" now has to be a number, not a phrase.

Parameter 7 is one of four (`notifications.CancellationLine`), in
lowercase because it follows the fixed word "Cancelación":

- `con devolución hasta 24 horas antes del turno.`
- `con devolución hasta el inicio del turno.` (complex without a configured window)
- `con devolución hasta 15 minutos después de la reserva.` (depending on the configured grace period)
- `sin devolución.` (no configured grace period)

**Buttons**:

| Index | Text | Base URL | Suffix the code sends |
|---|---|---|---|
| 0 | Ver ubicación | `https://www.google.com/maps/search/?api=1&query=` | `-34.603722,-58.381592`, or `name, address, city` URL-encoded if the complex has no coordinates |
| 1 | Cancelar reserva | `https://app.vibe.com.ar/` | `vibe-palermo/book/cancel?token=<token>` |

**Rendered example**

```
Su reserva está confirmada.

Cancha 1 en Vibe Palermo, el 15/03 de 09:00 a 10:30.

Seña abonada: $5.000. Resta pagar en el complejo: $15.000.
Cancelación con devolución hasta 24 horas antes del turno.

Gracias por su reserva.

[ Ver ubicación ]  [ Cancelar reserva ]
```

---

## 2. `reminder_2h`

The cron sends it about two hours before the slot starts.

**Body**, 6 variables:

```
Su reserva comienza en 2 horas.

{{1}} en {{2}}, el {{3}} de {{4}}.
La dirección es {{5}}.
Resta pagar en el complejo: {{6}}.

Le esperamos.
```

| # | Value | Example |
|---|---|---|
| 1 | court name | `Cancha 1` |
| 2 | complex name | `Vibe Palermo` |
| 3 | date, `dd/mm` | `15/03` |
| 4 | time | `09:00 a 10:30` |
| 5 | address and city | `Av. Santa Fe 1200, Buenos Aires` |
| 6 | balance pending at the complex, `$0` if everything is already paid | `$15.000` |

Parameter 6 is `notifications.BalanceAmount`: the pending amount, alone,
`$0` when nothing is left to pay. The fixed phrase "Resta pagar en el
complejo:" used to be part of the sentence the function returned. Now it
lives in the template. The body closes with "Le esperamos.", fixed text,
instead of ending on a variable: Meta's validator rejects a template that
ends in `{{n}}`, and a period stuck to the value does not count as a fixed
word.

**Buttons**:

| Index | Text | Base URL | Suffix the code sends |
|---|---|---|---|
| 0 | Cómo llegar | `https://www.google.com/maps/search/?api=1&query=` | `-34.603722,-58.381592`, or `name, address, city` URL-encoded |
| 1 | Cancelar reserva | `https://app.vibe.com.ar/` | `vibe-palermo/book/cancel?token=<token>` |

The cancellation token is generated by the reminder cron:
`booking_link_tokens` only stores a hash, so the one sent in the
confirmation cannot be recovered.

**Rendered example**

```
Su reserva comienza en 2 horas.

Cancha 1 en Vibe Palermo, el 15/03 de 09:00 a 10:30.
La dirección es Av. Santa Fe 1200, Buenos Aires.
Resta pagar en el complejo: $15.000.

Le esperamos.

[ Cómo llegar ]  [ Cancelar reserva ]
```

---

## 3. `booking_cancelled`

Sent through the three cancellation paths: the client cancels from the
public link, the owner cancels from the dashboard, or the
expired-payments sweep cancels an unpaid booking.

**Body**, 5 variables:

```
Su reserva fue cancelada.

{{1}} en {{2}}, el {{3}} de {{4}}.
Devolución: {{5}}

Ante cualquier consulta, contacte al complejo.
```

| # | Value | Example |
|---|---|---|
| 1 | court name | `Cancha 1` |
| 2 | complex name | `Vibe Palermo` |
| 3 | date, `dd/mm` | `15/03` |
| 4 | time | `09:00 a 10:30` |
| 5 | what happened to the deposit | `$5.000 enviados, se acreditan en los próximos días hábiles.` |

Parameter 5 is one of seven values, in lowercase because it follows the
fixed word "Devolución:". Six come from the cancellation results
(`internal/bookings/cancel.go`, `refundMessage`), with or without the
amount depending on whether there is one:

| Result | With amount | Without amount |
|---|---|---|
| no payment | none | `no hay pagos a devolver.` |
| past the deadline | `fuera de plazo, la seña de $5.000 no se devuelve.` | `fuera de plazo, la seña no se devuelve.` |
| issued | `$5.000 enviados, se acreditan en los próximos días hábiles.` | `enviada, se acredita en los próximos días hábiles.` |
| already issued | `$5.000, ya enviada.` | `ya enviada.` |
| in progress | `$5.000 en proceso, se notificará al completarse.` | `en proceso, se notificará al completarse.` |
| manual | `$5.000, a cargo del complejo de forma manual.` | `a cargo del complejo de forma manual.` |

A booking with an online deposit and a cash balance produces two of these,
joined into a single line with a space.

The seventh is the one from the expired-payments sweep
(`notifications.ExpiredUnpaidRefundLine`):

```
reserva vencida por falta de pago, no hay pagos a devolver.
```

**Button**:

| Index | Text | Base URL | Suffix the code sends |
|---|---|---|---|
| 0 | Nueva reserva | `https://app.vibe.com.ar/` | `vibe-palermo/book` |

**Rendered example**

```
Su reserva fue cancelada.

Cancha 1 en Vibe Palermo, el 15/03 de 09:00 a 10:30.
Devolución: $5.000 enviados, se acreditan en los próximos días hábiles.

Ante cualquier consulta, contacte al complejo.

[ Nueva reserva ]
```

---

## 4. `deposit_returned`

The name is not `deposit_refunded` because that template ended up loaded
in WhatsApp Manager with a different shape and Meta limits edits to one
per day. A deleted name stays blocked for 30 days. The queue's task type
(`wa:deposit_refunded`) does not change: it is a string persisted in
Redis.

Sent when a deposit actually comes back: MercadoPago reports a refund or
chargeback, an automatic cancellation refund completes, or a refund that
had failed completes on retry.

**Body**, 2 variables:

```
Se realizó la devolución de {{1}} de la seña de su reserva en {{2}}.

Se acredita en los próximos días hábiles, en el medio de pago utilizado.
```

| # | Value | Example |
|---|---|---|
| 1 | amount refunded | `$5.000` |
| 2 | complex name | `Vibe Palermo` |

The amount goes first because that is the order in which the approved
body ties them together: "la devolución de {{1}} ... en {{2}}."
MercadoPago's crediting timeframe is fixed text and not a parameter: it is
data about the provider, not about this refund, and fixed text cannot end
up empty by a caller's mistake.

**Button**:

| Index | Text | Base URL | Suffix the code sends |
|---|---|---|---|
| 0 | Nueva reserva | `https://app.vibe.com.ar/` | `vibe-palermo/book` |

**Rendered example**

```
Se realizó la devolución de $5.000 de la seña de su reserva en Vibe Palermo.

Se acredita en los próximos días hábiles, en el medio de pago utilizado.

[ Nueva reserva ]
```

---

## Recipient numbers

Meta routes Argentine mobiles with the `+54 9` format. The database
stores the E.164 the client typed, and `+54 11 5555 1234` is valid E.164
without the 9. `whatsapp.RecipientID` inserts it on send for `+54` numbers
whose national part has ten digits and does not start with 9. Every other
country passes through unchanged, including Brazil
(`+55 11 9 9999 9999`), where the 9 is part of the national number.
