package services

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/groot34/Authix/internal/repositories"
)

var (
	ErrInvalidPhone           = errors.New("services: invalid phone")
	ErrInvalidShippingLine1   = errors.New("services: invalid shipping line 1")
	ErrInvalidShippingCity    = errors.New("services: invalid shipping city")
	ErrInvalidShippingPostal  = errors.New("services: invalid shipping postal code")
	ErrInvalidShippingRegion  = errors.New("services: invalid shipping region")
	ErrInvalidShippingCountry = errors.New("services: invalid shipping country code")
	ErrInvalidUserID          = errors.New("services: invalid user id")
)

var (
	phonePattern  = regexp.MustCompile(`^\+?[0-9][0-9 ()-]*[0-9]$`)
	namePattern   = regexp.MustCompile(`^[\p{L}]+(?:[ .'-][\p{L}]+)*$`)
	postalPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 -]{1,10}[A-Za-z0-9]$`)
)

type CheckoutSubmission struct {
	UserID               *int64
	Email                string
	Phone                string
	ShippingAddressLine1 string
	ShippingAddressLine2 *string
	ShippingCity         string
	ShippingPostalCode   string
	ShippingRegion       string
	ShippingCountryCode  string
}

type CheckoutReceipt struct {
	ID        int64
	UserID    *int64
	Email     string
	CreatedAt time.Time
}

type checkoutRepository interface {
	Insert(ctx context.Context, c *repositories.Checkout) (*repositories.Checkout, error)
}

type CheckoutService struct {
	checkouts checkoutRepository
}

func NewCheckoutService(checkouts checkoutRepository) *CheckoutService {
	return &CheckoutService{checkouts: checkouts}
}

func validateCheckout(in *CheckoutSubmission) error {
	if in == nil {
		return errors.New("services: checkout submission is nil")
	}
	if !looksLikeEmail(in.Email) {
		return ErrInvalidEmail
	}
	phone := trimSpace(in.Phone)
	if !phonePattern.MatchString(phone) || countDigits(phone) < 7 || countDigits(phone) > 15 {
		return ErrInvalidPhone
	}
	if trimSpace(in.ShippingAddressLine1) == "" {
		return ErrInvalidShippingLine1
	}
	if !validPlaceName(in.ShippingCity) {
		return ErrInvalidShippingCity
	}
	postal := trimSpace(in.ShippingPostalCode)
	if !postalPattern.MatchString(postal) {
		return ErrInvalidShippingPostal
	}
	if !validPlaceName(in.ShippingRegion) {
		return ErrInvalidShippingRegion
	}
	country := trimSpace(in.ShippingCountryCode)
	if len(country) != 2 {
		return ErrInvalidShippingCountry
	}
	for i := 0; i < len(country); i++ {
		c := country[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
			return ErrInvalidShippingCountry
		}
	}
	if in.UserID != nil && *in.UserID <= 0 {
		return ErrInvalidUserID
	}
	return nil
}

func countDigits(value string) int {
	count := 0
	for _, char := range value {
		if char >= '0' && char <= '9' {
			count++
		}
	}
	return count
}

func validPlaceName(value string) bool {
	return namePattern.MatchString(trimSpace(value))
}

func (s *CheckoutService) Submit(ctx context.Context, in *CheckoutSubmission) (*CheckoutReceipt, error) {
	if err := validateCheckout(in); err != nil {
		return nil, err
	}
	country := strings.ToUpper(trimSpace(in.ShippingCountryCode))
	line2 := in.ShippingAddressLine2
	if line2 != nil {
		t := trimSpace(*line2)
		if t == "" {
			line2 = nil
		} else {
			line2 = &t
		}
	}
	repoCheckout := &repositories.Checkout{
		UserID:               in.UserID,
		Email:                normalizeEmail(in.Email),
		Phone:                trimSpace(in.Phone),
		ShippingAddressLine1: trimSpace(in.ShippingAddressLine1),
		ShippingAddressLine2: line2,
		ShippingCity:         trimSpace(in.ShippingCity),
		ShippingPostalCode:   trimSpace(in.ShippingPostalCode),
		ShippingRegion:       trimSpace(in.ShippingRegion),
		ShippingCountryCode:  country,
	}
	created, err := s.checkouts.Insert(ctx, repoCheckout)
	if err != nil {
		return nil, fmt.Errorf("services: checkout submit: %w", err)
	}
	return &CheckoutReceipt{
		ID:        created.ID,
		UserID:    created.UserID,
		Email:     created.Email,
		CreatedAt: created.CreatedAt,
	}, nil
}
