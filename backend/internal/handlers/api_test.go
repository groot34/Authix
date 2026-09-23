package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/authix/authix/internal/services"
)

type fakeAuthAPI struct {
	registerFn func(context.Context, string, string, string) (string, *services.RegisteredUser, error)
	lookupFn   func(context.Context, string) (bool, *services.RegisteredUser, error)
	verifyFn   func(context.Context, string, string) (*services.RegisteredUser, error)
	reissueFn  func(context.Context, string) (string, *services.RegisteredUser, error)
	findIDFn   func(context.Context, int64) (*services.RegisteredUser, error)
}

func (f *fakeAuthAPI) Register(ctx context.Context, email, first, last string) (string, *services.RegisteredUser, error) {
	return f.registerFn(ctx, email, first, last)
}
func (f *fakeAuthAPI) LookupRegistered(ctx context.Context, email string) (bool, *services.RegisteredUser, error) {
	return f.lookupFn(ctx, email)
}
func (f *fakeAuthAPI) VerifyOTP(ctx context.Context, email, code string) (*services.RegisteredUser, error) {
	return f.verifyFn(ctx, email, code)
}
func (f *fakeAuthAPI) ReissueOTP(ctx context.Context, email string) (string, *services.RegisteredUser, error) {
	return f.reissueFn(ctx, email)
}
func (f *fakeAuthAPI) FindUserByID(ctx context.Context, id int64) (*services.RegisteredUser, error) {
	return f.findIDFn(ctx, id)
}

type fakeCheckoutAPI struct {
	last *services.CheckoutSubmission
	fn   func(context.Context, *services.CheckoutSubmission) (*services.CheckoutReceipt, error)
}

func (f *fakeCheckoutAPI) Submit(ctx context.Context, in *services.CheckoutSubmission) (*services.CheckoutReceipt, error) {
	f.last = in
	return f.fn(ctx, in)
}

type fakeSessionAPI struct {
	created    *int64
	valid      *services.AuthenticatedSession
	createFn   func(context.Context, int64) (*services.SessionCredentials, error)
	validateFn func(context.Context, string) (*services.AuthenticatedSession, error)
	revokeFn   func(context.Context, string) error
}

func (f *fakeSessionAPI) Create(ctx context.Context, id int64) (*services.SessionCredentials, error) {
	f.created = &id
	return f.createFn(ctx, id)
}
func (f *fakeSessionAPI) Validate(ctx context.Context, token string) (*services.AuthenticatedSession, error) {
	return f.validateFn(ctx, token)
}
func (f *fakeSessionAPI) Revoke(ctx context.Context, token string) error {
	return f.revokeFn(ctx, token)
}

func testUser() *services.RegisteredUser {
	return &services.RegisteredUser{ID: 7, Email: "alice@example.com", FirstName: "Alice", LastName: "Smith"}
}

func testAPI(auth *fakeAuthAPI, checkout *fakeCheckoutAPI, sessions *fakeSessionAPI) *API {
	return NewAPI(auth, checkout, sessions, "authix_session", true)
}

func requestJSON(method, path, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestAPI_RegisterAndLookup(t *testing.T) {
	auth := &fakeAuthAPI{
		registerFn: func(_ context.Context, email, first, last string) (string, *services.RegisteredUser, error) {
			if email != "alice@example.com" || first != "Alice" || last != "Smith" {
				t.Fatalf("unexpected register input: %q %q %q", email, first, last)
			}
			return "123456", testUser(), nil
		},
		lookupFn: func(_ context.Context, email string) (bool, *services.RegisteredUser, error) {
			return email == "alice@example.com", testUser(), nil
		},
	}
	a := testAPI(auth, &fakeCheckoutAPI{}, &fakeSessionAPI{})
	response := httptest.NewRecorder()
	a.Register(response, requestJSON(http.MethodPost, "/api/register", `{"email":"alice@example.com","first_name":"Alice","last_name":"Smith"}`))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"otp_code":"123456"`) {
		t.Fatalf("register response: %d %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "otp_code_hash") {
		t.Fatal("registration leaked an OTP hash")
	}

	response = httptest.NewRecorder()
	a.Lookup(response, requestJSON(http.MethodPost, "/api/auth/lookup", `{"email":"alice@example.com"}`))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"registered":true`) {
		t.Fatalf("lookup response: %d %s", response.Code, response.Body.String())
	}
}

