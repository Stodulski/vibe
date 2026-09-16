-- +goose Up

SET LOCAL lock_timeout = '3s';

-- ============================================================================
-- THE WHOLE SCHEMA, ONCE.
--
-- This file replaces migrations 001 through 042 (031 and 039 were never used),
-- and then — a second squash, 2026-09-14 — the four migrations that had landed
-- on top of it: 002_user_identities, 003_optimistic_concurrency, 004_jobs and
-- 005_tenant_columns. It produces the schema those files produced when applied
-- in order — objects, ownership, grants and row-level security policies alike;
-- the proof is a pg_dump of a database migrated by the old chain diffed against
-- a pg_dump of one migrated by this file alone, with an empty result, plus a
-- catalog comparison of policies, grants, owners and default privileges. Both
-- squashes are recorded in docs/adr/0003-goose-migrations.md.
--
-- WHY A SQUASH, WHEN SECTION 12 OF THE SCHEMA GUIDELINES SAYS MIGRATIONS ARE
-- IMMUTABLE. The rule exists so two databases can never disagree about what
-- ran: once a file has been applied somewhere, editing it makes that
-- somewhere's history a lie. The product has never been deployed. There is no
-- production database, no staging database, and no customer row anywhere; the
-- only databases that have ever run the chain are the developer's own, the e2e
-- database the test suite truncates, and throwaway databases agents create and
-- drop. Every one of them is rebuilt from zero rather than migrated forward.
-- Under those conditions the rule protects nothing, and the chain it protects
-- had become 3,700 lines of intermediate states — columns added and dropped
-- again, a search vector that outlived its column, a version counter that never
-- guarded anything, a description that moved between two tables twice, an end
-- time that could not say which day it was on. Nobody
-- reading the schema should have to replay that to learn what a booking is.
-- The owner decided to collapse the chain; this file is that decision. From
-- the first deploy onward the rule applies to this file as written, and every
-- change is a new numbered migration on top of it.
--
-- WHAT THIS FILE KEEPS FROM THE CHAIN. The chain was unusually well argued:
-- almost every constraint carries the defect that motivated it, the query that
-- proved no row violated it, and the alternative that was argued down. That
-- knowledge is condensed here beside the object it explains, not copied. Data
-- steps that only repaired intermediate states — the refund_amount backfill,
-- the hourly-rate conversion, the cancellation_hours fix-up, the search-vector
-- rebuilds, the orphaned-court stamping, the payment_status split's backfill —
-- are gone, with a one-line note where the object they repaired lives. A fresh
-- database has nothing to repair.
--
-- CONVENTIONS THE CHAIN SETTLED, IN FORCE HERE.
--
--   * Every migration opens with SET LOCAL lock_timeout = '3s', Up and Down,
--     and every DATA migration (a backfill, a repair UPDATE) also opens with
--     SET LOCAL app.bypass_tenant = 'on': row-level security is FORCEd, so it
--     binds to the schema owner too, and a backfill without the bypass touches
--     zero rows and says nothing. This file has no data step.
--     goose runs outside the application's own budgets, and a migration that
--     queues behind a long transaction takes the whole table down with it.
--     LOCAL is load-bearing: a plain SET leaks past COMMIT into every later
--     migration on the same connection. Proved under goose, which wraps each
--     migration in a transaction: a blocked ALTER fails with SQLSTATE 55P03
--     after three seconds instead of waiting.
--   * Every function pins search_path = pg_catalog, public. None is SECURITY
--     DEFINER, so escalation is not the concern. booking_starts_at and
--     local_day are IMMUTABLE and the first sits inside an expression index;
--     an IMMUTABLE function that resolves a different cast because the
--     caller's search_path differs is an index whose stored values no longer
--     describe the rows, corrupted silently. The trigger functions are pinned
--     so there is only one class of function to reason about.
--   * Timestamps are timestamptz. A booking's calendar day and clock time are
--     stored as date + time, and the ONLY places they become an instant are
--     booking_starts_at(), local_day() and the two generated span columns,
--     all in the venue timezone 'America/Argentina/Buenos_Aires'. Copies of
--     that arithmetic are what produced every midnight defect in this
--     product's history; there are exactly four, all in this file, and a
--     per-venue timezone would have to replace all four at once.
--   * Half-open ranges everywhere: '[)'. A booking that ends when the next one
--     starts does not overlap it; a price band ending at 12:00 and one
--     starting at 12:00 coexist. internal/slots says the same thing.
--   * Trigger refusals raise ERRCODE 23514 with an explicit CONSTRAINT name,
--     so pgx surfaces them through PgError.ConstraintName exactly like a CHECK,
--     and a test can say which rule fired. Tests name these constraints:
--     keep every name in this file.
--   * A constraint that is missing on purpose looks identical, in a dump, to
--     one missing by omission. Where the chain argued one down, the argument
--     is kept here in a sentence.
--
-- Column order inside each table is the order the chain left it in (a dropped
-- column leaves a gap, a later ADD COLUMN goes last), so the pg_dump proof
-- above can be byte-identical. That is why a few late additions sit after
-- updated_at rather than beside their siblings.
--
-- The file ends with an ACCESS section: the two roles, ownership, grants and
-- the policies. It comes last because grants name every object and policies
-- name the tables, and because anything created after it would be owned by
-- the wrong role.
-- ============================================================================

-- ==================== EXTENSIONS ====================

-- citext: emails compare case-insensitively without a lower() on every read.
CREATE EXTENSION IF NOT EXISTS citext;

-- btree_gist: lets a scalar (court_id, an enum) share a GiST index with a
-- range, which is what every EXCLUDE constraint below needs.
CREATE EXTENSION IF NOT EXISTS btree_gist;

-- ==================== SERVER CAPABILITY GUARD ====================
--
-- The four tables that grow with traffic — bookings, payments, audit_log and
-- webhook_events — default their primary key to uuidv7() rather than
-- gen_random_uuid(); the argument is beside those tables. uuidv7() is a
-- PostgreSQL 18 built-in, so there is no extension to install and no function
-- of ours to maintain, but a server older than 18 would otherwise create four
-- tables with a default nobody checked. This fails the migration loudly
-- instead. The deployment is on 18.6 (docker-compose.yml, docker-compose.e2e.yml).
-- +goose StatementBegin
DO $$
BEGIN
    IF to_regprocedure('pg_catalog.uuidv7()') IS NULL THEN
        RAISE EXCEPTION
            'uuidv7() is a PostgreSQL 18 built-in and this server reports %; '
            'upgrade the server or change those four defaults to gen_random_uuid()',
            current_setting('server_version');
    END IF;
END;
$$;
-- +goose StatementEnd

-- ==================== ENUMS AND RANGE TYPES ====================

CREATE TYPE user_role      AS ENUM ('owner', 'client', 'superadmin');
CREATE TYPE booking_status AS ENUM ('pending', 'confirmed', 'cancelled', 'completed', 'no_show');

-- payments.status only. It was once shared with bookings.payment_status, which
-- is how 'partial_refund' — a state only a booking with several payment rows
-- can be in — ended up on a type that also described a single provider
-- transaction; bookings now carry collection_status and refund_status instead
-- (see the bookings table). The value stays because an enum value can only
-- ever be appended, never removed, which is also why the bookings pair is
-- text + CHECK and not a second enum. Giving payments.status its own
-- vocabulary (pending/approved/rejected/charged_back) is the still-open half
-- of that finding: it changes what the MercadoPago flow means, not the shape.
CREATE TYPE payment_status AS ENUM ('unpaid', 'deposit_paid', 'fully_paid', 'refunded', 'refund_pending', 'partial_refund');
CREATE TYPE payment_method AS ENUM ('mercadopago', 'cash', 'transfer');
CREATE TYPE court_type     AS ENUM ('indoor', 'outdoor', 'semi_covered');
-- 2026-09-15: volleyball, hockey and pickleball folded in directly (not a new
-- numbered migration) per docs/adr/0003-goose-migrations.md's still-open
-- direct-edit allowance; databases are recreated from scratch, not migrated
-- forward, so there is nothing an ADD VALUE migration would need to reach.
CREATE TYPE sport_type     AS ENUM ('padel', 'tennis', 'soccer', 'basketball', 'volleyball', 'hockey', 'pickleball');
CREATE TYPE day_of_week    AS ENUM ('monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday');

-- A range over time-of-day. PostgreSQL ships daterange/tsrange/tstzrange but
-- no timerange. It carried court_prices' first exclusion constraint; that
-- constraint now runs over minutes (span_min, below) because a timerange
-- cannot express a band that wraps past midnight. The type stays: dropping it
-- is a separate decision, and nothing else should ever be compared with it,
-- since a block and a booking have to be comparable and two range types are
-- not.
CREATE TYPE timerange AS RANGE (subtype = time);

-- ==================== SHARED FUNCTIONS ====================

-- +goose StatementBegin
CREATE FUNCTION trigger_set_updated_at() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path = pg_catalog, public
AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- OPTIMISTIC CONCURRENCY ON THE ADMIN TABLES (complexes, courts, court_prices).
--
-- Two staff members editing the same venue is otherwise last-write-wins. Each
-- loads the row, changes the one field they came for, and PUTs every column
-- back; whoever saves second overwrites the other's change with a value read
-- before that change existed, and both get 200 (API-08). `version` is what lets
-- the second one be told: a client that read version 3 sends it back with its
-- write, and if anything moved in between the row is at 4, the UPDATE matches
-- nothing, the stores turn that into data.ErrEditConflict and the handlers into
-- 409. A client that sends no version keeps the old behaviour exactly.
--
-- WHY THOSE THREE TABLES. They are what an owner and their staff edit through a
-- form, from two browsers, over the same minute. Bookings are deliberately not
-- among them: the race that matters there is two people claiming the same
-- hours, which a version counter cannot see and bookings_no_overlapping_span
-- already refuses. The counter this schema once carried on bookings is the one
-- described there as "not a lock" — that argument is about bookings, not about
-- a form.
--
-- WHY A TRIGGER RATHER THAN `version = version + 1` IN EACH UPDATE. Every
-- writer has to bump it, including the ones that do not check it: the
-- MercadoPago credential writes on complexes, the soft-delete, a backfill run by
-- hand. One of those forgetting is a version that stands still while the row
-- changes — worse than no version at all, a lock that reports success. The
-- trigger cannot be forgotten.
-- +goose StatementBegin
CREATE FUNCTION trigger_bump_version() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path = pg_catalog, public
AS $$
BEGIN
    -- OLD.version, not NEW.version: a writer that sends the whole row back
    -- would otherwise be able to write the counter itself, which is the one
    -- thing a version counter must not allow.
    NEW.version = OLD.version + 1;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- THE TENANT COLUMN ON THE FOUR CHILD TABLES, DERIVED RATHER THAN SUPPLIED.
--
-- court_prices, blocked_slots and slot_locks hang off a court and
-- booking_link_tokens off a booking; each reaches its tenant through that
-- parent, and each still carries its own complex_id so that the explicit half
-- of the isolation — the WHERE clause a human can read, which is what still
-- holds when a policy is relaxed or a path takes a bypass it did not need — is
-- writable on them as it is on the other fifteen tables.
--
-- A stored derivable value needs two things to stay honest, and both are paid
-- here rather than skipped:
--
--   * A COMPOSITE FOREIGN KEY on each of the four, (court_id, complex_id) ->
--     courts (id, complex_id) or (booking_id, complex_id) -> bookings (id,
--     complex_id), which is what makes the copy impossible to contradict. A
--     plain FK on the parent id alone would let a row name court A and tenant
--     B; the composite one is refused by the database. It carries the same
--     ON DELETE CASCADE the single-column key would have, and it replaces that
--     key rather than sitting beside it, so a deletion satisfies one constraint
--     and not two.
--
--   * THESE TRIGGERS, which fill the column from the parent, so no INSERT has
--     to supply it and none can supply it wrongly. They overwrite whatever the
--     caller passed rather than filling only NULLs: a derived column that can be
--     overridden is a derived column that will be, and the whole value of this
--     one is that it cannot disagree with the parent.
--
-- The parent is read under the same transaction as the write, and row-level
-- security on courts does not get in the way: the function runs inside the
-- caller's session, and every caller that may insert one of these rows can
-- already see its own court. A row whose court is invisible fails the composite
-- foreign key instead, which is the refusal we want.
-- +goose StatementBegin
CREATE FUNCTION set_complex_id_from_court() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    SELECT c.complex_id INTO NEW.complex_id
    FROM courts c
    WHERE c.id = NEW.court_id;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION set_complex_id_from_booking() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    SELECT b.complex_id INTO NEW.complex_id
    FROM bookings b
    WHERE b.id = NEW.booking_id;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- WHEN A BOOKING STARTS IS AN INSTANT, NOT A TIME OF DAY. `time + interval`
