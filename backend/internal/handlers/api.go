package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/groot34/Authix/internal/services"
)

type authAPI interface {
	Register(context.Context, string, string, string) (string, *services.RegisteredUser, error)
	LookupRegistered(context.Context, string) (bool, *services.RegisteredUser, error)
	VerifyOTP(context.Context, string, string) (*services.RegisteredUser, error)
	ReissueOTP(context.Context, string) (string, *services.RegisteredUser, error)
	FindUserByID(context.Context, int64) (*services.RegisteredUser, error)
}

type checkoutAPI interface {
	Submit(context.Context, *services.CheckoutSubmission) (*services.CheckoutReceipt, error)
}

type sessionAPI interface {
	Create(context.Context, int64) (*services.SessionCredentials, error)
	Validate(context.Context, string) (*services.AuthenticatedSession, error)
	Revoke(context.Context, string) error
}

type API struct {
	auth         authAPI
	checkout     checkoutAPI
	sessions     sessionAPI
	cookieName   string
	cookieSecure bool
}

func NewAPI(auth authAPI, checkout checkoutAPI, sessions sessionAPI, cookieName string, cookieSecure bool) *API {
	if cookieName == "" {
		cookieName = "authix_session"
	}
	return &API{auth: auth, checkout: checkout, sessions: sessions, cookieName: cookieName, cookieSecure: cookieSecure}
}

func (a *API) Register(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email     string `json:"email"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	code, user, err := a.auth.Register(r.Context(), in.Email, in.FirstName, in.LastName)
	if err != nil {
		a.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"otp_code": code, "user": publicUser(user)})
}

func (a *API) Lookup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email string `json:"email"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	found, user, err := a.auth.LookupRegistered(r.Context(), in.Email)
	if err != nil {
		a.writeServiceError(w, err)
		return
	}
	var responseUser any
	if found {
		responseUser = publicUser(user)
	}
	writeJSON(w, http.StatusOK, map[string]any{"registered": found, "user": responseUser})
}

func (a *API) Verify(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	user, err := a.auth.VerifyOTP(r.Context(), in.Email, in.Code)
	if err != nil {
		a.writeServiceError(w, err)
		return
	}
	credentials, err := a.sessions.Create(r.Context(), user.ID)
	if err != nil {
		a.writeServiceError(w, err)
		return
	}
	a.setSessionCookie(w, credentials.Token, credentials.ExpiresAt)
	writeJSON(w, http.StatusOK, map[string]any{"user": publicUser(user)})
}

func (a *API) Reissue(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email string `json:"email"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	code, user, err := a.auth.ReissueOTP(r.Context(), in.Email)
	if err != nil {
		if errors.Is(err, services.ErrUserNotRegistered) {
			writeError(w, http.StatusNotFound, "no registered account found for this email")
			return
		}
		a.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"otp_code": code, "user": publicUser(user)})
}

func (a *API) Me(w http.ResponseWriter, r *http.Request) {
	token, ok := a.sessionToken(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	session, err := a.sessions.Validate(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	user, err := a.auth.FindUserByID(r.Context(), session.UserID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": publicUser(user)})
}

func (a *API) Logout(w http.ResponseWriter, r *http.Request) {
	if token, ok := a.sessionToken(r); ok {
		if err := a.sessions.Revoke(r.Context(), token); err != nil &&
			!errors.Is(err, services.ErrSessionNotFound) &&
			!errors.Is(err, services.ErrInvalidSessionToken) {
			writeError(w, http.StatusInternalServerError, "could not log out")
			return
		}
	}
	a.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) Checkout(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email                string          `json:"email"`
		Phone                string          `json:"phone"`
		ShippingAddressLine1 string          `json:"shipping_address_line1"`
		ShippingAddressLine2 *string         `json:"shipping_address_line2"`
		ShippingCity         string          `json:"shipping_city"`
		ShippingPostalCode   string          `json:"shipping_postal_code"`
		ShippingRegion       string          `json:"shipping_region"`
		ShippingCountryCode  string          `json:"shipping_country_code"`
		UserID               json.RawMessage `json:"user_id"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if len(in.UserID) > 0 && string(in.UserID) != "null" {
		writeError(w, http.StatusBadRequest, "user_id is server-managed")
		return
	}
	var userID *int64
	if token, ok := a.sessionToken(r); ok {
		session, err := a.sessions.Validate(r.Context(), token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		id := session.UserID
		userID = &id
	}
	receipt, err := a.checkout.Submit(r.Context(), &services.CheckoutSubmission{
		UserID: userID, Email: in.Email, Phone: in.Phone,
		ShippingAddressLine1: in.ShippingAddressLine1, ShippingAddressLine2: in.ShippingAddressLine2,
		ShippingCity: in.ShippingCity, ShippingPostalCode: in.ShippingPostalCode,
		ShippingRegion: in.ShippingRegion, ShippingCountryCode: in.ShippingCountryCode,
	})
	if err != nil {
		a.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": receipt.ID, "user_id": receipt.UserID, "email": receipt.Email, "created_at": receipt.CreatedAt,
	})
}

