import {
  createLink,
  deleteAccount,
  editLink,
  getLinkStats,
  hasSession,
  listMyLinks,
  login,
  logout,
  onUnauthorized,
  register,
} from './api.js';
import { downloadQRCodePNG, renderQRCode } from './qrcode.js';
import {
  buildShortUrl,
  clearElement,
  createSafeAnchor,
  el,
  formatDate,
  initThemeToggle,
  isSafeHttpUrl,
  showToast,
} from './security.js';

const themeBtn = document.getElementById('theme-toggle');
const logoutBtn = document.getElementById('logout-btn');
const deleteAccOpenBtn = document.getElementById('delete-account-open-btn');
const toastEl = document.getElementById('toast');

// Auth elements
const authSection = document.getElementById('auth-section');
const appSection = document.getElementById('app-section');
const tabLogin = document.getElementById('tab-login');
const tabRegister = document.getElementById('tab-register');
const authForm = document.getElementById('auth-form');
const authEmail = document.getElementById('auth-email');
const authPassword = document.getElementById('auth-password');
const authSubmitBtn = document.getElementById('auth-submit-btn');
const authError = document.getElementById('auth-error');

// Create & list elements
const createForm = document.getElementById('dash-create-form');
const createUrlInput = document.getElementById('dash-url-input');
const createBtn = document.getElementById('dash-create-btn');
const createError = document.getElementById('dash-create-error');
const refreshLinksBtn = document.getElementById('refresh-links-btn');
const linksEmpty = document.getElementById('links-empty');
const linksTableWrap = document.getElementById('links-table-wrap');
const linksTbody = document.getElementById('links-tbody');

// Stats elements
const statsSection = document.getElementById('stats-section');
const statsCodeLabel = document.getElementById('stats-code-label');
const statsDaysSelect = document.getElementById('stats-days-select');
const statsError = document.getElementById('stats-error');
const statsTotalClicks = document.getElementById('stats-total-clicks');
const timelineCanvas = document.getElementById('chart-timeline');
const devicesCanvas = document.getElementById('chart-devices');
const referrersList = document.getElementById('stats-referrers');
const countriesList = document.getElementById('stats-countries');

// Modals
const editModal = document.getElementById('edit-modal');
const editForm = document.getElementById('edit-form');
const editCodeLabel = document.getElementById('edit-code-label');
const editUrlInput = document.getElementById('edit-url-input');
const editError = document.getElementById('edit-error');
const editCancelBtn = document.getElementById('edit-cancel-btn');

const qrModal = document.getElementById('qr-modal');
const qrCodeLabel = document.getElementById('qr-code-label');
const modalQrCanvas = document.getElementById('modal-qr-canvas');
const qrCloseBtn = document.getElementById('qr-close-btn');
const qrDownloadBtn = document.getElementById('qr-download-btn');

const deleteModal = document.getElementById('delete-account-modal');
const keepLinksCheckbox = document.getElementById('keep-links-checkbox');
const deleteError = document.getElementById('delete-error');
const deleteCancelBtn = document.getElementById('delete-cancel-btn');
const deleteConfirmBtn = document.getElementById('delete-confirm-btn');

let authMode = 'login';
let selectedStatsCode = null;
let editingCode = null;
let qrCurrentCode = null;
let timelineChart = null;
let devicesChart = null;

initThemeToggle(themeBtn);

function setBoxError(box, msg) {
  if (!msg) {
    box.hidden = true;
    box.textContent = '';
    return;
  }
  box.textContent = msg;
  box.hidden = false;
}

function syncView() {
  const loggedIn = hasSession();
  authSection.hidden = loggedIn;
  appSection.hidden = !loggedIn;
  logoutBtn.hidden = !loggedIn;
  deleteAccOpenBtn.hidden = !loggedIn;

  if (!loggedIn) {
    statsSection.hidden = true;
    selectedStatsCode = null;
    destroyCharts();
  }
}

onUnauthorized(() => {
  syncView();
  setBoxError(authError, 'Sessão expirada. Faça login novamente.');
});

tabLogin.addEventListener('click', () => {
  authMode = 'login';
  tabLogin.classList.add('active');
  tabRegister.classList.remove('active');
  authSubmitBtn.textContent = 'Entrar';
  authPassword.setAttribute('autocomplete', 'current-password');
  setBoxError(authError, '');
});

tabRegister.addEventListener('click', () => {
  authMode = 'register';
  tabRegister.classList.add('active');
  tabLogin.classList.remove('active');
  authSubmitBtn.textContent = 'Criar Conta';
  authPassword.setAttribute('autocomplete', 'new-password');
  setBoxError(authError, '');
});

