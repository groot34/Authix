package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/authix/authix/internal/repositories"
)

type fakeCheckoutRepo struct {
	insertFn func(ctx context.Context, c *repositories.Checkout) (*repositories.Checkout, error)
	lastArg  *repositories.Checkout
}

func (f *fakeCheckoutRepo) Insert(ctx context.Context, c *repositories.Checkout) (*repositories.Checkout, error) {
	f.lastArg = c
	if f.insertFn != nil {
		return f.insertFn(ctx, c)
	}
	out := *c
	out.ID = 42
	out.CreatedAt = time.Date(2026, 9, 23, 15, 0, 0, 0, time.UTC)
	return &out, nil
}

func guestSubmission() *CheckoutSubmission {
	return &CheckoutSubmission{
		Email:                "guest@example.com",
		Phone:                "+1555 0100",
		ShippingAddressLine1: "  99 Guest Ln  ",
		ShippingCity:         "  Guest Town ",
		ShippingPostalCode:   "99999",
		ShippingRegion:       "NY",
		ShippingCountryCode:  "us",
	}
}

func TestCheckoutService_SubmitGuest(t *testing.T) {
	repo := &fakeCheckoutRepo{}
	svc := NewCheckoutService(repo)
	r, err := svc.Submit(context.Background(), guestSubmission())
	if err != nil {
		t.Fatalf("Submit err: %v", err)
	}
	if r == nil || r.ID != 42 {
		t.Errorf("receipt want id=42 got %+v", r)
	}
	if r.UserID != nil {
		t.Errorf("guest checkout user_id should be nil, got %v", *r.UserID)
	}
	if r.Email != "guest@example.com" {
		t.Errorf("email not normalised: got %q", r.Email)
	}
	if !r.CreatedAt.IsZero() == false {
		t.Errorf("createdAt should be set")
	}
	got := repo.lastArg
	if got == nil {
		t.Fatal("repo.Insert not called")
	}
	if got.UserID != nil {
		t.Errorf("repo user_id want nil got %v", got.UserID)
	}
	if got.ShippingAddressLine1 != "99 Guest Ln" {
		t.Errorf("line1 not trimmed: %q", got.ShippingAddressLine1)
	}
	if got.ShippingCountryCode != "US" {
		t.Errorf("country code not uppercased: %q", got.ShippingCountryCode)
	}
}

func TestCheckoutService_SubmitLoggedInUser(t *testing.T) {
	repo := &fakeCheckoutRepo{}
	svc := NewCheckoutService(repo)
	line2 := "  Apt 4B  "
	userID := int64(7)
	in := guestSubmission()
	in.UserID = &userID
	in.ShippingAddressLine2 = &line2
	r, err := svc.Submit(context.Background(), in)
	if err != nil {
		t.Fatalf("Submit err: %v", err)
	}
	if r.UserID == nil || *r.UserID != 7 {
		t.Errorf("user_id want 7 got %v", r.UserID)
	}
	got := repo.lastArg
	if got == nil {
		t.Fatal("repo.Insert not called")
	}
	if got.UserID == nil || *got.UserID != 7 {
		t.Errorf("repo user_id want 7 got %v", got.UserID)
	}
	if got.ShippingAddressLine2 == nil || *got.ShippingAddressLine2 != "Apt 4B" {
		t.Errorf("line2 not trimmed: got %+v", got.ShippingAddressLine2)
	}
}

func TestCheckoutService_EmptyOptionalLine2_BecomesNil(t *testing.T) {
	repo := &fakeCheckoutRepo{}
	svc := NewCheckoutService(repo)
	line2 := "   \t\n  "
	in := guestSubmission()
	in.ShippingAddressLine2 = &line2
	if _, err := svc.Submit(context.Background(), in); err != nil {
		t.Fatalf("submit err: %v", err)
	}
	if repo.lastArg.ShippingAddressLine2 != nil {
		t.Errorf("whitespace-only line2 should become nil, got %q", *repo.lastArg.ShippingAddressLine2)
	}
}

func TestCheckoutService_Validation(t *testing.T) {
	svc := NewCheckoutService(&fakeCheckoutRepo{})
	ctx := context.Background()

	set := func(fn func(*CheckoutSubmission)) *CheckoutSubmission {
		in := guestSubmission()
		fn(in)
		return in
	}

	cases := []struct {
		name    string
		in      *CheckoutSubmission
		wantErr error
	}{
		{"nil submission", nil, errors.New("nil")},
		{"bad email", set(func(s *CheckoutSubmission) { s.Email = "" }), ErrInvalidEmail},
		{"no @", set(func(s *CheckoutSubmission) { s.Email = "notanemail" }), ErrInvalidEmail},
		{"empty phone", set(func(s *CheckoutSubmission) { s.Phone = "  " }), ErrInvalidPhone},
		{"empty line1", set(func(s *CheckoutSubmission) { s.ShippingAddressLine1 = "" }), ErrInvalidShippingLine1},
		{"empty city", set(func(s *CheckoutSubmission) { s.ShippingCity = "\t" }), ErrInvalidShippingCity},
		{"empty postal", set(func(s *CheckoutSubmission) { s.ShippingPostalCode = "" }), ErrInvalidShippingPostal},
		{"empty region", set(func(s *CheckoutSubmission) { s.ShippingRegion = "" }), ErrInvalidShippingRegion},
		{"country too short", set(func(s *CheckoutSubmission) { s.ShippingCountryCode = "U" }), ErrInvalidShippingCountry},
		{"country too long", set(func(s *CheckoutSubmission) { s.ShippingCountryCode = "USA" }), ErrInvalidShippingCountry},
		{"country digits", set(func(s *CheckoutSubmission) { s.ShippingCountryCode = "12" }), ErrInvalidShippingCountry},
		{"negative userID", set(func(s *CheckoutSubmission) { id := int64(-1); s.UserID = &id }), ErrInvalidUserID},
		{"zero userID", set(func(s *CheckoutSubmission) { id := int64(0); s.UserID = &id }), ErrInvalidUserID},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := svc.Submit(ctx, c.in)
			if err == nil {
				t.Fatalf("want error got nil")
			}
			if c.wantErr.Error() == "nil" {
				if !strings.Contains(err.Error(), "nil") {
					t.Errorf("want nil-submission error, got %v", err)
				}
				return
			}
			if !errors.Is(err, c.wantErr) {
				t.Errorf("want %v got %v", c.wantErr, err)
			}
		})
	}
}

func TestCheckoutService_RepoErrorWrapped(t *testing.T) {
	boom := errors.New("db boom")
	repo := &fakeCheckoutRepo{
		insertFn: func(ctx context.Context, c *repositories.Checkout) (*repositories.Checkout, error) {
			return nil, fmt.Errorf("repositories: insert checkout: %w", boom)
		},
	}
	svc := NewCheckoutService(repo)
	_, err := svc.Submit(context.Background(), guestSubmission())
	if err == nil {
		t.Fatal("want error got nil")
	}
	if !errors.Is(err, boom) {
		t.Errorf("original db error should be preserved in chain, got %v", err)
	}
	if !strings.Contains(err.Error(), "checkout submit") {
		t.Errorf("wrapper should describe the service call, got %v", err)
	}
}
