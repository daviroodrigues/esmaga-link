/**
 * Cliente REST e gestão de sessão JWT encapsulada em closure do módulo.
 * O access_token permanece apenas em sessionStorage e o refresh_token em cookie HttpOnly.
 */

const TOKEN_STORAGE_KEY = 'esmaga_jwt_token';

let authToken = (() => {
  try {
    return sessionStorage.getItem(TOKEN_STORAGE_KEY);
  } catch {
    return null;
  }
})();

let unauthorizedHandler = null;
let refreshPromise = null;

export function onUnauthorized(callback) {
  unauthorizedHandler = callback;
}

export function hasSession() {
  return typeof authToken === 'string' && authToken.length > 0;
}

export async function restoreSession() {
  if (hasSession()) return true;
  return tryRefreshSession();
}

function setSessionToken(token) {
  authToken = token;
  try {
    sessionStorage.setItem(TOKEN_STORAGE_KEY, token);
  } catch {
    // Fallback em memória caso sessionStorage esteja indisponível
  }
}

export function clearSession() {
  authToken = null;
  try {
    sessionStorage.removeItem(TOKEN_STORAGE_KEY);
  } catch {
    // Ignora erro de storage
  }
}

async function tryRefreshSession() {
  if (refreshPromise) return refreshPromise;
  refreshPromise = (async () => {
    try {
      const resp = await fetch('/api/v1/auth/refresh', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({}),
      });
      if (!resp.ok) return false;
      const data = await resp.json();
      if (data && data.access_token) {
        setSessionToken(data.access_token);
        return true;
      }
      return false;
    } catch {
      return false;
    } finally {
      refreshPromise = null;
    }
  })();
  return refreshPromise;
}

async function apiFetch(path, options = {}, allowRetry = true) {
  const headers = new Headers(options.headers || {});
  if (options.body !== undefined && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }
  if (authToken) {
    headers.set('Authorization', `Bearer ${authToken}`);
  }

  const response = await fetch(path, {
    ...options,
    credentials: 'include',
    headers,
  });

  if (response.status === 401 && allowRetry && !path.startsWith('/api/v1/auth/')) {
    const refreshed = await tryRefreshSession();
    if (refreshed) {
      return apiFetch(path, options, false);
    }
  }

  if (response.status === 401) {
    clearSession();
    if (typeof unauthorizedHandler === 'function') {
      unauthorizedHandler();
    }
  }

  const text = await response.text();
  let data = null;
  if (text) {
    try {
      data = JSON.parse(text);
    } catch {
      data = { error: text };
    }
  }

  if (!response.ok) {
    const message = (data && data.error) || `Erro HTTP ${response.status}`;
    const err = new Error(message);
    err.status = response.status;
    throw err;
  }

  return data;
}

export async function getPublicConfig() {
  return apiFetch('/api/v1/config', { method: 'GET' }, false);
}

export async function register(email, password, name = '', turnstileToken = '') {
  const payload = { email, password };
  if (name && name.trim()) payload.name = name.trim();
  if (turnstileToken) payload.turnstile_token = turnstileToken;
  return apiFetch('/api/v1/auth/register', {
    method: 'POST',
    body: JSON.stringify(payload),
  }, false);
}

export async function verifyEmail(token) {
  return apiFetch('/api/v1/auth/verify-email', {
    method: 'POST',
    body: JSON.stringify({ token }),
  }, false);
}

export async function resendVerification(email) {
  return apiFetch('/api/v1/auth/resend-verification', {
    method: 'POST',
    body: JSON.stringify({ email }),
  }, false);
}

export async function login(email, password) {
  const data = await apiFetch('/api/v1/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  }, false);
  if (!data || !data.access_token) {
    throw new Error('Access token não retornado pelo servidor');
  }
  setSessionToken(data.access_token);
  return data;
}

export async function logout() {
  try {
    await apiFetch('/api/v1/auth/logout', { method: 'POST', body: JSON.stringify({}) }, false);
  } catch {
    // Limpa sessão local mesmo se a chamada falhar
  }
  clearSession();
}

export async function forgotPassword(email, turnstileToken = '') {
  const payload = { email };
  if (turnstileToken) payload.turnstile_token = turnstileToken;
  return apiFetch('/api/v1/auth/forgot-password', {
    method: 'POST',
    body: JSON.stringify(payload),
  }, false);
}

export async function resetPassword(token, newPassword) {
  return apiFetch('/api/v1/auth/reset-password', {
    method: 'POST',
    body: JSON.stringify({ token, new_password: newPassword }),
  }, false);
}

export async function getMe() {
  return apiFetch('/api/v1/auth/me', { method: 'GET' });
}

export async function updateMe(payload) {
  return apiFetch('/api/v1/auth/me', {
    method: 'PATCH',
    body: JSON.stringify(payload),
  });
}

export async function deleteAccount(keepLinksActive = true) {
  const data = await apiFetch('/api/v1/auth/account', {
    method: 'DELETE',
    body: JSON.stringify({ keep_links_active: Boolean(keepLinksActive) }),
  });
  clearSession();
  return data;
}

export async function createLink(originalUrl, turnstileToken = '') {
  const payload = { original_url: originalUrl };
  if (turnstileToken) payload.turnstile_token = turnstileToken;
  return apiFetch('/api/v1/links', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export async function listMyLinks() {
  return apiFetch('/api/v1/links', { method: 'GET' });
}

export async function editLink(code, originalUrl) {
  return apiFetch(`/api/v1/links/${encodeURIComponent(code)}`, {
    method: 'PUT',
    body: JSON.stringify({ original_url: originalUrl }),
  });
}

export async function deleteLink(code) {
  return apiFetch(`/api/v1/links/${encodeURIComponent(code)}`, {
    method: 'DELETE',
  });
}

export async function getLinkStats(code, days = 7) {
  const safeDays = Math.max(1, Math.min(90, Number(days) || 7));
  return apiFetch(`/api/v1/links/${encodeURIComponent(code)}/stats?days=${safeDays}`, {
    method: 'GET',
  });
}