func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/register", a.Register)
	mux.HandleFunc("POST /api/auth/lookup", a.Lookup)
	mux.HandleFunc("POST /api/auth/verify", a.Verify)
	mux.HandleFunc("POST /api/auth/reissue", a.Reissue)
	mux.HandleFunc("GET /api/auth/me", a.Me)
	mux.HandleFunc("POST /api/auth/logout", a.Logout)
	mux.HandleFunc("POST /api/checkout", a.Checkout)
	return mux
}

func (a *API) setSessionCookie(w http.ResponseWriter, token string, expires time.Time) {
	sameSite := http.SameSiteLaxMode
	if a.cookieSecure {
		sameSite = http.SameSiteNoneMode
	}
	http.SetCookie(w, &http.Cookie{Name: a.cookieName, Value: token, Path: "/", Expires: expires, MaxAge: maxAge(expires), HttpOnly: true, Secure: a.cookieSecure, SameSite: sameSite})
}

func (a *API) clearSessionCookie(w http.ResponseWriter) {
	sameSite := http.SameSiteLaxMode
	if a.cookieSecure {
		sameSite = http.SameSiteNoneMode
	}
	http.SetCookie(w, &http.Cookie{Name: a.cookieName, Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(1, 0), HttpOnly: true, Secure: a.cookieSecure, SameSite: sameSite})
}

func (a *API) sessionToken(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(a.cookieName)
	if err != nil || cookie.Value == "" {
		return "", false
	}
	return cookie.Value, true
}

func (a *API) writeServiceError(w http.ResponseWriter, err error) {
	status, message := http.StatusInternalServerError, "request failed"
	switch {
	case errors.Is(err, services.ErrUserAlreadyRegistered):
		status, message = http.StatusConflict, "email is already registered"
	case errors.Is(err, services.ErrInvalidEmail), errors.Is(err, services.ErrInvalidName), errors.Is(err, services.ErrInvalidOTPFormat),
		errors.Is(err, services.ErrInvalidPhone), errors.Is(err, services.ErrInvalidShippingLine1), errors.Is(err, services.ErrInvalidShippingCity),
		errors.Is(err, services.ErrInvalidShippingPostal), errors.Is(err, services.ErrInvalidShippingRegion), errors.Is(err, services.ErrInvalidShippingCountry),
		errors.Is(err, services.ErrInvalidUserID):
		status, message = http.StatusBadRequest, "invalid request"
	case errors.Is(err, services.ErrOTPIncorrect), errors.Is(err, services.ErrOTPNotIssued), errors.Is(err, services.ErrOTPAlreadyUsed), errors.Is(err, services.ErrOTPExpired), errors.Is(err, services.ErrOTPInvalidState), errors.Is(err, services.ErrUserNotRegistered):
		status, message = http.StatusUnauthorized, "authentication failed"
	case errors.Is(err, services.ErrOTPRateLimited):
		status, message = http.StatusTooManyRequests, "too many verification attempts"
	}
	writeError(w, status, message)
}

func publicUser(user *services.RegisteredUser) map[string]any {
	return map[string]any{"id": user.ID, "email": user.Email, "first_name": user.FirstName, "last_name": user.LastName}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request")
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		writeError(w, http.StatusBadRequest, "request must contain one JSON value")
		return false
	} else if !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid JSON request")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func maxAge(expires time.Time) int {
	seconds := int(time.Until(expires).Seconds())
	if seconds < 1 {
		return 1
	}
	return seconds
}

func CORS(next http.Handler, allowedOrigins []string) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		if origin != "" && origin != "*" {
			allowed[origin] = struct{}{}
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if _, ok := allowed[origin]; ok {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Add("Vary", "Origin")
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