-- wraps at midnight: at 23:00 an upper bound of "+2 hours" is 01:00, below
-- the lower bound, and the 2-hour reminder selected the empty set from 22:00
-- local onward. `(date + start_time) AT TIME ZONE zone` is a timestamptz and
-- cannot wrap or straddle a date. IMMUTABLE is the same claim PostgreSQL makes
-- for timezone(text, timestamp) itself, and it is what lets the reminder index
-- be an expression index.
-- +goose StatementBegin
CREATE FUNCTION booking_starts_at(booking_date date, start_time time) RETURNS timestamptz
    LANGUAGE sql
    IMMUTABLE
    STRICT
    PARALLEL SAFE
    SET search_path = pg_catalog, public
AS $$
    SELECT (booking_date + start_time) AT TIME ZONE 'America/Argentina/Buenos_Aires'
$$;
-- +goose StatementEnd

COMMENT ON FUNCTION booking_starts_at(date, time) IS
    'The instant a booking begins. The one place date and start_time are combined in SQL.';

-- "THE BOOKINGS OF DAY X", ONCE, NOW THAT A BOOKING CAN BELONG TO TWO DAYS.
-- `WHERE date = $1` answers "started on day X"; a grid for day X has to draw
-- what "occupies day X", or it offers 00:00 as free under a booking that runs
-- until 01:00. Callers ask `span && local_day($1)`. An overnight booking
-- therefore appears on both days' lists, which is the honest projection of a
-- thing happening on both days; "sold on day X" is `lower(span)`.
-- +goose StatementBegin
CREATE FUNCTION local_day(d date) RETURNS tstzrange
    LANGUAGE sql
    IMMUTABLE
    STRICT
    PARALLEL SAFE
    SET search_path = pg_catalog, public
AS $$
    SELECT tstzrange(
        d::timestamp        AT TIME ZONE 'America/Argentina/Buenos_Aires',
        (d + 1)::timestamp  AT TIME ZONE 'America/Argentina/Buenos_Aires',
        '[)'
    )
$$;
-- +goose StatementEnd

COMMENT ON FUNCTION local_day(date) IS
    'A calendar day as an absolute range. The one place a local day becomes two instants.';

-- ==================== IDENTITY ====================

CREATE TABLE users (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email                 CITEXT NOT NULL,
    password_hash         BYTEA NOT NULL,
    first_name            TEXT NOT NULL,
    last_name             TEXT NOT NULL,
    phone                 TEXT NOT NULL,
    role                  user_role NOT NULL DEFAULT 'owner',
    is_active             BOOLEAN NOT NULL DEFAULT true,
    failed_login_attempts INT NOT NULL DEFAULT 0,
    locked_until          TIMESTAMPTZ,
    last_failed_login     TIMESTAMPTZ,
    email_verified        BOOLEAN NOT NULL DEFAULT false,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT users_email_key UNIQUE (email)
);
CREATE TRIGGER set_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

CREATE INDEX idx_users_created_at ON users (created_at DESC);

-- The unverified-account sweep. Unverified rows are a shrinking minority of a
-- growing table, which is exactly the shape a partial index is for.
CREATE INDEX idx_users_unverified_stale ON users (created_at)
    WHERE email_verified = false;

-- Every token is stored as its SHA-256; the plaintext is unrecoverable. The
-- hash is UNIQUE on all three tables: GetRefreshTokenByHash is a `:one` query,
-- so two rows with one hash would make the session a refresh returns depend on
-- which row the plan reached first. A natural collision is not the threat; a
-- re-issue bug or a restored backup double-inserting is, and a UNIQUE catches
-- that where a plain index does not.
CREATE TABLE refresh_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL,
    token_hash  BYTEA NOT NULL,
    used_at     TIMESTAMPTZ,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT refresh_tokens_token_hash_key UNIQUE (token_hash),
    CONSTRAINT refresh_tokens_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);
CREATE INDEX idx_refresh_tokens_user_id    ON refresh_tokens (user_id);
CREATE INDEX idx_refresh_tokens_expires_at ON refresh_tokens (expires_at);
-- The sweep is `expires_at <= now() OR (used_at IS NOT NULL AND used_at < ...)`.
-- The OR is what defeats a single index; with used_at indexed too the planner
-- builds a BitmapOr over both arms.
CREATE INDEX idx_refresh_tokens_used_at ON refresh_tokens (used_at)
    WHERE used_at IS NOT NULL;

CREATE TABLE email_verification_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL,
    token_hash  BYTEA NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT email_verification_tokens_token_hash_key UNIQUE (token_hash),
    CONSTRAINT email_verification_tokens_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);
CREATE INDEX idx_email_verification_tokens_user    ON email_verification_tokens (user_id);
CREATE INDEX idx_email_verification_tokens_expires ON email_verification_tokens (expires_at);

CREATE TABLE password_reset_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL,
    token_hash  BYTEA NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT password_reset_tokens_token_hash_key UNIQUE (token_hash),
    CONSTRAINT password_reset_tokens_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);
CREATE INDEX idx_password_reset_tokens_user    ON password_reset_tokens (user_id);
CREATE INDEX idx_password_reset_tokens_expires ON password_reset_tokens (expires_at);

-- EXTERNAL IDENTITY LINKS. One row per external identity linked to a local
-- account — today, one Google account behind Sign in with Google. Not
-- tenant-scoped: like users itself, an identity link belongs to the platform
-- account and not to any one complex, so it carries no row-level security
-- policy — the same posture as users, refresh_tokens,
-- email_verification_tokens and password_reset_tokens above, and for the same
-- reason (authentication runs before any tenant is known). Its DML grant comes
-- from the blanket GRANT in the ACCESS section, like every other table here.
CREATE TABLE user_identities (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL,
    provider   TEXT NOT NULL CHECK (provider IN ('google')),
    subject    TEXT NOT NULL,
    email      CITEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT user_identities_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    -- One local account per external identity: the same Google account
    -- cannot silently take over a second local user.
    CONSTRAINT user_identities_provider_subject_key UNIQUE (provider, subject),
    -- One linked identity per provider per account: a user cannot end up
    -- signed into two different Google accounts under one local user.
    CONSTRAINT user_identities_user_id_provider_key UNIQUE (user_id, provider)
);

CREATE INDEX idx_user_identities_user_id ON user_identities (user_id);

-- ==================== VENUES ====================