authForm.addEventListener('submit', async (event) => {
  event.preventDefault();
  setBoxError(authError, '');

  const email = authEmail.value.trim();
  const password = authPassword.value;
  authSubmitBtn.disabled = true;

  try {
    if (authMode === 'register') {
      await register(email, password);
      showToast(toastEl, 'Conta criada! Autenticando...');
    }
    await login(email, password);
    authPassword.value = '';
    syncView();
    await loadLinks();
  } catch (err) {
    setBoxError(authError, err.message || 'Falha na autenticação.');
  } finally {
    authSubmitBtn.disabled = false;
  }
});

logoutBtn.addEventListener('click', () => {
  logout();
  syncView();
  showToast(toastEl, 'Sessão encerrada.');
});

async function copyUrl(text) {
  try {
    await navigator.clipboard.writeText(text);
    showToast(toastEl, 'Link copiado.');
  } catch {
    showToast(toastEl, 'Não foi possível copiar.');
  }
}

function openEditModal(link) {
  editingCode = link.short_code;
  editCodeLabel.textContent = link.short_code;
  editUrlInput.value = link.original_url;
  setBoxError(editError, '');
  editModal.hidden = false;
}

async function openQrModal(code) {
  qrCurrentCode = code;
  qrCodeLabel.textContent = code;
  qrModal.hidden = false;
  await renderQRCode(modalQrCanvas, buildShortUrl(code), 220);
}

function renderLinksTable(links) {
  clearElement(linksTbody);

  if (!Array.isArray(links) || links.length === 0) {
    linksEmpty.hidden = false;
    linksTableWrap.hidden = true;
    statsSection.hidden = true;
    return;
  }

  linksEmpty.hidden = true;
  linksTableWrap.hidden = false;

  for (const link of links) {
    const shortUrl = buildShortUrl(link.short_code);
    const statusPill = el('span', {
      className: `status-pill ${link.is_active ? 'status-active' : 'status-inactive'}`,
      text: link.is_active ? 'Ativo' : 'Inativo',
    });

    const tr = el(
      'tr',
      {},
      el('td', {}, createSafeAnchor(shortUrl, link.short_code)),
      el(
        'td',
        { className: 'url-cell', title: link.original_url },
        createSafeAnchor(link.original_url, link.original_url)
      ),
      el('td', {}, statusPill),
      el('td', { text: formatDate(link.created_at) }),
      el(
        'td',
        { className: 'actions-cell' },
        el('button', {
          type: 'button',
          className: 'btn btn-secondary btn-sm',
          text: 'Copiar',
          onClick: () => copyUrl(shortUrl),
        }),
        el('button', {
          type: 'button',
          className: 'btn btn-secondary btn-sm',
          text: 'QR',
          onClick: () => openQrModal(link.short_code),
        }),
        el('button', {
          type: 'button',
          className: 'btn btn-secondary btn-sm',
          text: 'Editar',
          onClick: () => openEditModal(link),
        }),
        el('button', {
          type: 'button',
          className: 'btn btn-primary btn-sm',
          text: 'Stats',
          onClick: () => loadStats(link.short_code),
        })
      )
    );
    linksTbody.appendChild(tr);
  }
}

async function loadLinks() {
  try {
    const links = await listMyLinks();
    renderLinksTable(links);
    if (links && links.length > 0) {
      const targetCode =
        selectedStatsCode && links.some((l) => l.short_code === selectedStatsCode)
          ? selectedStatsCode
          : links[0].short_code;
      await loadStats(targetCode);
    }
  } catch (err) {
    showToast(toastEl, err.message || 'Erro ao carregar links.');
  }
}

function destroyCharts() {
  if (timelineChart) {
    timelineChart.destroy();
    timelineChart = null;
  }
  if (devicesChart) {
    devicesChart.destroy();
    devicesChart = null;
  }
}

function buildDateLabels(days) {
  const labels = [];
  const now = new Date();
  for (let i = days - 1; i >= 0; i--) {
    const d = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate() - i));
    labels.push(d.toISOString().slice(0, 10));
  }
  return labels;
}

function renderBreakdownList(listEl, dataMap, emptyLabel) {
  clearElement(listEl);
  const entries = Object.entries(dataMap || {}).sort((a, b) => b[1] - a[1]).slice(0, 8);
  if (entries.length === 0) {
    listEl.appendChild(el('li', { className: 'muted', text: emptyLabel }));
    return;
  }
  for (const [key, count] of entries) {
    listEl.appendChild(
      el('li', {}, el('span', { text: key }), el('strong', { text: String(count) }))
    );
  }
}

