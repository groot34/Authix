import { FormEvent, useEffect, useState } from 'react';
import { OtpModal } from './components/OtpModal';
import { api, ApiError, type RegisteredUser } from './lib/api';
import { isValidEmail } from './lib/validation';
import { useEmailRecognition } from './features/auth/useEmailRecognition';

const emptyCheckout = {
  email: '',
  phone: '',
  shipping_address_line1: '',
  shipping_address_line2: '',
  shipping_city: '',
  shipping_postal_code: '',
  shipping_region: '',
  shipping_country_code: '',
};

type CheckoutState = typeof emptyCheckout;

function App() {
  const [registrationEmail, setRegistrationEmail] = useState('');
  const [firstName, setFirstName] = useState('');
  const [lastName, setLastName] = useState('');
  const [registrationError, setRegistrationError] = useState('');
  const [registrationCode, setRegistrationCode] = useState('');
  const [registering, setRegistering] = useState(false);
  const [authenticatedUser, setAuthenticatedUser] = useState<RegisteredUser | null>(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [dismissedEmail, setDismissedEmail] = useState('');
  const [checkout, setCheckout] = useState<CheckoutState>(emptyCheckout);
  const [checkoutError, setCheckoutError] = useState('');
  const [checkoutSuccess, setCheckoutSuccess] = useState('');
  const [submittingCheckout, setSubmittingCheckout] = useState(false);
  const recognition = useEmailRecognition();

  useEffect(() => {
    api.me().then((result) => setAuthenticatedUser(result.user)).catch(() => undefined);
  }, []);

  useEffect(() => {
    recognition.setEmail(checkout.email);
  }, [checkout.email]);

  useEffect(() => {
    const email = recognition.email.trim().toLowerCase();
    if (recognition.status === 'registered' && recognition.user && email !== dismissedEmail && !modalOpen && !authenticatedUser) {
      setModalOpen(true);
    }
  }, [recognition.status, recognition.user, recognition.email, dismissedEmail, modalOpen, authenticatedUser]);

  async function register(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setRegistrationError('');
    setRegistrationCode('');
    if (!isValidEmail(registrationEmail)) {
      setRegistrationError('Enter a valid email address.');
      return;
    }
    if (!firstName.trim() || !lastName.trim()) {
      setRegistrationError('Add your first and last name to continue.');
      return;
    }
    setRegistering(true);
    try {
      const result = await api.register({ email: registrationEmail.trim(), first_name: firstName.trim(), last_name: lastName.trim() });
      setRegistrationCode(result.otp_code);
      setRegistrationEmail(result.user.email);
      setFirstName(result.user.first_name);
      setLastName(result.user.last_name);
    } catch (error) {
      setRegistrationError(error instanceof ApiError ? error.message : 'Registration could not be completed.');
    } finally {
      setRegistering(false);
    }
  }

  function updateCheckout(field: keyof CheckoutState, value: string) {
    setCheckout((current) => ({ ...current, [field]: value }));
    setCheckoutError('');
    setCheckoutSuccess('');
    if (field === 'email' && value.trim().toLowerCase() !== dismissedEmail) setDismissedEmail('');
  }

  async function submitCheckout(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setCheckoutError('');
    setCheckoutSuccess('');
    if (!isValidEmail(checkout.email)) {
      setCheckoutError('Enter a valid checkout email.');
      return;
    }
    if (authenticatedUser && checkout.email.trim().toLowerCase() !== authenticatedUser.email.trim().toLowerCase()) {
      setCheckoutError(`This session belongs to ${authenticatedUser.email}. Use that email or sign out before continuing.`);
      return;
    }
    const requiredFields: Array<keyof CheckoutState> = ['phone', 'shipping_address_line1', 'shipping_city', 'shipping_postal_code', 'shipping_region', 'shipping_country_code'];
    if (requiredFields.some((field) => !checkout[field].trim())) {
      setCheckoutError('Complete the required contact and shipping fields.');
      return;
    }
    setSubmittingCheckout(true);
    try {
      await api.checkout({
        ...checkout,
        shipping_address_line2: checkout.shipping_address_line2 || undefined,
      });
      setCheckoutSuccess('Your checkout details were saved successfully.');
    } catch (error) {
      setCheckoutError(error instanceof ApiError ? error.message : 'Checkout could not be saved.');
    } finally {
      setSubmittingCheckout(false);
    }
  }

  function handleGuestDismiss() {
    setDismissedEmail(recognition.email.trim().toLowerCase());
    setModalOpen(false);
  }

  function handleLogin(user: RegisteredUser) {
    setAuthenticatedUser(user);
    setCheckout((current) => ({ ...current, email: user.email }));
    setDismissedEmail(user.email.trim().toLowerCase());
    setModalOpen(false);
  }

  return (
    <div className="app-shell">
      <header className="topbar">
        <a className="brand" href="/" aria-label="Authix home"><span className="brand-mark" aria-hidden="true">A</span><span>Authix</span></a>
        <div className="topbar-meta"><span className="status-dot" aria-hidden="true" /><span>{authenticatedUser ? `Welcome, ${authenticatedUser.first_name}` : 'Foundation online'}</span></div>
      </header>

      <main className="auth-main">
        <section className="auth-intro" aria-labelledby="page-title">
          <p className="eyebrow">Secure access, kept simple</p>
          <h1 id="page-title">Your next checkout should remember you.</h1>
          <p className="hero-text">Create an Authix account or complete a checkout. Returning customers can verify their email without leaving the flow.</p>
          <div className="trust-line"><span className="status-dot" aria-hidden="true" /> No passwords stored</div>
        </section>

        <section className="auth-panel" aria-label="Account access">
          {authenticatedUser ? (
            <div className="authenticated-state">
              <p className="eyebrow">Signed in</p>
              <h2>Good to see you, {authenticatedUser.first_name}.</h2>
              <p className="panel-copy">Your secure session is active and ready to attach to this checkout.</p>
              <button className="secondary-action" type="button" onClick={() => api.logout().then(() => setAuthenticatedUser(null))}>Sign out</button>
            </div>
          ) : (
            <>
              <div className="panel-heading"><div><p className="eyebrow">New account</p><h2>Create your access</h2></div><span className="panel-step">01 / 02</span></div>
              <form onSubmit={register} noValidate>
                <label className="field-label" htmlFor="registration-email">Email address</label>
                <input id="registration-email" className="text-input" type="email" value={registrationEmail} onChange={(event) => setRegistrationEmail(event.target.value)} placeholder="you@example.com" autoComplete="email" />
                {registrationEmail && !isValidEmail(registrationEmail) && <p className="field-hint is-error">Use a complete email address.</p>}
                <div className="field-row"><div><label className="field-label" htmlFor="first-name">First name</label><input id="first-name" className="text-input" value={firstName} onChange={(event) => setFirstName(event.target.value)} autoComplete="given-name" /></div><div><label className="field-label" htmlFor="last-name">Last name</label><input id="last-name" className="text-input" value={lastName} onChange={(event) => setLastName(event.target.value)} autoComplete="family-name" /></div></div>
                {registrationError && <p className="form-message is-error" role="alert">{registrationError}</p>}
                <button className="primary-action action-wide" type="submit" disabled={registering}>{registering ? 'Creating…' : 'Create account'} <span aria-hidden="true">↗</span></button>
              </form>
              {registrationCode && <div className="code-reveal" role="status"><span>Your display code</span><strong>{registrationCode}</strong><small>Keep it nearby for verification.</small></div>}
            </>
          )}
        </section>

        <section className="checkout-panel" aria-labelledby="checkout-title">
          <div className="section-heading"><div><p className="eyebrow">Checkout</p><h2 id="checkout-title">Save your delivery details.</h2></div><p className="section-note">Guest or recognized</p></div>
          {authenticatedUser && <div className="member-banner"><span className="status-dot" aria-hidden="true" /><span>Signed in as <strong>{authenticatedUser.first_name} {authenticatedUser.last_name}</strong></span></div>}
          <form className="checkout-form" onSubmit={submitCheckout} noValidate>
            <div className="checkout-email-field">
              <label className="field-label" htmlFor="checkout-email">Email address</label>
              <input id="checkout-email" className="text-input" type="email" value={checkout.email} onChange={(event) => updateCheckout('email', event.target.value)} placeholder="you@example.com" autoComplete="email" />
              {checkout.email && !isValidEmail(checkout.email) && <p className="field-hint is-error">Use a complete email address.</p>}
              {recognition.status === 'checking' && <p className="field-hint">Checking your email…</p>}
              {recognition.status === 'unregistered' && <p className="field-hint">No account found. You can continue as a guest.</p>}
              {recognition.status === 'registered' && recognition.user && !authenticatedUser && <p className="field-hint is-recognized">Account recognized. Verification will open shortly.</p>}
              {recognition.status === 'error' && <p className="field-hint is-error">{recognition.error}</p>}
            </div>
            <div className="field-row"><div><label className="field-label" htmlFor="phone">Phone number</label><input id="phone" className="text-input" value={checkout.phone} onChange={(event) => updateCheckout('phone', event.target.value)} autoComplete="tel" /></div><div><label className="field-label" htmlFor="country">Country code</label><input id="country" className="text-input" maxLength={2} value={checkout.shipping_country_code} onChange={(event) => updateCheckout('shipping_country_code', event.target.value.toUpperCase())} placeholder="US" autoComplete="country" /></div></div>
            <div className="field-row"><div><label className="field-label" htmlFor="address-line1">Address line 1</label><input id="address-line1" className="text-input" value={checkout.shipping_address_line1} onChange={(event) => updateCheckout('shipping_address_line1', event.target.value)} autoComplete="address-line1" /></div><div><label className="field-label" htmlFor="address-line2">Address line 2 <span className="optional-label">Optional</span></label><input id="address-line2" className="text-input" value={checkout.shipping_address_line2} onChange={(event) => updateCheckout('shipping_address_line2', event.target.value)} autoComplete="address-line2" /></div></div>
            <div className="field-row field-row-three"><div><label className="field-label" htmlFor="city">City</label><input id="city" className="text-input" value={checkout.shipping_city} onChange={(event) => updateCheckout('shipping_city', event.target.value)} autoComplete="address-level2" /></div><div><label className="field-label" htmlFor="region">Region</label><input id="region" className="text-input" value={checkout.shipping_region} onChange={(event) => updateCheckout('shipping_region', event.target.value)} autoComplete="address-level1" /></div><div><label className="field-label" htmlFor="postal">Postal code</label><input id="postal" className="text-input" value={checkout.shipping_postal_code} onChange={(event) => updateCheckout('shipping_postal_code', event.target.value)} autoComplete="postal-code" /></div></div>
            {checkoutError && <p className="form-message is-error" role="alert">{checkoutError}</p>}
            {checkoutSuccess && <p className="form-message is-success" role="status">{checkoutSuccess}</p>}
            <button className="primary-action action-wide" type="submit" disabled={submittingCheckout}>{submittingCheckout ? 'Saving details…' : 'Save checkout details'} <span aria-hidden="true">↗</span></button>
          </form>
        </section>
      </main>

      <footer className="footer"><span>Authix</span><span>React / TypeScript / Vite</span></footer>
      <OtpModal email={recognition.email} open={modalOpen} onClose={handleGuestDismiss} onSuccess={handleLogin} />
    </div>
  );
}

export default App;
