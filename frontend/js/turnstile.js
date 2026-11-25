import { getPublicConfig } from './api.js';

let siteKeyPromise = null;
let scriptLoadPromise = null;

async function getSiteKey() {
  if (!siteKeyPromise) {
    siteKeyPromise = getPublicConfig()
      .then((cfg) => (cfg && cfg.turnstile_site_key ? cfg.turnstile_site_key : ''))
      .catch(() => '');
  }
  return siteKeyPromise;
}

function loadTurnstileScript() {
  if (window.turnstile) return Promise.resolve();
  if (scriptLoadPromise) return scriptLoadPromise;
  scriptLoadPromise = new Promise((resolve, reject) => {
    const script = document.createElement('script');
    script.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit';
    script.async = true;
    script.defer = true;
    script.onload = () => resolve();
    script.onerror = () => reject(new Error('Falha ao carregar Turnstile'));
    document.head.appendChild(script);
  });
  return scriptLoadPromise;
}

export async function mountTurnstile(containerEl) {
  if (!containerEl) {
    return { enabled: false, getToken: () => '', reset: () => {} };
  }

  const siteKey = await getSiteKey();
  if (!siteKey) {
    containerEl.hidden = true;
    return { enabled: false, getToken: () => '', reset: () => {} };
  }

  await loadTurnstileScript();
  containerEl.hidden = false;
  let currentToken = '';

  const widgetId = window.turnstile.render(containerEl, {
    sitekey: siteKey,
    callback: (token) => {
      currentToken = token;
    },
    'expired-callback': () => {
      currentToken = '';
    },
    'error-callback': () => {
      currentToken = '';
    },
  });

  return {
    enabled: true,
    getToken: () => currentToken,
    reset: () => {
      currentToken = '';
      if (window.turnstile && widgetId !== undefined) {
        window.turnstile.reset(widgetId);
      }
    },
  };
}
