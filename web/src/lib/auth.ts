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

export function csrfToken(): string {
  const prefix = 'binnacle_csrf=';
  return (
    document.cookie
      .split(';')
      .map((value) => value.trim())
      .find((value) => value.startsWith(prefix))
      ?.slice(prefix.length) ?? ''
  );
}

export async function authenticatedMutation<T>(
  path: string,
  method: 'POST' | 'PATCH' | 'DELETE',
  body?: unknown,
): Promise<T | null> {
  const response = await fetch(path, {
    method,
    credentials: 'same-origin',
    headers: {
      'X-CSRF-Token': decodeURIComponent(csrfToken()),
      ...(body === undefined ? {} : { 'Content-Type': 'application/json' }),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!response.ok) throw await errorFrom(response);
  if (response.status === 204) return null;
  return (await response.json()) as T;
}

export async function logout(all = false): Promise<void> {
  await authenticatedMutation(
    all ? '/api/v1/auth/logout-all' : '/api/v1/auth/logout',
    'POST',
  );
}

export function safeRedirect(value: string | null): string {
  if (!value) return '/watch';
  try {
    const target = new URL(value, location.origin);
    if (target.origin !== location.origin || !target.pathname.startsWith('/'))
      return '/watch';
    const allowed = [
      '/watch',
      '/resources',
      '/server',
      '/events',
      '/alerts',
      '/settings',
      '/onboarding',
    ];
    if (
      target.pathname.startsWith('//') ||
      !allowed.some(
        (path) =>
          target.pathname === path || target.pathname.startsWith(`${path}/`),
      )
    )
      return '/watch';
    return `${target.pathname}${target.search}${target.hash}`;
  } catch {
    return '/watch';
  }
}
