import { useEffect } from 'react';

export type ToastKind = 'success' | 'error';

interface ToastProps {
  kind: ToastKind;
  message: string;
  onDismiss: () => void;
}

export function Toast({ kind, message, onDismiss }: ToastProps) {
  useEffect(() => {
    const timer = window.setTimeout(onDismiss, 4500);
    return () => window.clearTimeout(timer);
  }, [message, onDismiss]);

  return (
    <div className={`toast toast-${kind}`} role={kind === 'error' ? 'alert' : 'status'}>
      <span>{message}</span>
      <button type="button" onClick={onDismiss} aria-label="Dismiss notification">×</button>
    </div>
  );
}