function renderCharts(stats, days) {
  if (typeof window.Chart === 'undefined') return;
  destroyCharts();

  const labels = buildDateLabels(days);
  const dateCounts = labels.map((day) => (stats.clicks_by_date && stats.clicks_by_date[day]) || 0);

  timelineChart = new window.Chart(timelineCanvas.getContext('2d'), {
    type: 'line',
    data: {
      labels,
      datasets: [
        {
          label: 'Cliques por dia',
          data: dateCounts,
          borderColor: '#0ea5e9',
          backgroundColor: 'rgba(14, 165, 233, 0.16)',
          tension: 0.25,
          fill: true,
        },
      ],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      scales: {
        y: { beginAtZero: true, ticks: { precision: 0 } },
      },
    },
  });

  const deviceEntries = Object.entries(stats.clicks_by_device || {});
  const deviceLabels = deviceEntries.length ? deviceEntries.map(([k]) => k) : ['sem dados'];
  const deviceValues = deviceEntries.length ? deviceEntries.map(([, v]) => v) : [0];

  devicesChart = new window.Chart(devicesCanvas.getContext('2d'), {
    type: 'doughnut',
    data: {
      labels: deviceLabels,
      datasets: [
        {
          data: deviceValues,
          backgroundColor: ['#0ea5e9', '#22c55e', '#f59e0b', '#94a3b8'],
        },
      ],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
    },
  });
}

async function loadStats(code) {
  selectedStatsCode = code;
  const days = Number(statsDaysSelect.value) || 7;
  statsSection.hidden = false;
  statsCodeLabel.textContent = code;
  setBoxError(statsError, '');

  try {
    const stats = await getLinkStats(code, days);
    statsTotalClicks.textContent = String(stats.total_clicks ?? 0);
    renderCharts(stats, days);
    renderBreakdownList(referrersList, stats.top_referrers, 'Nenhum referrer registrado');
    renderBreakdownList(countriesList, stats.clicks_by_country, 'Nenhum país registrado');
  } catch (err) {
    setBoxError(statsError, err.message || 'Erro ao carregar estatísticas.');
  }
}

createForm.addEventListener('submit', async (event) => {
  event.preventDefault();
  setBoxError(createError, '');

  const rawUrl = createUrlInput.value.trim();
  if (!isSafeHttpUrl(rawUrl)) {
    setBoxError(createError, 'Informe uma URL válida iniciando com http:// ou https://');
    return;
  }

  createBtn.disabled = true;
  try {
    const created = await createLink(rawUrl);
    createUrlInput.value = '';
    selectedStatsCode = created.short_code;
    await loadLinks();
    showToast(toastEl, 'Link criado com sucesso.');
  } catch (err) {
    setBoxError(createError, err.message || 'Erro ao criar link.');
  } finally {
    createBtn.disabled = false;
  }
});

refreshLinksBtn.addEventListener('click', loadLinks);

statsDaysSelect.addEventListener('change', () => {
  if (selectedStatsCode) {
    loadStats(selectedStatsCode);
  }
});

editCancelBtn.addEventListener('click', () => {
  editModal.hidden = true;
});

editForm.addEventListener('submit', async (event) => {
  event.preventDefault();
  setBoxError(editError, '');

  const nextUrl = editUrlInput.value.trim();
  if (!isSafeHttpUrl(nextUrl)) {
    setBoxError(editError, 'Informe uma URL válida iniciando com http:// ou https://');
    return;
  }

  try {
    await editLink(editingCode, nextUrl);
    editModal.hidden = true;
    await loadLinks();
    showToast(toastEl, 'URL de destino atualizada.');
  } catch (err) {
    setBoxError(editError, err.message || 'Erro ao editar link.');
  }
});

qrCloseBtn.addEventListener('click', () => {
  qrModal.hidden = true;
});

qrDownloadBtn.addEventListener('click', () => {
  if (qrCurrentCode) {
    downloadQRCodePNG(modalQrCanvas, qrCurrentCode);
  }
});

deleteAccOpenBtn.addEventListener('click', () => {
  setBoxError(deleteError, '');
  keepLinksCheckbox.checked = true;
  deleteModal.hidden = false;
});

deleteCancelBtn.addEventListener('click', () => {
  deleteModal.hidden = true;
});

deleteConfirmBtn.addEventListener('click', async () => {
  setBoxError(deleteError, '');
  deleteConfirmBtn.disabled = true;
  try {
    await deleteAccount(keepLinksCheckbox.checked);
    deleteModal.hidden = true;
    syncView();
    showToast(toastEl, 'Conta excluída com sucesso.');
  } catch (err) {
    setBoxError(deleteError, err.message || 'Erro ao excluir conta.');
  } finally {
    deleteConfirmBtn.disabled = false;
  }
});

syncView();
if (hasSession()) {
  loadLinks();
}
