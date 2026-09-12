# Runbook: a person asks to be removed

This is what to do when someone who booked a court asks for their data to be
deleted. It is written for whoever is on the other end of `hola@vibe.com.ar`,
and it assumes no context beyond a database connection.

It covers the **final client** — the person who booked, paid, and never had an
account. The venue owner is a different case and needs none of this: they have
a login and `DELETE /api/v1/auth/me`, which cascades their complexes, courts,
clients and bookings on its own.

## Why the client is a separate case

The client never signed up. Their name, phone and email are in the database
because a venue put them there — typed at the counter, or filled in on the
public booking form — and they have no login, no session and no endpoint of
their own. Until this runbook existed there was no path at all for "remove me",
and the answer would have been a manual `UPDATE` written under pressure by
whoever read the email.

## What is deleted, what is anonymized, and what is kept

Nothing is deleted. The client row stays and is **anonymized**: the person is
erased from it, the row itself survives so that everything pointing at it still
resolves.

**Erased**

| Where | What |
|---|---|
| `clients.first_name`, `last_name` | replaced with `Cliente Anonimizado` |
| `clients.phone` | replaced with `anonymized-<client id>` |
| `clients.email` | set to `NULL` |
| `clients.notes` | set to `NULL` — staff free text *about the person* ("prefiere la cancha 2, el marido es Juan"), which is about them even when it does not name them |
| `bookings.notes` | set to `NULL` on every booking of theirs — the same free text on the booking, and the only place a name or a phone number reaches a table that is not `clients` |

The phone is a derived placeholder rather than a blank or a shared constant,
and both halves of that matter. `clients.phone` is `NOT NULL` and unique per
complex, so a blank or a constant would refuse the second anonymization in the
same venue. And it deliberately does not look like a phone number: a value that
does can be dialled, messaged on WhatsApp, or matched by a later booking's
lookup against a real person who happens to be given that number.

**Kept, deliberately**

| Where | Why |
|---|---|
| `bookings` — every row, with its `client_id`, court, date and price | The venue's books have to still add up. Deleting a booking removes revenue from a report the owner is accountable for, and from an occupancy history they price against. The row now points at a client that names nobody. |
| `payments` — every row, with its amounts and its refund state | Money that moved is a record the venue and its accountant need, and the refund state machine has to still be able to answer for it. It is also not the requester's to erase: a transaction is about both parties. |
| `audit_log` — every entry | It is the record of who did what, including this operation. Erasing it would remove the evidence that the request was honoured. |

That split is the whole argument to give the requester if they ask for more:
**we can erase who you are; we cannot erase that a court was booked and paid
for.** Once the identity is gone, what remains is a court, an hour and an
amount, attached to nobody.

## Receiving the request

1. **Confirm it is them.** The request has to arrive from the email or phone on
   the client record, or the venue has to confirm it. A deletion request is a
   deletion request whoever sends it, and an unverified one is a way to erase
   somebody else's contact details from a venue they still book at.
2. **Find the client id.** The venue's owner can read it from the client's page
   in the app. Failing that, by phone:

   ```sql
   SELECT c.id, c.first_name, c.last_name, x.name AS complex
   FROM clients c JOIN complexes x ON x.id = c.complex_id
   WHERE c.phone = '+5491112345678';
   ```

   A person who has booked at two venues has **two client rows**, one per
   complex, and they are separate: each has to be anonymized on its own. Say so
   in the reply, and name the venues.
3. **Tell the venue.** The owner will see the client turn into
   `Cliente Anonimizado` and should hear why before they do.

## Running it

From `backend/`, with `DATABASE_URL` pointing at the database:

```bash
# Dry run first. This is the default: the work runs inside a transaction that
# is then rolled back, and the summary is what WOULD change.
go run ./cmd/anonymize -client=<client id>

# Read the summary. Check the complex is the right venue and the booking count
# looks like the person you are answering. Then:
go run ./cmd/anonymize -client=<client id> -apply
```

The output is the record to keep against the request:

```
client 5f2b… of complex 9a1c… anonymized
  erased:  name, phone, email, client notes
  erased:  notes on 3 booking(s)
  phone is now "anonymized-5f2b…"
  kept:    7 booking(s), 5 payment(s) — the venue's books and the refund state
  kept:    41 audit entry/entries for this complex — the record of who did what
```

Everything happens in one transaction, so a run that fails halfway leaves the
client either fully anonymized or untouched — never named in one table and
erased in another. Running it twice is safe: the second run finds nothing left
to erase and says so.

`-db-dsn` overrides `DATABASE_URL` if you need to point somewhere else. The
connection needs to read and write `clients` and `bookings` across tenants; the
command declares that scope itself, so no manual `SET` is needed.

## Replying

Tell them, in their own language, what was done:

- Your name, phone number and email address have been removed from the venue's
  records, along with any notes about you.
- The bookings and payments themselves remain, because the venue needs them for
  its accounts and its tax records. They no longer identify you.
- If you have booked at more than one venue, each one is handled separately —
  and name the ones that were.

## What is deliberately not automated

- **Deleting a booking.** It is never the answer to this request, and an
  operator asked for it should say no rather than run it: it silently changes
  revenue reports the owner is accountable for.
- **Bulk anonymization.** One client per run, on purpose. This is answering a
  named person, and a loop over a `SELECT` is how the wrong venue's clients get
  erased.
- **Backups.** Anonymizing the live database does not reach the snapshots
  Railway keeps. Those age out on their own retention; if a request needs them
  gone sooner, that is a conversation with the provider, not a command.
