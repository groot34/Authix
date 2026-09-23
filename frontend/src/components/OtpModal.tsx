import { useState } from 'react';
import { api, ApiError, type RegisteredUser } from '../lib/api';

interface OtpModalProps {
  email: string;
  open: boolean;
  onClose: () => void;
  onSuccess: (user: RegisteredUser) => void;
}

export function OtpModal({ email, open, onClose, onSuccess }: OtpModalProps) {
  const [code, setCode] = useState('');
  const [message, setMessage] = useState('');
  const [submitting, setSubmitting] = useState(false);

  if (!open) return null;

  async function submitCode() {
    setSubmitting(true);
    setMessage('');
    try {
      const result = await api.verify(email, code);
      onSuccess(result.user);
    } catch (error) {
      setMessage(error instanceof ApiError ? error.message : 'The code could not be verified.');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="modal-backdrop" role="presentation" onMouseDown={(event) => event.target === event.currentTarget && onClose()}>
      <section className="otp-modal" role="dialog" aria-modal="true" aria-labelledby="otp-title">
        <button className="modal-close" type="button" onClick={onClose} aria-label="Close verification dialog">×</button>
        <p className="eyebrow">Returning customer</p>
        <h2 id="otp-title">Enter your access code.</h2>
        <p className="modal-copy">We recognized <strong>{email}</strong>. Use the six-digit code associated with this account.</p>
        <label className="field-label" htmlFor="otp-code">One-time code</label>
        <input
          id="otp-code"
          className="text-input otp-input"
          inputMode="numeric"
          autoComplete="one-time-code"
          maxLength={6}
          value={code}
          onChange={(event) => setCode(event.target.value.replace(/\D/g, ''))}
          placeholder="000000"
          autoFocus
        />
        {message && <p className="form-message is-error" role="alert">{message}</p>}
        <button className="primary-action action-wide" type="button" disabled={submitting || code.length !== 6} onClick={submitCode}>
          {submitting ? 'Checking…' : 'Verify code'}
        </button>
        <div className="modal-actions">
          <button className="quiet-button" type="button" onClick={onClose}>Continue as guest</button>
        </div>
      </section>
    </div>
  );
}