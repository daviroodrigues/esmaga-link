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
  if (!isoString) return '—';
  const d = new Date(isoString);
  if (Number.isNaN(d.getTime())) return '—';
  return d.toLocaleString('pt-BR', {
    dateStyle: 'short',
    timeStyle: 'short',
  });
}

export function isDarkTheme() {
  const current = document.documentElement.getAttribute('data-theme');
  if (current === 'dark') return true;
  if (current === 'light') return false;
  return window.matchMedia('(prefers-color-scheme: dark)').matches;
}

export function initThemeToggle(buttonEl) {
  const THEME_KEY = 'esmaga_theme';
  try {
    const saved = localStorage.getItem(THEME_KEY);
    if (saved === 'dark' || saved === 'light') {
      document.documentElement.setAttribute('data-theme', saved);
    }
  } catch {
  }

  if (!buttonEl) return;

  const sunIcon = buttonEl.querySelector('[data-theme-icon="sun"]');
  const moonIcon = buttonEl.querySelector('[data-theme-icon="moon"]');

  const syncIcons = () => {
    const dark = isDarkTheme();
    if (sunIcon && moonIcon) {
      sunIcon.hidden = !dark;
      moonIcon.hidden = dark;
    }
    const label = dark ? 'Mudar para tema claro' : 'Mudar para tema escuro';
    buttonEl.setAttribute('aria-label', label);
    buttonEl.setAttribute('title', label);
    buttonEl.setAttribute('aria-pressed', String(dark));
  };

  syncIcons();

  buttonEl.addEventListener('click', () => {
    const next = isDarkTheme() ? 'light' : 'dark';
    document.documentElement.setAttribute('data-theme', next);
    try {
      localStorage.setItem(THEME_KEY, next);
    } catch {
    }
    syncIcons();
    document.dispatchEvent(new CustomEvent('esmaga:themechange', { detail: { theme: next } }));
  });

  const media = window.matchMedia('(prefers-color-scheme: dark)');
  if (typeof media.addEventListener === 'function') {
    media.addEventListener('change', () => {
      if (!document.documentElement.hasAttribute('data-theme')) {
        syncIcons();
        document.dispatchEvent(new CustomEvent('esmaga:themechange', { detail: { theme: isDarkTheme() ? 'dark' : 'light' } }));
      }
    });
  }
}

let toastTimer = null;
export function showToast(toastEl, message) {
  if (!toastEl) return;
  toastEl.textContent = '';
  requestAnimationFrame(() => {
    toastEl.textContent = message;
    toastEl.hidden = false;
  });
  if (toastTimer) clearTimeout(toastTimer);
  toastTimer = setTimeout(() => {
    toastEl.hidden = true;
  }, 2800);
}

const flashTimers = new WeakMap();
export function flashButtonLabel(buttonEl, tempText, durationMs = 1800) {
  if (!buttonEl) return;
  const original = buttonEl.dataset.originalLabel || buttonEl.textContent;
  buttonEl.dataset.originalLabel = original;
  buttonEl.textContent = tempText;

  const existing = flashTimers.get(buttonEl);
  if (existing) clearTimeout(existing);

  const timer = setTimeout(() => {
    buttonEl.textContent = original;
    flashTimers.delete(buttonEl);
  }, durationMs);
  flashTimers.set(buttonEl, timer);
}

export function initPasswordToggles(root = document) {
  const buttons = root.querySelectorAll('[data-toggle-password]');
  for (const btn of buttons) {
    const targetId = btn.getAttribute('data-toggle-password');
    const input = targetId ? document.getElementById(targetId) : null;
    if (!input) continue;

    const eyeOpen = btn.querySelector('[data-eye="open"]');
    const eyeClosed = btn.querySelector('[data-eye="closed"]');
    const baseLabel = btn.getAttribute('aria-label') || 'Mostrar senha';

    btn.addEventListener('click', () => {
      const showing = input.type === 'text';
      input.type = showing ? 'password' : 'text';
      if (eyeOpen && eyeClosed) {
        eyeOpen.hidden = !showing;
        eyeClosed.hidden = showing;
      }
      const nextVisible = !showing;
      const label = nextVisible ? baseLabel.replace(/^Mostrar/i, 'Ocultar') : baseLabel;
      btn.setAttribute('aria-label', label);
      btn.setAttribute('title', nextVisible ? 'Ocultar senha' : 'Mostrar senha');
      btn.setAttribute('aria-pressed', String(nextVisible));
    });
  }
}

export function applyPortfolioVisibility(config, root = document) {
  const enabled = Boolean(config && config.portfolio_mode);
  const nodes = root.querySelectorAll('[data-portfolio-only]');
  for (const node of nodes) {
    node.hidden = !enabled;
  }
  return enabled;
}

const FOCUSABLE_SELECTOR = [
  'a[href]',
  'button:not([disabled])',
  'input:not([disabled]):not([type="hidden"])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(', ');

export function createModalController(modals) {
  let activeModal = null;
  let lastTrigger = null;

  function getFocusable(container) {
    return Array.from(container.querySelectorAll(FOCUSABLE_SELECTOR)).filter(
      (node) => !node.hidden && node.offsetParent !== null
    );
  }

  function close(modalEl, { restoreFocus = true } = {}) {
    if (!modalEl || modalEl.hidden) return;
    modalEl.hidden = true;
    if (activeModal === modalEl) {
      activeModal = null;
      document.body.classList.remove('modal-open');
      const trigger = lastTrigger;
      lastTrigger = null;
      if (restoreFocus && trigger && typeof trigger.focus === 'function' && trigger.isConnected) {
        trigger.focus();
      }
    }
  }

  function open(modalEl, { initialFocus = null, preserveTrigger = false } = {}) {
    if (!modalEl) return;
    if (!preserveTrigger) {
      lastTrigger = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    }
    if (activeModal && activeModal !== modalEl) {
      activeModal.hidden = true;
    }
    activeModal = modalEl;
    modalEl.hidden = false;
    document.body.classList.add('modal-open');

    requestAnimationFrame(() => {
      const dialog = modalEl.querySelector('[role="dialog"]') || modalEl;
      if (initialFocus && typeof initialFocus.focus === 'function') {
        initialFocus.focus();
        return;
      }
      const focusable = getFocusable(dialog);
      if (focusable.length > 0) {
        focusable[0].focus();
      }
    });
  }

  for (const modal of modals) {
    if (!modal) continue;
    modal.addEventListener('click', (event) => {
      if (event.target === modal) {
        close(modal);
      }
    });
    const closeBtns = modal.querySelectorAll('[data-close-modal]');
    for (const btn of closeBtns) {
      btn.addEventListener('click', () => close(modal));
    }
  }

  document.addEventListener('keydown', (event) => {
    if (!activeModal || activeModal.hidden) return;

    if (event.key === 'Escape') {
      event.preventDefault();
      close(activeModal);
      return;
    }

    if (event.key === 'Tab') {
      const dialog = activeModal.querySelector('[role="dialog"]') || activeModal;
      const focusable = getFocusable(dialog);
      if (focusable.length === 0) return;

      const first = focusable[0];
      const last = focusable[focusable.length - 1];

      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    }
  });

  return { open, close };
}
