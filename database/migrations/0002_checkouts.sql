-- Migration 0002: checkout submissions table
-- Run against the `authix` PostgreSQL database, after 0001.

CREATE TABLE IF NOT EXISTS checkouts (
    id                       BIGSERIAL PRIMARY KEY,

    -- Set only when a recognised user completed the OTP flow before
    -- submitting. NULL for a guest checkout. On user DELETE we keep the
    -- checkout record (ON DELETE SET NULL) because it's the historical
    -- submission record.
    user_id                  BIGINT       NULL  REFERENCES users(id)
                                               ON DELETE SET NULL,

    -- Email, phone and shipping are stored directly on the submission so
    -- a guest checkout (user_id IS NULL) still has the full record we
    -- need, and so we keep the values the user actually submitted even
    -- if the user later changes their profile fields.
    email                    CITEXT       NOT NULL,
    phone                    TEXT         NOT NULL,

    shipping_address_line1   TEXT         NOT NULL,
    shipping_address_line2   TEXT         NULL,
    shipping_city            TEXT         NOT NULL,
    shipping_postal_code     TEXT         NOT NULL,
    shipping_region          TEXT         NOT NULL,
    shipping_country_code    CHAR(2)      NOT NULL,

    created_at               TIMESTAMPTZ  NOT NULL  DEFAULT now(),

    CONSTRAINT checkouts_email_nonempty CHECK (length(trim(email::text)) > 0),
    CONSTRAINT checkouts_phone_nonempty CHECK (length(trim(phone)) > 0),
    CONSTRAINT checkouts_shipping_line1_nonempty CHECK (length(trim(shipping_address_line1)) > 0),
    CONSTRAINT checkouts_shipping_city_nonempty CHECK (length(trim(shipping_city)) > 0),
    CONSTRAINT checkouts_shipping_postal_code_nonempty CHECK (length(trim(shipping_postal_code)) > 0),
    CONSTRAINT checkouts_shipping_region_nonempty CHECK (length(trim(shipping_region)) > 0),
    CONSTRAINT checkouts_shipping_country_code_nonempty CHECK (length(trim(shipping_country_code)) = 2)
);

CREATE INDEX IF NOT EXISTS checkouts_user_id_idx  ON checkouts (user_id);
CREATE INDEX IF NOT EXISTS checkouts_email_idx    ON checkouts (email);
CREATE INDEX IF NOT EXISTS checkouts_created_at_idx ON checkouts (created_at DESC);
