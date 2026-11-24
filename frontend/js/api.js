/**
 * Cliente REST e gestão de sessão JWT encapsulada em closure do módulo.
 * O token nunca é exposto em window.* e permanece apenas em sessionStorage.
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

export function onUnauthorized(callback) {
  unauthorizedHandler = callback;
}

export function hasSession() {
  return typeof authToken === 'string' && authToken.length > 0;
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

async function apiFetch(path, options = {}) {
  const headers = new Headers(options.headers || {});
  if (options.body !== undefined && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }
  if (authToken) {
    headers.set('Authorization', `Bearer ${authToken}`);
  }

  const response = await fetch(path, {
    ...options,
    headers,
  });

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

export async function register(email, password) {
  return apiFetch('/api/v1/auth/register', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  });
}

export async function login(email, password) {
  const data = await apiFetch('/api/v1/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  });
  if (!data || !data.token) {
    throw new Error('Token JWT não retornado pelo servidor');
  }
  setSessionToken(data.token);
  return data;
}

export function logout() {
  clearSession();
}

export async function deleteAccount(keepLinksActive = true) {
  const data = await apiFetch('/api/v1/auth/account', {
    method: 'DELETE',
    body: JSON.stringify({ keep_links_active: Boolean(keepLinksActive) }),
  });
  clearSession();
  return data;
}

export async function createLink(originalUrl) {
  return apiFetch('/api/v1/links', {
    method: 'POST',
    body: JSON.stringify({ original_url: originalUrl }),
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

export async function getLinkStats(code, days = 7) {
  const safeDays = Math.max(1, Math.min(90, Number(days) || 7));
  return apiFetch(`/api/v1/links/${encodeURIComponent(code)}/stats?days=${safeDays}`, {
    method: 'GET',
  });
}
