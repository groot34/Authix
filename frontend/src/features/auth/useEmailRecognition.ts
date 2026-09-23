import { useEffect, useRef, useState } from 'react';
import { api, type RegisteredUser } from '../../lib/api';
import { isValidEmail } from '../../lib/validation';

export type RecognitionStatus = 'idle' | 'checking' | 'registered' | 'unregistered' | 'error';

export function useEmailRecognition() {
  const [email, setEmail] = useState('');
  const [status, setStatus] = useState<RecognitionStatus>('idle');
  const [user, setUser] = useState<RegisteredUser | null>(null);
  const [error, setError] = useState('');
  const requestId = useRef(0);

  useEffect(() => {
    const value = email.trim();
    const currentRequest = ++requestId.current;
    const controller = new AbortController();

    if (!value) {
      setStatus('idle');
      setUser(null);
      setError('');
      return () => controller.abort();
    }

    if (!isValidEmail(value)) {
      setStatus('idle');
      setUser(null);
      setError('Enter a valid email address to check recognition.');
      return () => controller.abort();
    }

    setStatus('checking');
    setUser(null);
    setError('');
    const timer = window.setTimeout(() => {
      api.lookup(value, controller.signal)
        .then((result) => {
          if (currentRequest !== requestId.current) return;
          setUser(result.user);
          setStatus(result.registered ? 'registered' : 'unregistered');
        })
        .catch((reason: unknown) => {
          if (controller.signal.aborted || currentRequest !== requestId.current) return;
          setStatus('error');
          setError(reason instanceof Error ? reason.message : 'Email recognition is unavailable.');
        });
    }, 350);

    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [email]);

  return { email, setEmail, status, user, error };
}