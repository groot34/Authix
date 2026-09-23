package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/authix/authix/internal/database"
)

type Checkout struct {
	ID                   int64
	UserID               *int64
	Email                string
	Phone                string
	ShippingAddressLine1 string
	ShippingAddressLine2 *string
	ShippingCity         string
	ShippingPostalCode   string
	ShippingRegion       string
	ShippingCountryCode  string
	CreatedAt            time.Time
}

type CheckoutRepository struct {
	pool database.Pool
}

func NewCheckoutRepository(pool database.Pool) *CheckoutRepository {
	return &CheckoutRepository{pool: pool}
}

func (r *CheckoutRepository) Insert(ctx context.Context, c *Checkout) (*Checkout, error) {
	const q = `
INSERT INTO checkouts (
    user_id, email, phone,
    shipping_address_line1, shipping_address_line2,
    shipping_city, shipping_postal_code, shipping_region, shipping_country_code
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, user_id, email, phone,
          shipping_address_line1, shipping_address_line2,
          shipping_city, shipping_postal_code, shipping_region, shipping_country_code,
          created_at;`

	var userID sql.Null[int64]
	var line2 sql.Null[string]
	out := &Checkout{}
	err := r.pool.DB().QueryRowContext(ctx, q,
		nullableUserID(c.UserID),
		c.Email,
		c.Phone,
		c.ShippingAddressLine1,
		nullableString(c.ShippingAddressLine2),
		c.ShippingCity,
		c.ShippingPostalCode,
		c.ShippingRegion,
		c.ShippingCountryCode,
	).Scan(
		&out.ID,
		&userID,
		&out.Email,
		&out.Phone,
		&out.ShippingAddressLine1,
		&line2,
		&out.ShippingCity,
		&out.ShippingPostalCode,
		&out.ShippingRegion,
		&out.ShippingCountryCode,
		&out.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("repositories: insert checkout: %w", err)
	}
	if userID.Valid {
		v := userID.V
		out.UserID = &v
	}
	if line2.Valid {
		v := line2.V
		out.ShippingAddressLine2 = &v
	}
	return out, nil
}

func nullableUserID(v *int64) sql.Null[int64] {
	if v == nil {
		return sql.Null[int64]{}
	}
	return sql.Null[int64]{V: *v, Valid: true}
}

func nullableString(v *string) sql.Null[string] {
	if v == nil {
		return sql.Null[string]{}
	}
	return sql.Null[string]{V: *v, Valid: true}
}
