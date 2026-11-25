export function isSafeHttpUrl(rawUrl) {
  if (typeof rawUrl !== 'string' || rawUrl.trim() === '') {
    return false;
  }
  try {
    const parsed = new URL(rawUrl);
    return parsed.protocol === 'http:' || parsed.protocol === 'https:';
  } catch {
    return false;
  }
}

export function buildShortUrl(shortCode) {
  return `${window.location.origin}/r/${encodeURIComponent(String(shortCode))}`;
}

export function clearElement(node) {
  node.replaceChildren();
}

export function el(tag, options = {}, ...children) {
  const node = document.createElement(tag);

  if (options.className) {
    node.className = options.className;
  }
  if (options.text !== undefined && options.text !== null) {
    node.textContent = String(options.text);
  }
  if (options.title) {
    node.title = String(options.title);
  }
  if (options.type) {
    node.type = options.type;
  }
  if (options.attrs) {
    for (const [k, v] of Object.entries(options.attrs)) {
      if (v !== undefined && v !== null) {
        node.setAttribute(k, String(v));
      }
    }
  }
  if (options.onClick) {
    node.addEventListener('click', options.onClick);
  }

  for (const child of children) {
    if (child === null || child === undefined) continue;
    if (typeof child === 'string' || typeof child === 'number') {
      node.appendChild(document.createTextNode(String(child)));
    } else {
      node.appendChild(child);
    }
  }

  return node;
}

export function createSafeAnchor(href, text, className = '') {
  const anchor = document.createElement('a');
  if (isSafeHttpUrl(href)) {
    anchor.href = href;
    anchor.target = '_blank';
    anchor.rel = 'noopener noreferrer';
  } else {
    anchor.href = '#';
  }
  if (className) {
    anchor.className = className;
  }
  anchor.textContent = text;
  return anchor;
}

export function formatDate(isoString) {
  if (!isoString) return '-';
  const d = new Date(isoString);
  if (Number.isNaN(d.getTime())) return '-';
  return d.toLocaleString('pt-BR', {
    dateStyle: 'short',
    timeStyle: 'short',
  });
}

export function initThemeToggle(buttonEl) {
  const THEME_KEY = 'esmaga_theme';
  const saved = localStorage.getItem(THEME_KEY);
  if (saved === 'dark' || saved === 'light') {
    document.documentElement.setAttribute('data-theme', saved);
  }

  if (!buttonEl) return;

  const sunIcon = buttonEl.querySelector('[data-theme-icon="sun"]');
  const moonIcon = buttonEl.querySelector('[data-theme-icon="moon"]');

  const syncIcons = () => {
    const current = document.documentElement.getAttribute('data-theme');
    const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
    const isDark = current ? current === 'dark' : prefersDark;

    if (sunIcon && moonIcon) {
      sunIcon.hidden = !isDark;
      moonIcon.hidden = isDark;
    }
    const label = isDark ? 'Mudar para tema claro' : 'Mudar para tema escuro';
    buttonEl.setAttribute('aria-label', label);
    buttonEl.setAttribute('title', label);
  };

  syncIcons();
  buttonEl.addEventListener('click', () => {
    const current = document.documentElement.getAttribute('data-theme');
    const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
    const isDark = current ? current === 'dark' : prefersDark;
    const next = isDark ? 'light' : 'dark';
    document.documentElement.setAttribute('data-theme', next);
    localStorage.setItem(THEME_KEY, next);
    syncIcons();
  });
}

let toastTimer = null;
export function showToast(toastEl, message) {
  if (!toastEl) return;
  toastEl.textContent = message;
  toastEl.hidden = false;
  if (toastTimer) clearTimeout(toastTimer);
  toastTimer = setTimeout(() => {
    toastEl.hidden = true;
  }, 2600);
}

export function initPasswordToggles(root = document) {
  const buttons = root.querySelectorAll('[data-toggle-password]');
  for (const btn of buttons) {
    const targetId = btn.getAttribute('data-toggle-password');
    const input = targetId ? document.getElementById(targetId) : null;
    if (!input) continue;

    const eyeOpen = btn.querySelector('[data-eye="open"]');
    const eyeClosed = btn.querySelector('[data-eye="closed"]');

    btn.addEventListener('click', () => {
      const showing = input.type === 'text';
      input.type = showing ? 'password' : 'text';
      if (eyeOpen && eyeClosed) {
        eyeOpen.hidden = !showing;
        eyeClosed.hidden = showing;
      }
      const label = showing ? 'Mostrar senha' : 'Ocultar senha';
      btn.setAttribute('aria-label', label);
      btn.setAttribute('title', label);
    });
  }
}