func TestAPI_InvalidJSONAndServiceErrors(t *testing.T) {
	auth := &fakeAuthAPI{
		registerFn: func(context.Context, string, string, string) (string, *services.RegisteredUser, error) {
			return "", nil, services.ErrInvalidEmail
		},
	}
	a := testAPI(auth, &fakeCheckoutAPI{}, &fakeSessionAPI{})
	response := httptest.NewRecorder()
	a.Register(response, requestJSON(http.MethodPost, "/api/register", `{bad`))
	if response.Code != http.StatusBadRequest {
		t.Errorf("malformed JSON status want 400 got %d", response.Code)
	}
	response = httptest.NewRecorder()
	a.Register(response, requestJSON(http.MethodPost, "/api/register", `{"email":"bad","first_name":"A","last_name":"B"}`))
	if response.Code != http.StatusBadRequest || strings.Contains(response.Body.String(), "invalid email") {
		t.Errorf("invalid input response leaked details: %d %s", response.Code, response.Body.String())
	}
}

func TestAPI_VerifyCreatesSessionOnlyOnSuccess(t *testing.T) {
	sessions := &fakeSessionAPI{
		createFn: func(context.Context, int64) (*services.SessionCredentials, error) {
			return &services.SessionCredentials{Token: "opaque-token", UserID: 7, ExpiresAt: time.Now().Add(time.Hour)}, nil
		},
	}
	auth := &fakeAuthAPI{verifyFn: func(context.Context, string, string) (*services.RegisteredUser, error) { return testUser(), nil }}
	a := testAPI(auth, &fakeCheckoutAPI{}, sessions)
	response := httptest.NewRecorder()
	a.Verify(response, requestJSON(http.MethodPost, "/api/auth/verify", `{"email":"alice@example.com","code":"123456"}`))
	if response.Code != http.StatusOK || sessions.created == nil || *sessions.created != 7 {
		t.Fatalf("verify response/session: %d created=%v", response.Code, sessions.created)
	}
	cookie := response.Result().Cookies()[0]
	if cookie.Name != "authix_session" || cookie.Value != "opaque-token" || !cookie.HttpOnly || !cookie.Secure {
		t.Fatalf("unexpected session cookie: %+v", cookie)
	}

	sessions.created = nil
	auth.verifyFn = func(context.Context, string, string) (*services.RegisteredUser, error) {
		return nil, services.ErrOTPExpired
	}
	response = httptest.NewRecorder()
	a.Verify(response, requestJSON(http.MethodPost, "/api/auth/verify", `{"email":"alice@example.com","code":"123456"}`))
	if response.Code != http.StatusUnauthorized || sessions.created != nil {
		t.Fatalf("failed verify should not create session: %d created=%v", response.Code, sessions.created)
	}
}

func TestAPI_MeLogoutAndCheckoutIdentity(t *testing.T) {
	sessions := &fakeSessionAPI{
		valid: &services.AuthenticatedSession{UserID: 7},
		validateFn: func(context.Context, string) (*services.AuthenticatedSession, error) {
			return &services.AuthenticatedSession{UserID: 7}, nil
		},
		revokeFn: func(context.Context, string) error { return nil },
	}
	auth := &fakeAuthAPI{findIDFn: func(_ context.Context, id int64) (*services.RegisteredUser, error) {
		if id != 7 {
			return nil, services.ErrUserNotRegistered
		}
		return testUser(), nil
	}}
	checkout := &fakeCheckoutAPI{fn: func(_ context.Context, in *services.CheckoutSubmission) (*services.CheckoutReceipt, error) {
		return &services.CheckoutReceipt{ID: 11, UserID: in.UserID, Email: in.Email}, nil
	}}
	a := testAPI(auth, checkout, sessions)

	req := requestJSON(http.MethodGet, "/api/auth/me", "")
	req.AddCookie(&http.Cookie{Name: "authix_session", Value: "opaque-token"})
	response := httptest.NewRecorder()
	a.Me(response, req)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "alice@example.com") {
		t.Fatalf("me response: %d %s", response.Code, response.Body.String())
	}

	req = requestJSON(http.MethodPost, "/api/checkout", `{"email":"alice@example.com","phone":"1","shipping_address_line1":"A","shipping_city":"C","shipping_postal_code":"P","shipping_region":"R","shipping_country_code":"US","user_id":999}`)
	response = httptest.NewRecorder()
	a.Checkout(response, req)
	if response.Code != http.StatusBadRequest || checkout.last != nil {
		t.Fatalf("client user_id should be rejected: %d %+v", response.Code, checkout.last)
	}

	req = requestJSON(http.MethodPost, "/api/checkout", `{"email":"alice@example.com","phone":"1","shipping_address_line1":"A","shipping_city":"C","shipping_postal_code":"P","shipping_region":"R","shipping_country_code":"US"}`)
	req.AddCookie(&http.Cookie{Name: "authix_session", Value: "opaque-token"})
	response = httptest.NewRecorder()
	a.Checkout(response, req)
	if response.Code != http.StatusCreated || checkout.last.UserID == nil || *checkout.last.UserID != 7 {
		t.Fatalf("checkout did not use session user: %d %+v", response.Code, checkout.last)
	}

	response = httptest.NewRecorder()
	a.Logout(response, req)
	if response.Code != http.StatusNoContent || response.Result().Cookies()[0].MaxAge != -1 {
		t.Fatalf("logout response/cookie: %d %+v", response.Code, response.Result().Cookies())
	}
}

