import { createLink, getPublicConfig } from './api.js';
import { downloadQRCodePNG, renderQRCode } from './qrcode.js';
import { mountTurnstile } from './turnstile.js';
import {
  applyPortfolioVisibility,
  buildShortUrl,
  clearElement,
  createSafeAnchor,
  el,
  flashButtonLabel,
  formatDate,
  initThemeToggle,
  isSafeHttpUrl,
  showToast,
} from './security.js';

const HISTORY_KEY = 'esmaga_recent_links';
const MAX_HISTORY = 5;

const themeBtn = document.getElementById('theme-toggle');
const form = document.getElementById('shorten-form');
const urlInput = document.getElementById('url-input');
const shortenBtn = document.getElementById('shorten-btn');
const errorBox = document.getElementById('home-error');

const resultCard = document.getElementById('result-card');
const resultLinkSlot = document.getElementById('result-link-slot');
const resultOriginal = document.getElementById('result-original');
const copyResultBtn = document.getElementById('copy-result-btn');
const qrCanvas = document.getElementById('result-qr-canvas');
const downloadQrBtn = document.getElementById('download-qr-btn');

const historySection = document.getElementById('history-section');
const historyTbody = document.getElementById('history-tbody');
const clearHistoryBtn = document.getElementById('clear-history-btn');
const toastEl = document.getElementById('toast');
const turnstileContainer = document.getElementById('home-turnstile');

let lastShortUrl = '';
let lastShortCode = '';
let turnstileCtrl = { enabled: false, getToken: () => '', reset: () => {} };

initThemeToggle(themeBtn);
getPublicConfig().then((cfg) => applyPortfolioVisibility(cfg));
mountTurnstile(turnstileContainer).then((ctrl) => {
  turnstileCtrl = ctrl;
});

function setError(message) {
  if (!message) {
    errorBox.hidden = true;
    errorBox.textContent = '';
    urlInput.removeAttribute('aria-invalid');
    return;
  }
  errorBox.textContent = message;
  errorBox.hidden = false;
  urlInput.setAttribute('aria-invalid', 'true');
}

function loadHistory() {
  try {
    const raw = localStorage.getItem(HISTORY_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed.filter(
      (item) =>
        item &&
        typeof item.short_code === 'string' &&
        item.short_code.length > 0 &&
        isSafeHttpUrl(item.original_url)
    );
  } catch {
    return [];
  }
}

function saveToHistory(link) {
  if (!link || !link.short_code || !isSafeHttpUrl(link.original_url)) return;
  const current = loadHistory().filter((item) => item.short_code !== link.short_code);
  const next = [
    {
      short_code: link.short_code,
      original_url: link.original_url,
      created_at: link.created_at || new Date().toISOString(),
    },
    ...current,
  ].slice(0, MAX_HISTORY);

  try {
    localStorage.setItem(HISTORY_KEY, JSON.stringify(next));
  } catch {
  }
}

async function copyToClipboard(text, triggerBtn = null) {
  try {
    await navigator.clipboard.writeText(text);
    if (triggerBtn) {
      flashButtonLabel(triggerBtn, 'Copiado!');
    }
    showToast(toastEl, 'Link copiado para a área de transferência.');
  } catch {
    showToast(toastEl, 'Não foi possível copiar automaticamente.');
  }
}

function renderHistory() {
  const items = loadHistory();
  clearElement(historyTbody);

  if (items.length === 0) {
    historySection.hidden = true;
    return;
  }

  historySection.hidden = false;

  for (const item of items) {
    const shortUrl = buildShortUrl(item.short_code);
    const copyBtn = el('button', {
      type: 'button',
      className: 'btn btn-secondary btn-sm',
      text: 'Copiar',
      attrs: { 'aria-label': `Copiar link curto ${item.short_code}` },
    });
    copyBtn.addEventListener('click', () => copyToClipboard(shortUrl, copyBtn));

    const tr = el(
      'tr',
      {},
      el('td', { attrs: { 'data-label': 'Link curto' } }, createSafeAnchor(shortUrl, item.short_code, 'short-code-link')),
      el(
        'td',
        { className: 'url-cell', title: item.original_url, attrs: { 'data-label': 'Destino' } },
        createSafeAnchor(item.original_url, item.original_url)
      ),
      el('td', { className: 'date-cell', text: formatDate(item.created_at), attrs: { 'data-label': 'Criado em' } }),
      el('td', { className: 'actions-cell' }, copyBtn)
    );
    historyTbody.appendChild(tr);
  }
}

async function showResult(link) {
  lastShortCode = link.short_code;
  lastShortUrl = buildShortUrl(link.short_code);

  clearElement(resultLinkSlot);
  resultLinkSlot.appendChild(createSafeAnchor(lastShortUrl, lastShortUrl));
  resultOriginal.textContent = `Destino: ${link.original_url}`;
  qrCanvas.setAttribute('aria-label', `QR Code do link curto ${link.short_code}`);
  resultCard.hidden = false;

  await renderQRCode(qrCanvas, lastShortUrl, 160);
}

form.addEventListener('submit', async (event) => {
  event.preventDefault();
  setError('');

  const rawUrl = urlInput.value.trim();
  if (!isSafeHttpUrl(rawUrl)) {
    setError('Informe uma URL válida iniciando com http:// ou https://');
    urlInput.focus();
    return;
  }

  shortenBtn.disabled = true;
  shortenBtn.textContent = 'Encurtando...';
  form.setAttribute('aria-busy', 'true');

  try {
    const link = await createLink(rawUrl, turnstileCtrl.getToken());
    await showResult(link);
    saveToHistory(link);
    renderHistory();
    urlInput.value = '';
    turnstileCtrl.reset();
  } catch (err) {
    setError(err.message || 'Erro ao encurtar URL.');
    turnstileCtrl.reset();
  } finally {
    shortenBtn.disabled = false;
    shortenBtn.textContent = 'Esmagar link';
    form.removeAttribute('aria-busy');
  }
});

urlInput.addEventListener('input', () => {
  if (!errorBox.hidden) {
    setError('');
  }
});

copyResultBtn.addEventListener('click', () => {
  if (lastShortUrl) {
    copyToClipboard(lastShortUrl, copyResultBtn);
  }
});

downloadQrBtn.addEventListener('click', () => {
  if (lastShortCode) {
    downloadQRCodePNG(qrCanvas, lastShortCode);
    showToast(toastEl, 'Download do QR Code iniciado.');
  }
});

clearHistoryBtn.addEventListener('click', () => {
  try {
    localStorage.removeItem(HISTORY_KEY);
  } catch {
  }
  renderHistory();
  showToast(toastEl, 'Histórico local limpo.');
});

renderHistory();