-- A complex is the tenant. Everything an owner sees is scoped by complex_id,
-- and the composite foreign keys further down are what keep a booking, its
-- court and its client inside one tenant at the schema level rather than by a
-- dozen hand-written `if booking.ComplexID != complex.ID` comparisons.
--
-- No description column: the venue's prose became the amenities vocabulary,
-- and a court got the free text instead (courts.description). No search
-- vector: nothing ever searched it, and its trigger read a dropped column for
-- two migrations before anyone noticed. When full-text search over venues is
-- wanted it comes back as a GENERATED column, which cannot drift.
CREATE TABLE complexes (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id            UUID NOT NULL,
    name                TEXT NOT NULL,
    slug                TEXT NOT NULL,
    address             TEXT NOT NULL,
    city                TEXT NOT NULL,
    province            TEXT NOT NULL,
    country_code        TEXT NOT NULL DEFAULT 'AR',
    currency            TEXT NOT NULL DEFAULT 'ARS',
    phone               TEXT NOT NULL,
    email               CITEXT,
    logo_url            TEXT,
    cover_url           TEXT,
    deposit_percentage  INTEGER NOT NULL DEFAULT 30,
    -- Hours before the start inside which a cancellation forfeits the deposit.
    -- Bounded 1..168: the reader treats <= 0 as "always refundable", which also
    -- made the booking's public link immortal, and the two zero rows that ever
    -- existed came from a request that omitted the field, not from a venue
    -- choosing it. The column default of 24 never applies (the insert names
    -- the column), so the floor has to live here.
    cancellation_hours  INTEGER NOT NULL DEFAULT 24,
    latitude            DOUBLE PRECISION,
    longitude           DOUBLE PRECISION,
    is_active           BOOLEAN NOT NULL DEFAULT true,
    -- MercadoPago OAuth credentials, sealed as authenticated ciphertext by
    -- internal/data/mpcred.go. NULL means not connected; '' is forbidden so
    -- "connected with nothing" is not a representable state. The CHECKs are
    -- the floor under the Go-side refusal, not the guard itself.
    mp_access_token     TEXT,
    mp_refresh_token    TEXT,
    mp_user_id          TEXT,
    deleted_at          TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- A closed vocabulary rather than prose or a join table: it can be
    -- filtered and rendered as one icon everywhere, the set is small and
    -- always read whole, and `amenities @> '{bar}'` has a GIN index the day
    -- something asks. Duplicates are not constrained (a CHECK cannot hold a
    -- subquery); the handler deduplicates, and a duplicate costs a badge
    -- rendered twice, not a wrong answer.
    amenities           TEXT[] NOT NULL DEFAULT '{}',
    -- From MercadoPago's expires_in; without it every connected venue was
    -- blindly refreshed every 12h.
    mp_token_expires_at TIMESTAMPTZ,
    -- The optimistic-concurrency counter; see trigger_bump_version above for
    -- why it is a trigger and not an expression in each UPDATE. NOT NULL
    -- DEFAULT 1 so there is no "not versioned yet" state for a reader to
    -- handle.
    version             INTEGER NOT NULL DEFAULT 1,
    -- complexes.slug is UNIQUE across the whole table, deleted rows included:
    -- the slug is the public URL and stays reserved after a soft delete.
    CONSTRAINT complexes_slug_key UNIQUE (slug),
    CONSTRAINT complexes_owner_id_fkey
        FOREIGN KEY (owner_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT complexes_deposit_percentage_range
        CHECK (deposit_percentage >= 0 AND deposit_percentage <= 100),
    CONSTRAINT complexes_cancellation_hours_range
        CHECK (cancellation_hours >= 1 AND cancellation_hours <= 168),
    CONSTRAINT complexes_mp_access_token_nonempty  CHECK (mp_access_token <> ''),
    CONSTRAINT complexes_mp_refresh_token_nonempty CHECK (mp_refresh_token <> ''),
    CONSTRAINT complexes_mp_user_id_nonempty       CHECK (mp_user_id <> ''),
    CONSTRAINT complexes_amenities_known
        CHECK (amenities <@ ARRAY[
            'parking',
            'changing_rooms',
            'showers',
            'bar',
            'racket_rental',
            'pro_shop',
            'wifi',
            'lockers',
            'lessons',
            'tournaments',
            'accessible',
            'match_recording'
        ]::TEXT[])
);
-- Both fire BEFORE UPDATE and they touch different columns, so neither depends
-- on the other; the names order them (set_updated_at, then set_version).
CREATE TRIGGER set_updated_at BEFORE UPDATE ON complexes
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();
CREATE TRIGGER set_version BEFORE UPDATE ON complexes
    FOR EACH ROW EXECUTE FUNCTION trigger_bump_version();

CREATE INDEX idx_complexes_owner_id   ON complexes (owner_id);
CREATE INDEX idx_complexes_slug       ON complexes (slug) WHERE deleted_at IS NULL;
CREATE INDEX idx_complexes_city       ON complexes (city) WHERE deleted_at IS NULL;
CREATE INDEX idx_complexes_created_at ON complexes (created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_complexes_amenities  ON complexes USING gin (amenities);
-- The MP token-refresh cron's own predicate; before it the sweep walked
-- idx_complexes_created_at with every real condition pushed into a Filter.
CREATE INDEX idx_complexes_mp_refresh_due ON complexes (mp_token_expires_at)
    WHERE mp_refresh_token IS NOT NULL AND deleted_at IS NULL;

-- Opening hours per weekday. close_time <= open_time means "closes after
-- midnight" (20:00-02:00), and the reader adds 1440 minutes; is_closed rows
-- carry 00:00/00:00. So there is deliberately NO CHECK (open_time < close_time)
-- here, unlike blocked_slots: it would delete a shipped feature. The one shape
-- that is wrong is a zero-length open window, which the wrap would turn into a
-- 24-hour day.
CREATE TABLE complex_schedules (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    complex_id  UUID NOT NULL,
    day         day_of_week NOT NULL,
    open_time   TIME NOT NULL,
    close_time  TIME NOT NULL,
    is_closed   BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT complex_schedules_complex_id_day_key UNIQUE (complex_id, day),
    CONSTRAINT complex_schedules_complex_id_fkey
        FOREIGN KEY (complex_id) REFERENCES complexes (id) ON DELETE CASCADE,
    CONSTRAINT complex_schedules_open_window_not_empty
        CHECK (is_closed OR open_time <> close_time)
);
CREATE TRIGGER set_updated_at BEFORE UPDATE ON complex_schedules
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- No duration_minutes on a court: a booking's length is chosen per request
-- from the three permitted durations, the same on every court, so a per-court
-- default described nothing and was dropped.
CREATE TABLE courts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    complex_id  UUID NOT NULL,
    name        TEXT NOT NULL,
    sport       sport_type NOT NULL DEFAULT 'padel',
    court_type  court_type NOT NULL DEFAULT 'indoor',
    is_active   BOOLEAN NOT NULL DEFAULT true,
    deleted_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Free text where the values do not repeat across venues (the floor, the
    -- walls, the lighting); structured data where they do (amenities, on the
    -- venue). Nullable so "never described" and "cleared" stay distinct.
    -- Capped at about two lines on a storefront card, here as well as in the
    -- handler because seeds and imports reach the table directly.
    description TEXT,
    -- The optimistic-concurrency counter; see trigger_bump_version above.
    version     INTEGER NOT NULL DEFAULT 1,
    -- Pointed at by the composite FKs on bookings, and by the ones that carry
    -- complex_id onto court_prices, blocked_slots and slot_locks. Redundant for
    -- uniqueness (id alone is the PK); it exists only to be referenced.
    CONSTRAINT courts_id_complex_id_key UNIQUE (id, complex_id),
    CONSTRAINT courts_complex_id_fkey
        FOREIGN KEY (complex_id) REFERENCES complexes (id) ON DELETE CASCADE,
    CONSTRAINT courts_description_length
        CHECK (description IS NULL OR char_length(description) <= 200)
);
CREATE TRIGGER set_updated_at BEFORE UPDATE ON courts
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();
CREATE TRIGGER set_version BEFORE UPDATE ON courts
    FOR EACH ROW EXECUTE FUNCTION trigger_bump_version();

CREATE INDEX idx_courts_complex_id ON courts (complex_id) WHERE deleted_at IS NULL;
-- A referential-integrity probe from complexes carries no predicate, so it
-- cannot use the partial index above and would scan the table on every
-- cascade. The unqualified twin serves the FK.
CREATE INDEX idx_courts_complex_id_all ON courts (complex_id);

-- Court names are unique per complex among LIVE courts only. An owner who
-- deletes "Cancha 3" and later adds a new "Cancha 3" is doing something
-- ordinary. Exact-match, not case-folded: whether "Cancha 1" and "cancha 1"
-- are the same court is a product decision, not a schema invariant. This is
-- the constraint that becomes irrecoverable if added late: every reminder and
-- every export identifies a court by name.
CREATE UNIQUE INDEX courts_active_name_unique ON courts (complex_id, name)
    WHERE deleted_at IS NULL;

-- A DELETED VENUE TAKES ITS COURTS WITH IT. Both tables carry deleted_at and
-- referential integrity cannot see soft deletion, so a live court under a
-- deleted complex was a structurally valid row that every court query still
-- returned. Two triggers, because a cascade alone only fires when the parent
-- moves:
--
--   1. the cascade: deleting a complex stamps its live courts with the
--      complex's OWN deleted_at (the court stopped being reachable when the
--      venue did; NOW() would be a fact the trigger invented). AFTER, so a
--      rule refusing the parent write cannot leave stamped children behind;
--      the WHEN clause makes an ordinary edit cost nothing and stops a
--      re-delete from restamping with a later timestamp.
--   2. the refusal: a court cannot be live while its complex is deleted,
--      which covers the two writes a cascade never sees — inserting a court
--      under a deleted venue and clearing a court's own deleted_at.
--
-- Section 15 allows triggers for bookkeeping, and both are on that side of
-- the line: the cascade copies a timestamp one table out, the refusal is an
-- integrity constraint a CHECK cannot express (it reads another table) and a
-- foreign key cannot express (it cannot say "and the parent's deleted_at must
-- be NULL"). ComplexModel.SoftDeleteCascade does the same in one transaction
-- and reports the count; what it cannot do is bind the writers it does not
-- own.
--
-- Restoring is not the inverse: clearing complexes.deleted_at does not revive
-- the courts. A restore that wants them back clears the complex FIRST, then
-- un-stamps courts one statement at a time; the refusal permits that order
-- and no other. court_prices, complex_schedules and blocked_slots get no
-- deleted_at on purpose: none is independently addressable, so once the
-- court or the venue is out of the views they are unreachable, and their FKs
-- already cascade on a real delete. bookings keep everything; they are the
-- record of money that moved.
--
-- The chain's backfill (stamp courts already orphaned before enforcement
-- existed) is dropped: an init file has no orphans to stamp.

-- +goose StatementBegin
CREATE FUNCTION complexes_cascade_soft_delete_to_courts() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path = pg_catalog, public
AS $$
BEGIN
    UPDATE courts
    SET deleted_at = NEW.deleted_at,
        is_active  = false
    WHERE complex_id = NEW.id
      AND deleted_at IS NULL;

    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER complexes_cascade_soft_delete_to_courts
    AFTER UPDATE ON complexes
    FOR EACH ROW
    WHEN (OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL)
    EXECUTE FUNCTION complexes_cascade_soft_delete_to_courts();

-- The WHEN clause means the primary-key lookup is paid only on writes that
-- leave a court live, and never on the cascade's own UPDATE. A concurrent
-- revive holds its row lock across commit, so a simultaneous deletion blocks
-- on it and restamps; either the revive is refused or it is undone, and no
-- interleaving ends with a live court under a deleted complex.
-- +goose StatementBegin
CREATE FUNCTION courts_forbid_live_under_deleted_complex() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path = pg_catalog, public
AS $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM complexes
        WHERE id = NEW.complex_id
          AND deleted_at IS NOT NULL
    ) THEN
        RAISE EXCEPTION
            'court %: cannot be live while complex % is soft-deleted',
            NEW.id, NEW.complex_id
            USING ERRCODE = '23514',
                  CONSTRAINT = 'courts_no_live_under_deleted_complex';
    END IF;

    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER courts_forbid_live_under_deleted_complex
    BEFORE INSERT OR UPDATE ON courts
    FOR EACH ROW
    WHEN (NEW.deleted_at IS NULL)
    EXECUTE FUNCTION courts_forbid_live_under_deleted_complex();

-- THE FILTERED VIEWS. The public site, the public booking flow, the owner
-- dashboards and the availability grid read these; the tables stay for the
-- readers that legitimately see deleted rows — the admin console, the audit
-- trail, the slug reservation, and the joins that put a court's name on a
-- historical booking (through the view that name would silently become '').
--
-- active_courts carries the join even though the triggers make it redundant.
-- It is redundant GIVEN the triggers, and a view whose truth depends on a
-- trigger elsewhere quietly becomes wrong when that trigger is dropped,
-- disabled for a bulk load, or missing on a restore. It costs a primary-key
-- lookup against a table with one row per venue.
--
-- Columns are listed, not `*`: a view freezes its expansion at creation time
-- either way, so the only choice is whether the freeze is visible. Adding a
-- column to complexes or courts means CREATE OR REPLACE VIEW in the same
-- migration and `sqlc generate`; internal/data's db.Complex(c) / db.Court(c)
-- conversions compile only while the view carries every column of the table
-- in the same order, which is the guard.
--
-- Both views are made security_invoker in the ACCESS section below, and that
-- is not optional: a view runs as its owner, the owner is vibe_migrator, and
-- without it every read through these two — most of the public and owner
-- read surface — would be evaluated as the owner and return every tenant's
-- rows.

CREATE VIEW active_complexes AS
SELECT id, owner_id, name, slug, address, city, province, country_code, currency,
       phone, email, logo_url, cover_url, deposit_percentage, cancellation_hours,
       latitude, longitude, is_active, mp_access_token, mp_refresh_token, mp_user_id,
       deleted_at, created_at, updated_at, amenities, mp_token_expires_at, version
FROM complexes
WHERE deleted_at IS NULL;

COMMENT ON VIEW active_complexes IS
    'Complexes that are not soft-deleted. Read this, not the table, unless the caller needs deleted rows (admin, audit, slug reservation).';

CREATE VIEW active_courts AS
SELECT c.id, c.complex_id, c.name, c.sport, c.court_type, c.is_active,
       c.deleted_at, c.created_at, c.updated_at, c.description, c.version
FROM courts c
JOIN complexes cx ON cx.id = c.complex_id
WHERE c.deleted_at IS NULL
  AND cx.deleted_at IS NULL;

COMMENT ON VIEW active_courts IS
    'Courts that are not soft-deleted and whose complex is not soft-deleted either. Read this, not the table, unless the caller needs deleted rows (admin, audit, the court name on a historical booking).';

-- PRICE BANDS. price is an hourly rate in centavos; a booking of any permitted
-- duration is priced by walking it in 30-minute blocks against the band that
-- covers each block (internal/pricing.BookingPrice).
--
-- A band may run past midnight: Thursday 22:00-01:30 is the late end of
-- Thursday night and pays Thursday's rate, because a slot is priced by the
-- opening window it came out of. `time_from <= t AND t < time_to` on TIME
-- columns cannot say that (a band ending at 00:00 covered nothing), so
-- span_min is the band as minutes from that weekday's own midnight, with the
-- upper bound past 1440 when it wraps: [1320, 1530). Minutes have no midnight
-- in them, and a range whose ends are ordered by construction cannot be
-- compared backwards. time_from/time_to stay what the owner edits; span_min
-- derives from them.
--
-- The exclusion constraint is the actual invariant: for a court and a
-- weekday, at most one band covers any minute. findPrice's first-match loop is
-- correct because of it rather than merely repeatable; a UNIQUE on the
-- endpoints would have caught byte-identical bands and let 08:00-12:00 and
-- 10:00-14:00 coexist, and then the price a client is shown and the price
-- they are charged would come from two different sorts of one ambiguity.
-- Equal ends are refused: under the generation above time_to = time_from is
-- a 24-hour band, and an owner typing the same time twice meant zero length.
-- Same rule as complex_schedules_open_window_not_empty.
CREATE TABLE court_prices (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    court_id    UUID NOT NULL,
    price       INTEGER NOT NULL,
    day_type    day_of_week NOT NULL,
    time_from   TIME NOT NULL DEFAULT '08:00',
    time_to     TIME NOT NULL DEFAULT '23:00',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    span_min    int4range
        GENERATED ALWAYS AS (
            int4range(
                (EXTRACT(HOUR FROM time_from) * 60 + EXTRACT(MINUTE FROM time_from))::int,
                (EXTRACT(HOUR FROM time_to) * 60 + EXTRACT(MINUTE FROM time_to))::int
                    + CASE WHEN time_to <= time_from THEN 1440 ELSE 0 END,
                '[)'
            )
        ) STORED,
    -- The optimistic-concurrency counter; see trigger_bump_version above.
    version     INTEGER NOT NULL DEFAULT 1,
    -- The tenant, derived from the court by set_complex_id below and held
    -- honest by the composite foreign key; see set_complex_id_from_court above.
    complex_id  UUID NOT NULL,
    CONSTRAINT court_prices_court_id_fkey
        FOREIGN KEY (court_id, complex_id) REFERENCES courts (id, complex_id) ON DELETE CASCADE,
    CONSTRAINT court_prices_price_check CHECK (price >= 0),
    CONSTRAINT court_prices_band_not_empty CHECK (time_from <> time_to),
    CONSTRAINT court_prices_no_overlapping_rule
        EXCLUDE USING gist (
            court_id  WITH =,
            day_type  WITH =,
            span_min  WITH &&
        )
);
CREATE TRIGGER set_complex_id BEFORE INSERT OR UPDATE ON court_prices
    FOR EACH ROW EXECUTE FUNCTION set_complex_id_from_court();
