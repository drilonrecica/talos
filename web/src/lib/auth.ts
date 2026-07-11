export interface SessionInfo {
  user: { id: string; username: string };
  expiresAt: string;
  absoluteExpiresAt: string;
}

interface APIError {
  error?: {
    code?: string;
    message?: string;
    details?: { retryAfterSeconds?: number };
  };
}

export class AuthError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly retryAfterSeconds?: number,
  ) {
    super(message);
  }
}

async function errorFrom(response: Response): Promise<AuthError> {
  let body: APIError = {};
  try {
    body = (await response.json()) as APIError;
  } catch {
    // The safe generic message below covers invalid upstream responses.
  }
  return new AuthError(
    body.error?.message ?? 'The request could not be completed.',
    response.status,
    body.error?.details?.retryAfterSeconds,
  );
}

export async function login(username: string, password: string): Promise<void> {
  const response = await fetch('/api/v1/auth/login', {
    method: 'POST',
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  });
  if (!response.ok) throw await errorFrom(response);
}

export async function currentSession(): Promise<SessionInfo | null> {
  const response = await fetch('/api/v1/auth/session', {
    credentials: 'same-origin',
  });
  if (response.status === 401) return null;
  if (!response.ok) throw await errorFrom(response);
  return (await response.json()) as SessionInfo;
}

function csrfToken(): string {
  const prefix = 'talos_csrf=';
  return (
    document.cookie
      .split(';')
      .map((value) => value.trim())
      .find((value) => value.startsWith(prefix))
      ?.slice(prefix.length) ?? ''
  );
}

export async function logout(all = false): Promise<void> {
  const response = await fetch(
    all ? '/api/v1/auth/logout-all' : '/api/v1/auth/logout',
    {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'X-CSRF-Token': decodeURIComponent(csrfToken()) },
    },
  );
  if (!response.ok) throw await errorFrom(response);
}

export function safeRedirect(value: string | null): string {
  if (!value) return '/overview';
  try {
    const target = new URL(value, location.origin);
    if (target.origin !== location.origin || !target.pathname.startsWith('/'))
      return '/overview';
    if (target.pathname.startsWith('//') || target.pathname === '/login')
      return '/overview';
    return `${target.pathname}${target.search}${target.hash}`;
  } catch {
    return '/overview';
  }
}