func TestAPI_MeRejectsInvalidSessionAndMapsRateLimit(t *testing.T) {
	sessions := &fakeSessionAPI{
		validateFn: func(context.Context, string) (*services.AuthenticatedSession, error) {
			return nil, services.ErrSessionNotFound
		},
	}
	auth := &fakeAuthAPI{verifyFn: func(context.Context, string, string) (*services.RegisteredUser, error) {
		return nil, services.ErrOTPRateLimited
	}}
	a := testAPI(auth, &fakeCheckoutAPI{}, sessions)

	response := httptest.NewRecorder()
	a.Me(response, httptest.NewRequest(http.MethodGet, "/api/auth/me", nil))
	if response.Code != http.StatusUnauthorized {
		t.Errorf("missing session status want 401 got %d", response.Code)
	}

	req := requestJSON(http.MethodGet, "/api/auth/me", "")
	req.AddCookie(&http.Cookie{Name: "authix_session", Value: "bad-token"})
	response = httptest.NewRecorder()
	a.Me(response, req)
	if response.Code != http.StatusUnauthorized {
		t.Errorf("invalid session status want 401 got %d", response.Code)
	}

	response = httptest.NewRecorder()
	a.Verify(response, requestJSON(http.MethodPost, "/api/auth/verify", `{"email":"a@example.com","code":"123456"}`))
	if response.Code != http.StatusTooManyRequests {
		t.Errorf("rate limit status want 429 got %d", response.Code)
	}
}

func TestAPI_CORS(t *testing.T) {
	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }), []string{"http://localhost:5173"})
	request := httptest.NewRequest(http.MethodOptions, "/api/register", nil)
	request.Header.Set("Origin", "http://localhost:5173")
	request.Header.Set("Access-Control-Request-Method", "POST")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || response.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" || response.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("allowed CORS response: %d %v", response.Code, response.Header())
	}

	request = httptest.NewRequest(http.MethodOptions, "/api/register", nil)
	request.Header.Set("Origin", "https://evil.example")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || response.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("disallowed CORS response: %d %v", response.Code, response.Header())
	}
}

func TestAPI_GuestCheckout(t *testing.T) {
	checkout := &fakeCheckoutAPI{fn: func(_ context.Context, in *services.CheckoutSubmission) (*services.CheckoutReceipt, error) {
		if in.UserID != nil {
			t.Fatalf("guest checkout has user ID %v", in.UserID)
		}
		return &services.CheckoutReceipt{ID: 12}, nil
	}}
	sessions := &fakeSessionAPI{validateFn: func(context.Context, string) (*services.AuthenticatedSession, error) {
		return nil, errors.New("should not call")
	}}
	a := testAPI(&fakeAuthAPI{}, checkout, sessions)
	response := httptest.NewRecorder()
	a.Checkout(response, requestJSON(http.MethodPost, "/api/checkout", `{"email":"guest@example.com","phone":"1","shipping_address_line1":"A","shipping_city":"C","shipping_postal_code":"P","shipping_region":"R","shipping_country_code":"US"}`))
	if response.Code != http.StatusCreated {
		t.Fatalf("guest checkout status: %d", response.Code)
	}
}

func TestAPIResponseIsJSON(t *testing.T) {
	a := testAPI(&fakeAuthAPI{registerFn: func(context.Context, string, string, string) (string, *services.RegisteredUser, error) {
		return "", nil, services.ErrInvalidEmail
	}}, &fakeCheckoutAPI{}, &fakeSessionAPI{})
	response := httptest.NewRecorder()
	a.Register(response, requestJSON(http.MethodPost, "/api/register", `{}`))
	var body map[string]string
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil || body["error"] == "" {
		t.Fatalf("invalid JSON error body: %v %s", err, response.Body.String())
	}
}