CREATE TRIGGER set_updated_at BEFORE UPDATE ON court_prices
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();
CREATE TRIGGER set_version BEFORE UPDATE ON court_prices
    FOR EACH ROW EXECUTE FUNCTION trigger_bump_version();

CREATE INDEX idx_court_prices_court_id ON court_prices (court_id);
CREATE INDEX idx_court_prices_complex  ON court_prices (complex_id);

COMMENT ON COLUMN court_prices.price IS
    'Hourly rate in centavos (price per 60 minutes), in effect for [time_from, time_to) on day_type. A booking of any permitted duration is priced by walking it in 30-minute blocks against this rate (internal/pricing.BookingPrice).';

-- MAINTENANCE BLOCKS. A block is an interval, same zone and same half-open
-- convention as bookings.span, so `blocked_slots.span && bookings.span` is a
-- direct comparison and neither side can wrap. Before that, the in-transaction
-- guard compared clock readings and a 23:00 + 120-minute booking (end_time
-- '01:00') sailed over a 23:00-23:59 block, reproduced against a real database.
--
-- The start_time < end_time CHECK stays: the span is built from date+start to
-- date+end and is only well-defined while the end is later in the same day.
-- Known limitation, recorded rather than fixed: a block cannot cross midnight
-- the way a booking can; a venue files two rows. Letting it would mean a
-- duration column and a rebuilt span, a deliberate separate migration.
--
-- What the EXCLUDE closes is block-versus-block. Block-versus-booking is a
-- cross-table invariant no constraint can carry: InsertBlockedSlot takes the
-- same court-day advisory locks BookingModel.InsertSafe takes (every local
-- day the hours touch, ascending) and asks the mirror question inside its own
-- transaction (internal/data/slot_guard.go, courts.go). The span here is the
-- vocabulary those checks compare, not the guard.
CREATE TABLE blocked_slots (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    court_id    UUID NOT NULL,
    date        DATE NOT NULL,
    start_time  TIME NOT NULL,
    end_time    TIME NOT NULL,
    reason      TEXT,
    created_by  UUID,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    span        tstzrange
        GENERATED ALWAYS AS (
            tstzrange(
                (date + start_time) AT TIME ZONE 'America/Argentina/Buenos_Aires',
                (date + end_time)   AT TIME ZONE 'America/Argentina/Buenos_Aires',
                '[)'
            )
        ) STORED,
    -- The tenant, derived from the court by set_complex_id below; see
    -- set_complex_id_from_court above.
    complex_id  UUID NOT NULL,
    CONSTRAINT blocked_slots_court_id_fkey
        FOREIGN KEY (court_id, complex_id) REFERENCES courts (id, complex_id) ON DELETE CASCADE,
    CONSTRAINT blocked_slots_created_by_fkey
        FOREIGN KEY (created_by) REFERENCES users (id),
    -- The name is the one PostgreSQL generated for the original unnamed CHECK;
    -- comments in internal/data refer to it.
    CONSTRAINT blocked_slots_check CHECK (start_time < end_time),
    CONSTRAINT blocked_slots_no_overlapping_span
        EXCLUDE USING gist (
            court_id WITH =,
            span     WITH &&
        )
);
CREATE TRIGGER set_complex_id BEFORE INSERT OR UPDATE ON blocked_slots
    FOR EACH ROW EXECUTE FUNCTION set_complex_id_from_court();
CREATE TRIGGER set_updated_at BEFORE UPDATE ON blocked_slots
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

CREATE INDEX idx_blocked_slots_court_date ON blocked_slots (court_id, date);
CREATE INDEX idx_blocked_slots_date       ON blocked_slots (date);
CREATE INDEX idx_blocked_slots_created_by ON blocked_slots (created_by);
CREATE INDEX idx_blocked_slots_complex    ON blocked_slots (complex_id);

-- ==================== CLIENTS ====================

-- A client belongs to one complex; the same person at two venues is two rows.
-- total_bookings and no_shows are counters maintained by increment statements,
-- and a negative counter is not a smaller number, it is proof an increment ran
-- against the wrong row, silently inverting the is_blocked heuristics.
CREATE TABLE clients (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    complex_id     UUID NOT NULL,
    first_name     TEXT NOT NULL,
    last_name      TEXT NOT NULL,
    phone          TEXT NOT NULL,
    email          CITEXT,
    notes          TEXT,
    is_blocked     BOOLEAN NOT NULL DEFAULT false,
    total_bookings INTEGER NOT NULL DEFAULT 0,
    no_shows       INTEGER NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT clients_complex_id_phone_key UNIQUE (complex_id, phone),
    -- Referenced by bookings_client_in_same_complex.
    CONSTRAINT clients_id_complex_id_key UNIQUE (id, complex_id),
    CONSTRAINT clients_complex_id_fkey
        FOREIGN KEY (complex_id) REFERENCES complexes (id) ON DELETE CASCADE,
    CONSTRAINT clients_counters_non_negative
        CHECK (total_bookings >= 0 AND no_shows >= 0)
);
CREATE TRIGGER set_updated_at BEFORE UPDATE ON clients
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- The list endpoint paginates by keyset on (created_at, id); an index that
-- stops at complex_id resolves the tiebreak with a sort on every page, which
-- defeats the point of keyset pagination.
CREATE INDEX idx_clients_complex_id ON clients (complex_id, created_at, id);

-- ==================== BOOKINGS ====================

