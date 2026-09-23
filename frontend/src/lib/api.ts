const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080').replace(/\/$/, '');

export interface RegisteredUser {
  id: number;
  email: string;
  first_name: string;
  last_name: string;
}

export interface RegisterRequest {
  email: string;
  first_name: string;
  last_name: string;
}

export interface CheckoutRequest {
  email: string;
  phone: string;
  shipping_address_line1: string;
  shipping_address_line2?: string;
  shipping_city: string;
  shipping_postal_code: string;
  shipping_region: string;
  shipping_country_code: string;
}

export interface CheckoutReceipt {
  id: number;
  user_id: number | null;
  email: string;
  created_at: string;
}

interface ApiErrorBody {
  error?: string;
}

export class ApiError extends Error {
  constructor(public readonly status: number, message: string) {
    super(message);
    this.name = 'ApiError';
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...init,
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...init.headers,
    },
  });

  const body = (await response.json().catch(() => null)) as T | ApiErrorBody | null;
  if (!response.ok) {
    const message = body && typeof body === 'object' && 'error' in body && body.error
      ? body.error
      : 'The request could not be completed.';
    throw new ApiError(response.status, message);
  }
  return body as T;
}

export const api = {
  register(input: RegisterRequest) {
    return request<{ otp_code: string; user: RegisteredUser }>('/api/register', {
      method: 'POST',
      body: JSON.stringify(input),
    });
  },

  lookup(email: string, signal?: AbortSignal) {
    return request<{ registered: boolean; user: RegisteredUser | null }>('/api/auth/lookup', {
      method: 'POST',
      signal,
      body: JSON.stringify({ email }),
    });
  },

  verify(email: string, code: string) {
    return request<{ user: RegisteredUser }>('/api/auth/verify', {
      method: 'POST',
      body: JSON.stringify({ email, code }),
    });
  },

  reissue(email: string) {
    return request<{ otp_code: string; user: RegisteredUser }>('/api/auth/reissue', {
      method: 'POST',
      body: JSON.stringify({ email }),
    });
  },

  me() {
    return request<{ user: RegisteredUser }>('/api/auth/me', { method: 'GET' });
  },

  logout() {
    return request<void>('/api/auth/logout', { method: 'POST' });
  },

  checkout(input: CheckoutRequest) {
    return request<CheckoutReceipt>('/api/checkout', {
      method: 'POST',
      body: JSON.stringify(input),
    });
  },
};

export { API_BASE_URL };