-- A BOOKING'S SPAN IS A RANGE, AND OVERLAP IS THE DATABASE'S JOB.
--
-- date + start_time does not identify a moment, and every question about when
-- a booking happens once had to rebuild the moment by hand, wrongly at
-- midnight. Twice that cost money or a reminder: a 23:00 booking stored with
-- end_time = 00:00 was invisible to `end_time > $start`, and a 22:30 booking
-- committed on top of it — two clients, one court, both paid. `span` is the
-- instant pair as stored truth, GENERATED from date, start_time and
-- duration_minutes so there is no second thing to keep in sync. The
-- arithmetic runs in the timestamp domain before AT TIME ZONE: `time +
-- interval` wraps at midnight (the defect), `timestamp + interval` does not,
-- and both it and timezone(text, timestamp) are IMMUTABLE, which a generated
-- column requires.
--
-- There is no end_time column. It was start_time + duration_minutes written
-- through modular clock arithmetic, so a two-hour booking at 23:00 stored
-- '01:00' — a correct answer to "what will the clock read" and a wrong answer
-- to "when does this end", shipped on every payload and into every
-- confirmation message with nothing saying which 01:00. A CHECK tying it to
-- start_time + duration would have had to encode the wrap to be true at all.
-- The wire carries starts_at/ends_at read off lower(span)/upper(span), and
-- server copy marks an end on the following day (internal/timezone).
--
-- bookings_no_overlapping_span refuses the 22:30 booking the old unique index
-- on (court, date, start_time) could not see, for every write path there will
-- ever be. Its predicate is NOT IN ('cancelled', 'no_show'), never merely
-- <> 'cancelled': a no_show is a court that went unused, recorded differently
-- because the client owes for it, and marking one has to put the hours back
-- on sale or there is no reason to mark it. releasedBookingStatuses in
-- internal/data/slot_guard.go is the Go copy of that list.
--
-- What the constraint does NOT do, so nobody deletes the guard that still has
-- work: it cannot express the stale-pending carve-out (whether an unpaid
-- public booking still holds its court depends on the clock, and a constraint
-- sees rows, not clocks), and it cannot see blocked_slots (EXCLUDE is
-- single-table). Both live in internal/data/slot_guard.go under the court-day
-- advisory locks.
--
-- MONEY. deposit_amount is arithmetic input, not decoration: ConfirmPayment
-- adds it to what was collected and decides fully_paid from the sum, so an
-- inflated deposit marks an unpaid booking as settled; hence <= price.
-- duration_minutes is restricted to the grid's permitted lengths. No version
-- counter: exactly one of four writers ever checked it, so it was not a lock,
-- and what actually refuses the race it was meant for is the trigger below.
--
-- TWO AXES, NOT ONE ENUM. What a booking has settled used to be one
-- six-value payment_status, and those values were two independent facts glued
-- together — how much was collected (unpaid | deposit_paid | fully_paid) and
-- where the give-back stands (none | pending | partial | full). Gluing them
-- destroyed the collection fact the moment a refund started: a deposit being
-- refunded and a full payment being refunded both read 'refund_pending'.
-- collection_status and refund_status are the two facts. text + CHECK rather
-- than enums because an enum cannot lose or reorder a value (which is how the
-- old type got stuck), and rather than a lookup table because
-- collection_status = 'unpaid' sits inside the stale-pending carve-out that
-- runs on every availability read and inside the partial index that serves
-- it; a partial index cannot read another table. The chain's backfill mapped
-- the old values as unpaid -> (unpaid, none), deposit_paid -> (deposit_paid,
-- none), fully_paid -> (fully_paid, none), refund_pending -> (fully_paid,
-- pending), partial_refund -> (fully_paid, partial), refunded -> (fully_paid,
-- full): a refund presupposes money was taken, and fully_paid is the strongest
-- claim that cannot weaken the rule below. A fresh database has no rows to
-- map, so the backfill is not here.
--
-- bookings_refund_needs_collected_money (you cannot refund money you never
-- took) is what makes the trigger below exactly as strong as the single-enum
-- rule it replaced: without it an (unpaid, full) row could exist and be
-- written back to (unpaid, none), which the old enum refused as
-- refunded -> unpaid. fully_paid -> deposit_paid is deliberately still
-- allowed: a stale MercadoPago deposit webhook landing on a booking the owner
-- already settled in cash is exactly that write, and which fact wins is a
-- product decision, not a floor. The refund axis is not monotonic for the
-- same reason (cash after a chargeback).
--
-- STATUS TRANSITIONS are two monotonicity rules, deliberately not the
-- handler's twenty-five-cell matrix: a terminal status (cancelled, completed,
-- no_show) may not return to a live one (pending, confirmed), and
-- collection_status may never return to unpaid. Together they refuse
-- "cancelled + refunded -> confirmed + unpaid" — money returned and the court
-- claimed again — and they are a strict subset of the handler's policy, so
-- the floor can never contradict the policy as it evolves. The matrix would
-- have rejected the refund path (deposit_paid -> refunded directly), drifted
-- as a second copy nobody reads, and turned every product change into a
-- migration. Terminal-to-terminal stays allowed (a full refund on a completed
-- booking).
--
-- TIME-ORDERED IDS. bookings, payments, audit_log and webhook_events are the
-- four tables that grow with traffic rather than with the customer count, and
-- they alone default their key to uuidv7() instead of gen_random_uuid(). A v4
-- key is sixteen random bytes, so every insert lands at a random point of the
-- B-tree: the write set is the whole index rather than its tail, the pages that
-- matter never stay in cache, and the index fragments as it fills — while these
-- four are read by time ("this week's bookings", "today's payments", "what
-- happened on the 9th"), an order a random key has nothing to do with. uuidv7()
-- is the same 128 bits with the first 48 given to a millisecond timestamp:
-- inserts land at the right-hand edge, adjacent rows are adjacent in time, and
-- the remaining 74 random bits are still far more than enough that an id is not
-- guessable. The server guard is at the top of this file.
--
-- The rule for whoever adds the next table: nothing whose id is a secret gets
-- v7, because a v7 id publishes the millisecond it was created. There is none
-- here — every token in this schema is a hash in its own column, never a
-- primary key.
CREATE TABLE bookings (
    id                UUID PRIMARY KEY DEFAULT uuidv7(),
    complex_id        UUID NOT NULL,
    court_id          UUID NOT NULL,
    client_id         UUID NOT NULL,
    date              DATE NOT NULL,
    start_time        TIME NOT NULL,
    duration_minutes  INTEGER NOT NULL DEFAULT 90,
    price             INTEGER NOT NULL,
    deposit_amount    INTEGER NOT NULL DEFAULT 0,
    status            booking_status NOT NULL DEFAULT 'pending',
    reminder_sent_2h  BOOLEAN NOT NULL DEFAULT false,
    notes             TEXT,
    -- NULL for a public (storefront) booking, the owner's user for a manual
    -- one. The stale-pending carve-out and its index key on that distinction.
    created_by        UUID,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- "THIS CANCELLATION OWES A REFUND", written by the same UPDATE that
    -- cancels, only when the cancel path's own owesRefund is true, and cleared
    -- by ClaimRefund once a durable claim exists. The reconciliation sweep
    -- selects on this column and nothing else: status = cancelled with
    -- payment_status still paid and no attempt row is ALSO the shape a
    -- deliberately declined out-of-window cancellation produces, and a sweep
    -- that could not tell them apart would refund money the venue may keep.
    refund_intent_at  TIMESTAMPTZ,
    span              tstzrange
        GENERATED ALWAYS AS (
            tstzrange(
                (date + start_time) AT TIME ZONE 'America/Argentina/Buenos_Aires',
                ((date + start_time) + make_interval(mins => duration_minutes))
                    AT TIME ZONE 'America/Argentina/Buenos_Aires',
                '[)'
            )
        ) STORED,
    collection_status TEXT NOT NULL DEFAULT 'unpaid',
    refund_status     TEXT NOT NULL DEFAULT 'none',
    -- Referenced by payments_booking_in_same_complex.
    CONSTRAINT bookings_id_complex_id_key UNIQUE (id, complex_id),
    CONSTRAINT bookings_complex_id_fkey
        FOREIGN KEY (complex_id) REFERENCES complexes (id) ON DELETE CASCADE,
    -- No ON DELETE clause on court_id and client_id, on purpose: a court
    -- cannot vanish from under a booking, which is why courts are soft-deleted.
    CONSTRAINT bookings_court_id_fkey
        FOREIGN KEY (court_id) REFERENCES courts (id),
    CONSTRAINT bookings_client_id_fkey
        FOREIGN KEY (client_id) REFERENCES clients (id),
    CONSTRAINT bookings_created_by_fkey
        FOREIGN KEY (created_by) REFERENCES users (id),
    -- CROSS-TENANT COMPOSITE KEYS. A row naming one tenant's complex and
    -- another tenant's court was structurally valid; with these it is not.
    -- complex_id therefore stops being independently writable: moving a
    -- booking to another complex means moving its court and client in the
    -- same statement, accepted knowingly. The single-column FKs above are
    -- kept beside them: one redundant index probe per insert, in exchange for
    -- never getting referential semantics subtly wrong in a rollback.
    CONSTRAINT bookings_court_in_same_complex
        FOREIGN KEY (court_id, complex_id) REFERENCES courts (id, complex_id),
    CONSTRAINT bookings_client_in_same_complex
        FOREIGN KEY (client_id, complex_id) REFERENCES clients (id, complex_id),
    CONSTRAINT bookings_duration_minutes_check CHECK (duration_minutes > 0),
    CONSTRAINT bookings_duration_minutes_permitted CHECK (duration_minutes IN (60, 90, 120)),
    CONSTRAINT bookings_price_check CHECK (price >= 0),
    CONSTRAINT bookings_deposit_amount_check CHECK (deposit_amount >= 0),
    CONSTRAINT bookings_deposit_within_price CHECK (deposit_amount <= price),
    -- An intent marker on a live booking is not a state the product has: it is
    -- set only by a cancel path in the statement that writes cancelled, and
    -- the reversal trigger keeps a cancelled row from going live again.
    CONSTRAINT bookings_refund_intent_only_when_cancelled
        CHECK (refund_intent_at IS NULL OR status = 'cancelled'),
    CONSTRAINT bookings_collection_status_valid
        CHECK (collection_status IN ('unpaid', 'deposit_paid', 'fully_paid')),
    CONSTRAINT bookings_refund_status_valid
        CHECK (refund_status IN ('none', 'pending', 'partial', 'full')),
    CONSTRAINT bookings_refund_needs_collected_money
        CHECK (refund_status = 'none' OR collection_status <> 'unpaid'),
    CONSTRAINT bookings_no_overlapping_span
        EXCLUDE USING gist (
            court_id WITH =,
            span     WITH &&
        ) WHERE (status NOT IN ('cancelled', 'no_show'))
);
CREATE TRIGGER set_updated_at BEFORE UPDATE ON bookings
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- +goose StatementBegin
CREATE FUNCTION bookings_forbid_status_reversal() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path = pg_catalog, public
AS $$
BEGIN
    IF OLD.status IN ('cancelled', 'completed', 'no_show')
       AND NEW.status IN ('pending', 'confirmed') THEN
        RAISE EXCEPTION
            'booking %: status cannot return to % from the terminal status %',
            OLD.id, NEW.status, OLD.status
            USING ERRCODE = '23514',
                  CONSTRAINT = 'bookings_status_no_terminal_reentry';
    END IF;

    IF OLD.collection_status <> 'unpaid' AND NEW.collection_status = 'unpaid' THEN
        RAISE EXCEPTION
            'booking %: collection_status cannot return to unpaid from %',
            OLD.id, OLD.collection_status
            USING ERRCODE = '23514',
                  CONSTRAINT = 'bookings_collection_status_no_return_to_unpaid';
    END IF;

    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER bookings_forbid_status_reversal
    BEFORE UPDATE ON bookings
    FOR EACH ROW
    WHEN (OLD.status IS DISTINCT FROM NEW.status
          OR OLD.collection_status IS DISTINCT FROM NEW.collection_status)
    EXECUTE FUNCTION bookings_forbid_status_reversal();

CREATE INDEX idx_bookings_complex_date ON bookings (complex_id, date, status);
CREATE INDEX idx_bookings_court_date   ON bookings (court_id, date, start_time);
CREATE INDEX idx_bookings_client_date  ON bookings (client_id, date DESC);
CREATE INDEX idx_bookings_created_by   ON bookings (created_by);

-- The day-scoped dashboard reads by complex and the exclusion constraint's
-- GiST index is per court, so `span && local_day($2)` by complex gets its own.
-- Not partial: the dashboard draws cancelled bookings struck through.
CREATE INDEX idx_bookings_complex_span ON bookings USING gist (complex_id, span);

-- The 2-hour reminder ranges over booking_starts_at(date, start_time); no
-- index on the raw columns orders that expression.
CREATE INDEX idx_bookings_reminder_2h ON bookings (booking_starts_at(date, start_time))
    WHERE status = 'confirmed' AND reminder_sent_2h = false;

-- The expiry sweep over unpaid public bookings; the predicate is the one
-- slot_guard.go's stale-pending carve-out spells, minus the clock.
CREATE INDEX idx_bookings_expired_pending ON bookings (created_at)
    WHERE status = 'pending' AND collection_status = 'unpaid' AND created_by IS NULL;

CREATE INDEX idx_bookings_confirmed_date ON bookings (date)
    WHERE status = 'confirmed';

-- Every row is NULL except an unclaimed orphan.
CREATE INDEX idx_bookings_refund_intent ON bookings (refund_intent_at)
    WHERE refund_intent_at IS NOT NULL;

-- A minted, expiring credential for a booking's public routes (status,
-- cancel-info, cancel), hashed at rest like the identity tokens; before it,
-- whoever held the booking UUID owned the booking. 1:N because hash-at-rest
-- makes the plaintext unrecoverable, so every process that emits a link (the
-- insert, and hours later the webhook confirmation) mints its own row.
-- expires_at is stored, not derived: deriving it would read
-- complexes.cancellation_hours, which can change at any time. Deliberately no
-- "every booking has a token" trigger: production has exactly one insert path
-- and it mints inside its own transaction.
CREATE TABLE booking_link_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    booking_id  UUID NOT NULL,
    token_hash  BYTEA NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- The tenant, derived from the booking by set_complex_id below; see
    -- set_complex_id_from_booking above.
    complex_id  UUID NOT NULL,
    CONSTRAINT booking_link_tokens_token_hash_key UNIQUE (token_hash),
    CONSTRAINT booking_link_tokens_booking_id_fkey
        FOREIGN KEY (booking_id, complex_id) REFERENCES bookings (id, complex_id) ON DELETE CASCADE
);
CREATE TRIGGER set_complex_id BEFORE INSERT OR UPDATE ON booking_link_tokens
    FOR EACH ROW EXECUTE FUNCTION set_complex_id_from_booking();

CREATE INDEX idx_booking_link_tokens_booking ON booking_link_tokens (booking_id);
CREATE INDEX idx_booking_link_tokens_expires ON booking_link_tokens (expires_at);
CREATE INDEX idx_booking_link_tokens_complex ON booking_link_tokens (complex_id);

-- Short-lived holds the storefront takes while a client checks out (Redis is
-- the primary mechanism; this is the durable fallback).
CREATE TABLE slot_locks (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    court_id    UUID NOT NULL,
    date        DATE NOT NULL,
    start_time  TIME NOT NULL,
    end_time    TIME NOT NULL,
    booking_id  UUID,
    locked_by   TEXT NOT NULL DEFAULT 'public_booking',
    locked_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at  TIMESTAMPTZ NOT NULL,
    -- The tenant, derived from the court by set_complex_id below; see
    -- set_complex_id_from_court above.
    complex_id  UUID NOT NULL,
    CONSTRAINT slot_locks_court_id_date_start_time_key UNIQUE (court_id, date, start_time),
    CONSTRAINT slot_locks_court_id_fkey
        FOREIGN KEY (court_id, complex_id) REFERENCES courts (id, complex_id) ON DELETE CASCADE,
    CONSTRAINT slot_locks_booking_id_fkey
        FOREIGN KEY (booking_id) REFERENCES bookings (id) ON DELETE SET NULL
);
CREATE TRIGGER set_complex_id BEFORE INSERT OR UPDATE ON slot_locks
    FOR EACH ROW EXECUTE FUNCTION set_complex_id_from_court();

CREATE INDEX idx_slot_locks_expires ON slot_locks (expires_at);
CREATE INDEX idx_slot_locks_complex ON slot_locks (complex_id);
-- Also serves the FK probe from bookings: `booking_id = $1` implies the
-- predicate, and the planner proves it, so no unqualified twin is needed.
CREATE INDEX idx_slot_locks_booking ON slot_locks (booking_id)
    WHERE booking_id IS NOT NULL;

-- ==================== MONEY ====================

-- One row per payment event on a booking: the MercadoPago deposit, a cash
-- balance the owner confirmed, each its own row. refund_amount is NOT NULL
-- because the two report readers disagreed about NULL (one COALESCEd it to 0,
-- the other failed the monthly export with a scan error) and no Go path can
-- write one. It is bounded by amount + service_fee, not amount: a full refund
-- returns the fee too, because that is what the client actually paid, and a
-- cap of amount would refuse refunds the product intends to make.
-- uuidv7() rather than gen_random_uuid(): see the note above the bookings
-- table for why these four keys are time-ordered.
CREATE TABLE payments (
    id               UUID PRIMARY KEY DEFAULT uuidv7(),
    booking_id       UUID NOT NULL,
    complex_id       UUID NOT NULL,
    amount           INTEGER NOT NULL,
    service_fee      INTEGER NOT NULL DEFAULT 0,
    method           payment_method NOT NULL,
    status           payment_status NOT NULL DEFAULT 'unpaid',
    mp_payment_id    TEXT,
    mp_preference_id TEXT,
    refund_amount    INTEGER NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- MercadoPago's status_detail ("accredited", "cc_rejected_insufficient_
    -- amount", "partially_refunded"): the specific reason, kept rather than
    -- only logged.
    status_detail    TEXT,
    CONSTRAINT payments_booking_id_fkey
        FOREIGN KEY (booking_id) REFERENCES bookings (id),
    CONSTRAINT payments_complex_id_fkey
        FOREIGN KEY (complex_id) REFERENCES complexes (id) ON DELETE CASCADE,
    -- Every revenue query scopes by payments.complex_id. A payment whose
    -- complex_id differed from its booking's would land in one tenant's
    -- monthly total with its booking in another's: money paid to the wrong
    -- owner, and neither report showing anything unusual.
    CONSTRAINT payments_booking_in_same_complex
        FOREIGN KEY (booking_id, complex_id) REFERENCES bookings (id, complex_id),
    CONSTRAINT payments_amount_check CHECK (amount >= 0),
    CONSTRAINT payments_service_fee_check CHECK (service_fee >= 0),
    CONSTRAINT payments_refund_amount_check CHECK (refund_amount >= 0),
    CONSTRAINT payments_refund_within_amount_paid
        CHECK (refund_amount <= amount + service_fee)
);
CREATE TRIGGER set_updated_at BEFORE UPDATE ON payments
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

CREATE INDEX idx_payments_booking ON payments (booking_id);
-- One row per provider payment id; the webhook deduplicates on this.
CREATE UNIQUE INDEX idx_payments_mp_payment_id ON payments (mp_payment_id)
    WHERE mp_payment_id IS NOT NULL;
-- Revenue by local calendar day.
CREATE INDEX idx_payments_created_date_ar ON payments (
    ((created_at AT TIME ZONE 'America/Argentina/Buenos_Aires')::date)
);
CREATE INDEX idx_payments_complex_created_date ON payments (
    complex_id,
    ((created_at AT TIME ZONE 'America/Argentina/Buenos_Aires')::date)
) WHERE status != 'refunded';
-- The partial index above excludes exactly the rows a CASCADE from complexes
-- still has to reach, so the FK probe needs the unqualified twin.
CREATE INDEX idx_payments_complex_id_all ON payments (complex_id);

-- The refund queue. Since refunds became claim-first, ClaimRefund writes a row
-- BEFORE MercadoPago is called, so this holds one row per refund attempted,
-- not one per failure, and almost all of them resolve within seconds.
-- 'exhausted' rows are never deleted on a timer: each is money a client is
-- still owed.
CREATE TABLE failed_refunds (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_id    UUID NOT NULL,
    booking_id    UUID NOT NULL,
    complex_id    UUID NOT NULL,
    amount        INT NOT NULL,
    mp_payment_id TEXT,
    error_message TEXT,
    retry_count   INT NOT NULL DEFAULT 0,
    max_retries   INT NOT NULL DEFAULT 5,
    next_retry_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status        TEXT NOT NULL DEFAULT 'pending',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at   TIMESTAMPTZ,
    CONSTRAINT failed_refunds_payment_id_fkey
        FOREIGN KEY (payment_id) REFERENCES payments (id),
    CONSTRAINT failed_refunds_booking_id_fkey
        FOREIGN KEY (booking_id) REFERENCES bookings (id),
    CONSTRAINT failed_refunds_complex_id_fkey
        FOREIGN KEY (complex_id) REFERENCES complexes (id) ON DELETE CASCADE,
    CONSTRAINT failed_refunds_amount_check CHECK (amount > 0),
    CONSTRAINT failed_refunds_status_check
        CHECK (status IN ('pending', 'processing', 'resolved', 'exhausted'))
);
CREATE TRIGGER set_updated_at BEFORE UPDATE ON failed_refunds
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- The sweeper reclaims pending rows, attempts abandoned in processing, and
-- exhausted ones after a cooldown; ordered on next_retry_at, which is both
-- the filter and the ORDER BY. 'resolved' is the state almost every row ends
-- in, so keeping it out is what stops the index growing with the table.
CREATE INDEX idx_failed_refunds_pending ON failed_refunds (next_retry_at)
    WHERE status IN ('pending', 'processing', 'exhausted');
-- Retention: partial on the one terminal state that may be deleted.
CREATE INDEX idx_failed_refunds_resolved ON failed_refunds (resolved_at)
    WHERE status = 'resolved';
CREATE INDEX idx_failed_refunds_complex    ON failed_refunds (complex_id);
CREATE INDEX idx_failed_refunds_booking    ON failed_refunds (booking_id);
CREATE INDEX idx_failed_refunds_payment_id ON failed_refunds (payment_id);

-- ==================== OPERATIONS ====================

-- THE DURABLE INBOX FOR PROVIDER NOTIFICATIONS. A row lands here and commits
-- before the webhook answers 200, so a 200 means "durably recorded" rather
-- than "received", and anything that fails afterwards is retryable. Before
-- it, a transient failure after the 200 left the client's money captured, the
-- booking pending until the expiry cron cancelled it, and a log line as the
-- only trace.
--
-- (provider, external_id) is deliberately NOT unique: MercadoPago delivers
-- several notifications for one payment as its status moves, each its own
-- row. Deduplication stays where it is, the mp_webhook:<id> lease and the
-- existence check in internal/payments; this is an inbox and a forensic log,
-- not an idempotency key. Same status vocabulary and backoff columns as
-- failed_refunds, so both sweepers read the same way.
-- uuidv7() rather than gen_random_uuid(): see the note above the bookings
-- table for why these four keys are time-ordered.
CREATE TABLE webhook_events (
    id            UUID PRIMARY KEY DEFAULT uuidv7(),
    provider      TEXT NOT NULL DEFAULT 'mercadopago',
    external_id   TEXT NOT NULL,
    event_type    TEXT NOT NULL,
    payload       JSONB NOT NULL,
    status        TEXT NOT NULL DEFAULT 'pending',
    retry_count   INT NOT NULL DEFAULT 0,
    max_retries   INT NOT NULL DEFAULT 5,
    next_retry_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error    TEXT,
    received_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT webhook_events_provider_check CHECK (provider <> ''),
    CONSTRAINT webhook_events_status_check
        CHECK (status IN ('pending', 'processing', 'processed', 'exhausted')),
    CONSTRAINT webhook_events_retry_count_check CHECK (retry_count >= 0),
    CONSTRAINT webhook_events_max_retries_check CHECK (max_retries > 0)
);
CREATE TRIGGER set_updated_at BEFORE UPDATE ON webhook_events
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- Due rows still pending plus rows abandoned in processing; partial so the
-- index stays small once most of the table is processed.
CREATE INDEX idx_webhook_events_pending ON webhook_events (next_retry_at)
    WHERE status IN ('pending', 'processing');
-- "Every delivery MercadoPago made for this payment."
CREATE INDEX idx_webhook_events_external ON webhook_events (provider, external_id);
-- The retention sweep deletes by processed_at.
CREATE INDEX idx_webhook_events_processed ON webhook_events (processed_at)
    WHERE status = 'processed';

-- A LOCK THAT DOES NOT COST A CONNECTION. Session advisory locks held a pooled
-- connection for the length of a MercadoPago call; with a pool of 25 the pool
-- ran dry at about 1.9 webhooks per second and every tenant's requests failed.
-- The queues behind these locks now claim their own rows conditionally, so
-- the lock is an optimisation (stop several instances starting one sweep,
-- stop two deliveries for one payment being worked at once) and an
-- optimisation must not be able to exhaust the pool. A lease is one INSERT ON
-- CONFLICT that succeeds only when the existing lease has expired, and one
-- DELETE. A crashed holder blocks its key until the lease lapses, which is
-- bounded by the deadline of the context that took it (leaseTTL in
-- internal/data/locks.go).
CREATE TABLE job_locks (
    -- 'cron:<job>' or 'mp_webhook:<payment id>'. The primary key is what makes
    -- the conflict clause exclusive.
    key         TEXT PRIMARY KEY,
    -- Which attempt holds it: the release deletes by (key, holder), so a caller
    -- whose lease lapsed and was taken by someone else cannot delete the new
    -- holder's lock on its way out.
    holder      UUID NOT NULL,
    acquired_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at  TIMESTAMPTZ NOT NULL,
    CONSTRAINT job_locks_key_check CHECK (key <> '')
);
-- The release's second clause sweeps leases whose holder died long ago.
CREATE INDEX idx_job_locks_expires ON job_locks (expires_at);

-- ONE DURABLE WORK QUEUE. There were three, and they said the same thing three
-- ways (JOB-02): internal/notifier, a Redis list with five keys and five Lua
-- scripts of its own — a pending list, a processing list, a claims hash, a
-- delayed sorted set and a dead-letter list, none of it in Postgres, so a Redis
-- that lost its data lost every queued email; and webhook_events and
-- failed_refunds above, with the same status vocabulary, the same
-- retry_count/max_retries/next_retry_at columns and the same backoff written out
-- twice. They shared a vocabulary because they shared a guarantee. They did not
-- share a line of code, so every fix to one of them was a fix to one of them:
-- the jitter this table's writers apply (OUT-02), the dedup key that stops a
-- redelivered webhook sending a second confirmation (JOB-04), the claim that
-- does not need a lease table.
--
-- THE CLAIM. A worker takes rows with
--
--     UPDATE jobs SET status = 'processing', ...
--     WHERE id IN (SELECT id FROM jobs WHERE status = 'pending'
--                    AND run_at <= NOW() ORDER BY run_at
--                  FOR UPDATE SKIP LOCKED LIMIT $n)
--     RETURNING ...
--
-- SKIP LOCKED is the whole design. Every instance runs that statement on the
-- same tick and they take disjoint rows without one of them ever waiting on
-- another: a row somebody else has locked is passed over, not queued behind.
-- The two tables it replaces used a conditional UPDATE per row instead, which is
-- correct but serialises every instance through the same row on its way to
-- finding out it lost, and needed a job_locks lease on top to stop the convoy.
-- job_locks stays — it is a cron mutex, not a queue.
--
-- NO TENANT COLUMN AND NO ROW-LEVEL SECURITY, the same posture as job_locks and
-- webhook_events (see "TABLES THAT GET NOTHING" in the ACCESS section): a job is
-- the platform's own deferred work, claimed by a background worker that has no
-- request and therefore no tenant. Anything a job needs to touch a tenant's rows
-- with is in its payload, and the store it reaches through is under the policies
-- as usual.
CREATE TABLE jobs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- The handler registry's key: 'email:booking_confirmation' and the rest of
    -- internal/notifications' task types. It is stored, so renaming one strands
    -- whatever is already queued under the old name.
    type         TEXT NOT NULL,
    -- The handler's whole argument. jsonb rather than json so a payload can be
    -- queried by an operator reading the dead letter without parsing it first.
    payload      JSONB NOT NULL,
    status       TEXT NOT NULL DEFAULT 'pending',
    -- The earliest instant a worker may claim this row: now for an ordinary
    -- enqueue, and the backoff for every retry.
    run_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- attempts counts claims, not failures — Claim increments it — so a handler
    -- that takes the process down with it has still spent one. That is what
    -- bounds a payload that kills whichever instance reads it.
    attempts     INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 5,
    last_error   TEXT,
    -- The claim itself. locked_at is what the stale sweep measures the lease
    -- against; locked_by names the worker for the line that says which one went
    -- away.
    locked_at    TIMESTAMPTZ,
    locked_by    TEXT,
    -- The idempotency key, nullable because most work does not need one. See
    -- the unique index below for why it is not a UNIQUE constraint.
    dedup_key    TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT jobs_type_check CHECK (type <> ''),
    CONSTRAINT jobs_status_check
        CHECK (status IN ('pending', 'processing', 'done', 'failed')),
    CONSTRAINT jobs_attempts_check CHECK (attempts >= 0),
    CONSTRAINT jobs_max_attempts_check CHECK (max_attempts > 0),
    -- A dedup key of '' is a caller that meant NULL and got the zero value.
    -- Every such caller would collide with every other, which is the worst
    -- possible reading of "no key".
    CONSTRAINT jobs_dedup_key_check CHECK (dedup_key IS NULL OR dedup_key <> '')
);
CREATE TRIGGER set_updated_at BEFORE UPDATE ON jobs
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- THE CLAIM INDEX. Exactly the predicate and the order of the inner SELECT
-- above, and partial on the one status a claim can match — 'done' is where
-- almost every row ends up, so keeping it out is what stops the index growing
-- with the table the way the two it replaces did.
CREATE INDEX idx_jobs_claim ON jobs (run_at) WHERE status = 'pending';

-- The stale sweep: rows claimed longer ago than the lease.
CREATE INDEX idx_jobs_stale ON jobs (locked_at) WHERE status = 'processing';

-- DEDUP. A partial unique index rather than a UNIQUE constraint, because the
-- column is nullable and most rows have no key: Postgres treats NULLs as
-- distinct, so a plain unique index would work, but it would also index every
-- keyless row for nothing. ON CONFLICT (dedup_key) DO NOTHING infers this index,
-- which is what makes a second Enqueue under the same key a no-op rather than a
-- second email.
CREATE UNIQUE INDEX idx_jobs_dedup_key ON jobs (dedup_key) WHERE dedup_key IS NOT NULL;

-- Retention deletes by the instant a row was closed out, which for a done row is
-- its last update.
CREATE INDEX idx_jobs_done ON jobs (updated_at) WHERE status = 'done';

COMMENT ON TABLE jobs IS
    'The durable work queue. Claimed with SELECT ... FOR UPDATE SKIP LOCKED; see internal/jobs.';
COMMENT ON COLUMN jobs.dedup_key IS
    'sha256 of the job type and what identifies the work (recipient, booking, event). A second enqueue under an existing key does nothing.';
COMMENT ON COLUMN jobs.attempts IS
    'Claims, not failures: incremented by the claim itself, so a handler that kills the process still spends one.';

-- Append-only and unpruned; the table most likely to grow large. Both foreign
-- keys are ON DELETE SET NULL because a trail that refuses the deletion of the
-- account or venue it witnesses is the opposite of a trail: the entry outlives
-- both, entity_id (no FK) still names the thing acted on, and the actor is
-- named in the value. CASCADE would delete a departing account's history,
-- which is what "who cancelled that booking?" is asked about.
-- uuidv7() rather than gen_random_uuid(): see the note above the bookings
-- table for why these four keys are time-ordered. audit_log is the table most
-- likely to grow large, and it is read by time more than by anything else.
CREATE TABLE audit_log (
    id          UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id     UUID,
    complex_id  UUID,
    action      TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id   UUID,
    old_value   JSONB,
    new_value   JSONB,
    ip_address  INET,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT audit_log_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE SET NULL,
    CONSTRAINT audit_log_complex_id_fkey
        FOREIGN KEY (complex_id) REFERENCES complexes (id) ON DELETE SET NULL
);

-- Both list views page by keyset on (created_at, id) DESC. Carrying id and
-- matching the direction exactly lets the planner drop the sort node and turn
-- the row-comparison cursor into a single Index Cond; measured at 500,000
-- rows the unscoped platform view went from a 1.6s seq scan to 0.6ms.
CREATE INDEX idx_audit_log_created_at ON audit_log (created_at DESC, id DESC);
CREATE INDEX idx_audit_log_complex    ON audit_log (complex_id, created_at DESC, id DESC);
CREATE INDEX idx_audit_log_entity     ON audit_log (entity_type, entity_id);
-- The FK probe on account deletion; without it DeleteUser seq-scans the whole
-- trail while holding locks.
CREATE INDEX idx_audit_log_user_id    ON audit_log (user_id);

-- ==================== ACCESS: ROLES, OWNERSHIP, GRANTS, ROW-LEVEL SECURITY ====================

-- TWO ROLES, AND THE APPLICATION IS NEITHER THE OWNER NOR A SUPERUSER.
--
--   vibe_migrator  owns every table, view, sequence, function and type in
--                  schema public; what goose and the embedded migrator connect
--                  as (DB_MIGRATOR_URL). Not a superuser: DDL here, nothing else.
--   vibe_app       what the API connects as (DATABASE_URL). SELECT/INSERT/
--                  UPDATE/DELETE on tables and views, USAGE on sequences,
--                  EXECUTE on functions, USAGE on the schema. NOSUPERUSER and
--                  NOBYPASSRLS, so the policies below bind to it.
--
-- Until the deploy points those two variables at these roles, both are
-- latent: a superuser is exempt from grants and carries rolbypassrls, so the
-- existing suites keep running unchanged. A read-only third role was left out
-- because nothing in this system reads without writing.
--
-- Roles are CLUSTER-scoped and this file is DATABASE-scoped. Every second
-- database on an instance (the e2e database, a scratch one, a restore) finds
-- the roles already there, so CREATE ROLE is guarded by a catalog check and
-- everything after it is a GRANT or an ALTER that can be issued twice. The
-- roles are created NOLOGIN with no password; the deploy issues
-- ALTER ROLE ... LOGIN PASSWORD out of band, once per cluster (the Makefile's
-- e2e-db-roles target does it for the test cluster). A password in a
-- migration is a password in git and in the server log; a GUC carrying one is
-- visible in pg_settings to every session on the connection, and the embedded
-- migrator has no hook to set it anyway. Setting only the attributes that
-- decide whether RLS is enforcement or decoration is deliberate: LOGIN and the
-- password belong to the operator.
--
-- The Down NEVER drops the roles, however well guarded. A per-database
-- rollback must not have cluster-wide effects: an earlier draft dropped them
-- when nothing else referenced them, and a rollback on one database took the
-- roles out from under another database's running suite within the hour. The
-- Down returns what this database gave (ownership, grants, default
-- privileges, DROP OWNED BY) and leaves two role names with no privileges,
-- which costs nothing; removing them is three hand-run statements.

-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'vibe_migrator') THEN
        CREATE ROLE vibe_migrator
            NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'vibe_app') THEN
        CREATE ROLE vibe_app
            NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
    END IF;
END;
$$;
-- +goose StatementEnd

-- Pinned on roles that may predate this file. NOBYPASSRLS is the one that
-- matters; NOSUPERUSER is stated because a superuser ignores NOBYPASSRLS.
ALTER ROLE vibe_app NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;
ALTER ROLE vibe_migrator NOSUPERUSER NOBYPASSRLS;

COMMENT ON ROLE vibe_migrator IS
    'Owns schema public. The role goose and the embedded migrator connect as (DB_MIGRATOR_URL). Not a superuser: DDL on this schema and nothing else.';
COMMENT ON ROLE vibe_app IS
    'The role the API connects as (DATABASE_URL). DML only, NOBYPASSRLS, so the tenant policies in db/migrations/001_init.sql apply to it.';

-- OWNERSHIP moves to vibe_migrator by walking the catalog rather than by a
-- list: it is idempotent, it cannot go stale when an object is added above,
-- and it is the only reliable way to skip what belongs to an extension
-- (citext's and btree_gist's 240-odd functions, types and operators, named by
-- pg_depend deptype 'e'). Tables and sequences move before views, because
-- ALTER VIEW ... OWNER checks the new owner can read what the view selects
-- from. goose's own table moves with the rest: bookkeeping is the migrator's.

-- +goose StatementBegin
DO $$
DECLARE
    obj record;
BEGIN
    FOR obj IN
        SELECT c.oid::regclass::text AS ident,
               CASE c.relkind
                   WHEN 'S' THEN 'SEQUENCE'
                   WHEN 'v' THEN 'VIEW'
                   WHEN 'm' THEN 'MATERIALIZED VIEW'
                   WHEN 'f' THEN 'FOREIGN TABLE'
                   ELSE 'TABLE'
               END AS kind,
               CASE c.relkind WHEN 'v' THEN 2 WHEN 'm' THEN 2 ELSE 1 END AS phase
          FROM pg_class c
          JOIN pg_namespace n ON n.oid = c.relnamespace
         WHERE n.nspname = 'public'
           AND c.relkind IN ('r', 'p', 'S', 'v', 'm', 'f')
           AND NOT c.relispartition
           AND pg_get_userbyid(c.relowner) <> 'vibe_migrator'
           AND NOT EXISTS (
               SELECT 1 FROM pg_depend d
                WHERE d.classid = 'pg_class'::regclass
                  AND d.objid = c.oid
                  AND d.deptype = 'e')
         ORDER BY phase, ident
    LOOP
        EXECUTE format('ALTER %s %s OWNER TO vibe_migrator', obj.kind, obj.ident);
    END LOOP;

    FOR obj IN
        SELECT p.oid::regprocedure::text AS ident,
               CASE p.prokind
                   WHEN 'a' THEN 'AGGREGATE'
                   WHEN 'p' THEN 'PROCEDURE'
                   ELSE 'FUNCTION'
               END AS kind
          FROM pg_proc p
          JOIN pg_namespace n ON n.oid = p.pronamespace
         WHERE n.nspname = 'public'
           AND pg_get_userbyid(p.proowner) <> 'vibe_migrator'
           AND NOT EXISTS (
               SELECT 1 FROM pg_depend d
                WHERE d.classid = 'pg_proc'::regclass
                  AND d.objid = p.oid
                  AND d.deptype = 'e')
    LOOP
        EXECUTE format('ALTER %s %s OWNER TO vibe_migrator', obj.kind, obj.ident);
    END LOOP;

    -- Enumerations, domains, ranges and standalone composites. Array and
    -- multirange types follow the type they derive from; a table's row type
    -- follows the table.
    FOR obj IN
        SELECT t.oid::regtype::text AS ident
          FROM pg_type t
          JOIN pg_namespace n ON n.oid = t.typnamespace
         WHERE n.nspname = 'public'
           AND t.typtype IN ('e', 'd', 'r', 'c')
           AND t.typcategory <> 'A'
           AND (t.typrelid = 0
                OR EXISTS (SELECT 1 FROM pg_class c
                            WHERE c.oid = t.typrelid AND c.relkind = 'c'))
           AND pg_get_userbyid(t.typowner) <> 'vibe_migrator'
           AND NOT EXISTS (
               SELECT 1 FROM pg_depend d
                WHERE d.classid = 'pg_type'::regclass
                  AND d.objid = t.oid
                  AND d.deptype = 'e')
    LOOP
        EXECUTE format('ALTER TYPE %s OWNER TO vibe_migrator', obj.ident);
    END LOOP;
END;
$$;
-- +goose StatementEnd

-- On PostgreSQL 15+ CREATE on public is no longer granted to PUBLIC, so the
-- owner needs it explicitly or the next migration's CREATE TABLE fails the day
-- DB_MIGRATOR_URL stops naming a superuser.
GRANT USAGE, CREATE ON SCHEMA public TO vibe_migrator;

-- DML on every table and view, and nothing else: no TRUNCATE, REFERENCES or
-- TRIGGER. goose's bookkeeping is revoked: the only Go that reads it is
-- internal/migrate, on DB_MIGRATOR_URL.
GRANT USAGE ON SCHEMA public TO vibe_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO vibe_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO vibe_app;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO vibe_app;

REVOKE ALL ON TABLE goose_db_version FROM vibe_app;
REVOKE ALL ON SEQUENCE goose_db_version_id_seq FROM vibe_app;

-- Default privileges are per creating role, and two roles can create a table
-- here: vibe_migrator after the cutover, and whoever runs migrations today (a
-- superuser). Both are registered, or a migration applied before the cutover
-- creates a table vibe_app cannot read and the symptom shows on the deploy
-- after the one that caused it.
-- +goose StatementBegin
DO $$
DECLARE
    creator text;
BEGIN
    FOREACH creator IN ARRAY ARRAY['vibe_migrator', current_user] LOOP
        EXECUTE format(
            'ALTER DEFAULT PRIVILEGES FOR ROLE %I IN SCHEMA public '
            'GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO vibe_app', creator);
        EXECUTE format(
            'ALTER DEFAULT PRIVILEGES FOR ROLE %I IN SCHEMA public '
            'GRANT USAGE, SELECT ON SEQUENCES TO vibe_app', creator);
        EXECUTE format(
            'ALTER DEFAULT PRIVILEGES FOR ROLE %I IN SCHEMA public '
            'GRANT EXECUTE ON FUNCTIONS TO vibe_app', creator);
    END LOOP;
END;
$$;
-- +goose StatementEnd

-- ROW-LEVEL SECURITY ON EVERY TENANT-SCOPED TABLE.
--
-- One tenant's rows were kept from another by a line of Go repeated by hand
-- in twelve handlers (`if booking.ComplexID != complex.ID { NotFound }`) while
-- fifteen queries fetch by primary key with no tenant predicate at all. The
-- objection is not that somebody forgot the line; it is that forgetting it is
-- silent. Now the database refuses: app.complex_id is a session setting the
-- application stamps from the request context (internal/data/tenant.go, the
-- pool's PrepareConn hook and DB.Begin's SET LOCAL), and a row whose
-- complex_id does not match it does not exist for SELECT, UPDATE and DELETE,
-- while an INSERT naming another tenant is refused by WITH CHECK. The Go
-- comparison stays: two walls, the handler's 404 with a human sentence and
-- the database's "no rows" for the case the handler never considered.
--
-- ENABLE and FORCE, so the policies bind to the schema owner as well as to
-- vibe_app; only a genuine superuser (the operator's psql) reads across
-- tenants. FORCE is also what every later data migration pays for, with the
-- bypass line in the header.
--
-- THE PREDICATE: complex_id = nullif(current_setting('app.complex_id', true), '')::uuid.
--   * missing_ok = true: an unstamped session gets NULL, not 42704.
--   * nullif(..., ''): a connection returned to the pool is re-stamped by the
--     next borrower with '' when it has no tenant; ''::uuid would be 22P02.
--   * NULL = uuid is NULL, not TRUE: a session with no tenant matches nothing.
--     That is the fail-closed default, and why a path the application forgot
--     to stamp breaks visibly in tests instead of leaking in production.
-- Written out in each policy rather than in a helper: a SQL function pinning
-- its search_path cannot be inlined and would run once per row, and a policy
-- expression is stored already parsed, so the search_path question does not
-- arise.
--
-- THE SECOND POLICY, tenant_bypass, is current_setting('app.bypass_tenant',
-- true) = 'on'. Permissive policies are OR-ed, so a scoped session sees its
-- own rows and a bypassed one sees everything. The application sets it at
-- exactly the boundaries that legitimately span tenants or run before the
-- tenant is known — the cron sweeps, the superadmin console, and resolving
-- the tenant itself (the complex from the URL, the storefront from a slug,
-- the public cancel link from a token hash, the webhook from a payment id) —
-- through one route table with a reason per entry (internal/middleware).
-- It is a defence against a forgotten WHERE, not against a compromised
-- vibe_app, which can set any custom GUC; the pool's extended protocol is
-- what keeps a smuggled parameter from appending a SET.
--
-- TABLES THAT GET NOTHING: users, user_identities, refresh_tokens,
-- email_verification_tokens, password_reset_tokens (accounts, not tenant data —
-- authentication runs before any tenant is known and a policy here means nobody
-- can log in); job_locks (cluster-wide cron leases with no tenant by design);
-- jobs (the platform's own deferred work, claimed by a worker that has no
-- request and therefore no tenant); webhook_events (the raw provider envelope
-- stored before anything parsed it; the tenant boundary for that path is on
-- payments, which the sweep joins through); goose_db_version (revoked above).
--
-- EVERY POLICY BELOW IS ONE COLUMN COMPARISON, INCLUDING THE FOUR CHILD TABLES.
-- court_prices, blocked_slots and slot_locks hang off a court and
-- booking_link_tokens off a booking, so each could reach its tenant through its
-- parent with an EXISTS subquery instead — correct, and a primary-key lookup the
-- planner runs as a semi-join, but the only policies in the schema that had to
-- be read twice to be believed, and a semi-join per row where the others cost a
-- column comparison. They carry their own complex_id (see the four tables and
-- set_complex_id_from_court / set_complex_id_from_booking above), which is a
-- stored derivable value and pays the full price of one: a composite foreign key
-- so it cannot contradict the parent, and a trigger that fills it so no caller
-- has to and none can get it wrong.
--
-- audit_log.complex_id is nullable (ON DELETE SET NULL): such a row belongs to
-- no tenant, matches no isolation policy, and is the operator's record,
-- reachable through the superadmin console under the bypass.

-- complexes: the tenant table itself, keyed on its own primary key.
ALTER TABLE complexes ENABLE ROW LEVEL SECURITY;
ALTER TABLE complexes FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON complexes
    USING (id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON complexes
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

ALTER TABLE courts ENABLE ROW LEVEL SECURITY;
ALTER TABLE courts FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON courts
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON courts
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

ALTER TABLE clients ENABLE ROW LEVEL SECURITY;
ALTER TABLE clients FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON clients
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON clients
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

ALTER TABLE bookings ENABLE ROW LEVEL SECURITY;
ALTER TABLE bookings FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON bookings
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON bookings
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

ALTER TABLE payments ENABLE ROW LEVEL SECURITY;
ALTER TABLE payments FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON payments
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON payments
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

ALTER TABLE complex_schedules ENABLE ROW LEVEL SECURITY;
ALTER TABLE complex_schedules FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON complex_schedules
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON complex_schedules
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

ALTER TABLE failed_refunds ENABLE ROW LEVEL SECURITY;
ALTER TABLE failed_refunds FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON failed_refunds
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON failed_refunds
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

ALTER TABLE audit_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_log FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON audit_log
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON audit_log
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

ALTER TABLE court_prices ENABLE ROW LEVEL SECURITY;
ALTER TABLE court_prices FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON court_prices
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON court_prices
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

ALTER TABLE blocked_slots ENABLE ROW LEVEL SECURITY;
ALTER TABLE blocked_slots FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON blocked_slots
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON blocked_slots
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

ALTER TABLE slot_locks ENABLE ROW LEVEL SECURITY;
ALTER TABLE slot_locks FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON slot_locks
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON slot_locks
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

-- Resolved by token hash on a public route with no owner and no tenant yet;
-- that path runs under the bypass, and the isolation policy covers the
-- owner-side reads and the sweep of expired tokens.
ALTER TABLE booking_link_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE booking_link_tokens FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON booking_link_tokens
    USING (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid)
    WITH CHECK (complex_id = nullif(current_setting('app.complex_id', true), '')::uuid);
CREATE POLICY tenant_bypass ON booking_link_tokens
    USING (current_setting('app.bypass_tenant', true) = 'on')
    WITH CHECK (current_setting('app.bypass_tenant', true) = 'on');

-- THE TWO VIEWS BECOME security_invoker, WHICH IS NOT OPTIONAL. A view runs
-- with its owner's privileges, the owner is vibe_migrator, and most of the
-- public and owner-facing reads of complexes and courts go through these two;
-- without this every read through them would return every tenant's rows to
-- any caller allowed to read the view. security_invoker (PostgreSQL 15+)
-- evaluates the view's query as the caller, so the policies on the tables
-- apply; the caller needs SELECT on the tables, granted above.
ALTER VIEW active_complexes SET (security_invoker = true);
ALTER VIEW active_courts SET (security_invoker = true);

-- +goose Down

SET LOCAL lock_timeout = '3s';

-- Everything the Up created, in reverse dependency order. Policies and
-- triggers go with their tables; the multirange goes with timerange. After
-- this the database holds nothing but goose's own bookkeeping table, owned by
-- the role running this Down again, and the two roles keep existing with no
-- privileges in this database (see the ACCESS section for why they are never
-- dropped here).

ALTER VIEW active_courts RESET (security_invoker);
ALTER VIEW active_complexes RESET (security_invoker);

DROP VIEW active_courts;
DROP VIEW active_complexes;

DROP TABLE audit_log;
DROP TABLE jobs;
DROP TABLE job_locks;
DROP TABLE webhook_events;
DROP TABLE failed_refunds;
DROP TABLE payments;
DROP TABLE slot_locks;
DROP TABLE booking_link_tokens;
DROP TABLE bookings;
DROP TABLE clients;
DROP TABLE blocked_slots;
DROP TABLE court_prices;
DROP TABLE courts;
DROP TABLE complex_schedules;
DROP TABLE complexes;
DROP TABLE password_reset_tokens;
DROP TABLE email_verification_tokens;
DROP TABLE refresh_tokens;
DROP TABLE user_identities;
DROP TABLE users;

DROP FUNCTION set_complex_id_from_booking();
DROP FUNCTION set_complex_id_from_court();
DROP FUNCTION bookings_forbid_status_reversal();
DROP FUNCTION courts_forbid_live_under_deleted_complex();
DROP FUNCTION complexes_cascade_soft_delete_to_courts();
DROP FUNCTION local_day(date);
DROP FUNCTION booking_starts_at(date, time);
DROP FUNCTION trigger_bump_version();
DROP FUNCTION trigger_set_updated_at();

DROP TYPE timerange;
DROP TYPE day_of_week;
DROP TYPE sport_type;
DROP TYPE court_type;
DROP TYPE payment_method;
DROP TYPE payment_status;
DROP TYPE booking_status;
DROP TYPE user_role;

DROP EXTENSION btree_gist;
DROP EXTENSION citext;

-- The ACCESS section, undone: ownership of what is left (goose's table and its
-- sequence) back to the role running this, the default privileges and the
-- schema grants revoked, and DROP OWNED BY vibe_app — per-database, the exact
-- inverse of the GRANTs. If this Down runs as vibe_migrator itself there is no
-- other owner to name, and it says so instead of guessing.

-- +goose StatementBegin
DO $$
DECLARE
    obj record;
    target text := current_user;
BEGIN
    IF target = 'vibe_migrator' THEN
        RAISE NOTICE 'ownership left with vibe_migrator: this Down is running as vibe_migrator itself and has no other owner to name';
    ELSE
        FOR obj IN
            SELECT c.oid::regclass::text AS ident,
                   CASE c.relkind WHEN 'S' THEN 'SEQUENCE' ELSE 'TABLE' END AS kind
              FROM pg_class c
              JOIN pg_namespace n ON n.oid = c.relnamespace
             WHERE n.nspname = 'public'
               AND c.relkind IN ('r', 'S')
               AND pg_get_userbyid(c.relowner) = 'vibe_migrator'
             ORDER BY ident
        LOOP
            EXECUTE format('ALTER %s %s OWNER TO %I', obj.kind, obj.ident, target);
        END LOOP;
    END IF;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
DO $$
DECLARE
    creator text;
BEGIN
    FOREACH creator IN ARRAY ARRAY['vibe_migrator', current_user] LOOP
        EXECUTE format(
            'ALTER DEFAULT PRIVILEGES FOR ROLE %I IN SCHEMA public '
            'REVOKE SELECT, INSERT, UPDATE, DELETE ON TABLES FROM vibe_app', creator);
        EXECUTE format(
            'ALTER DEFAULT PRIVILEGES FOR ROLE %I IN SCHEMA public '
            'REVOKE USAGE, SELECT ON SEQUENCES FROM vibe_app', creator);
        EXECUTE format(
            'ALTER DEFAULT PRIVILEGES FOR ROLE %I IN SCHEMA public '
            'REVOKE EXECUTE ON FUNCTIONS FROM vibe_app', creator);
    END LOOP;
END;
$$;
-- +goose StatementEnd

REVOKE ALL ON ALL FUNCTIONS IN SCHEMA public FROM vibe_app;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM vibe_app;
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM vibe_app;
REVOKE USAGE ON SCHEMA public FROM vibe_app;
REVOKE USAGE, CREATE ON SCHEMA public FROM vibe_migrator;

-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'vibe_app') THEN
        EXECUTE 'DROP OWNED BY vibe_app';
    END IF;
END;
$$;
-- +goose StatementEnd
