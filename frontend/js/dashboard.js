import {
  createLink,
  deleteAccount,
  deleteLink,
  editLink,
  forgotPassword,
  getLinkStats,
  getMe,
  hasSession,
  listMyLinks,
  login,
  logout,
  onUnauthorized,
  register,
  resendVerification,
  restoreSession,
  updateMe,
} from './api.js';
import { downloadQRCodePNG, renderQRCode } from './qrcode.js';
import { mountTurnstile } from './turnstile.js';
import {
  buildShortUrl,
  clearElement,
  createSafeAnchor,
  el,
  formatDate,
  initPasswordToggles,
  initThemeToggle,
  isSafeHttpUrl,
  showToast,
} from './security.js';

const themeBtn = document.getElementById('theme-toggle');
const accountOpenBtn = document.getElementById('account-open-btn');
const logoutBtn = document.getElementById('logout-btn');
const deleteAccOpenBtn = document.getElementById('delete-account-open-btn');
const toastEl = document.getElementById('toast');

// Auth elements
const authSection = document.getElementById('auth-section');
const appSection = document.getElementById('app-section');
const tabLogin = document.getElementById('tab-login');
const tabRegister = document.getElementById('tab-register');
const authForm = document.getElementById('auth-form');
const authNameField = document.getElementById('auth-name-field');
const authName = document.getElementById('auth-name');
const authEmail = document.getElementById('auth-email');
const authPasswordField = document.getElementById('auth-password-field');
const authPassword = document.getElementById('auth-password');
const authPasswordConfirmField = document.getElementById('auth-password-confirm-field');
const authPasswordConfirm = document.getElementById('auth-password-confirm');
const authTurnstileEl = document.getElementById('auth-turnstile');
const authSubmitBtn = document.getElementById('auth-submit-btn');
const forgotToggleBtn = document.getElementById('forgot-toggle-btn');
const authError = document.getElementById('auth-error');
const authInfo = document.getElementById('auth-info');
const verifyPendingBox = document.getElementById('verify-pending-box');
const pendingEmailLabel = document.getElementById('pending-email-label');
const pendingBackBtn = document.getElementById('pending-back-btn');
const pendingResendBtn = document.getElementById('pending-resend-btn');

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

const accountModal = document.getElementById('account-modal');
const accountEmailLabel = document.getElementById('account-email-label');
const accountForm = document.getElementById('account-form');
const accountNameInput = document.getElementById('account-name-input');
const accountCurrentPass = document.getElementById('account-current-pass');
const accountNewPass = document.getElementById('account-new-pass');
const accountNewPassConfirm = document.getElementById('account-new-pass-confirm');
const accountError = document.getElementById('account-error');
const accountCancelBtn = document.getElementById('account-cancel-btn');
const accountSaveBtn = document.getElementById('account-save-btn');

let authMode = 'login';
let pendingEmail = '';
let authTurnstileCtrl = { enabled: false, getToken: () => '', reset: () => {} };
let selectedStatsCode = null;
let editingCode = null;
let qrCurrentCode = null;
let timelineChart = null;
let devicesChart = null;

initThemeToggle(themeBtn);
initPasswordToggles();
mountTurnstile(authTurnstileEl).then((ctrl) => {
  authTurnstileCtrl = ctrl;
  updateAuthUI();
});

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
  accountOpenBtn.hidden = !loggedIn;
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

function updateAuthUI() {
  verifyPendingBox.hidden = true;
  authForm.hidden = false;
  authInfo.hidden = true;
  setBoxError(authError, '');

  const isRegister = authMode === 'register';
  const isForgot = authMode === 'forgot';

  tabLogin.classList.toggle('active', authMode === 'login');
  tabRegister.classList.toggle('active', isRegister);

  authNameField.hidden = !isRegister;
  authPasswordField.hidden = isForgot;
  authPassword.required = !isForgot;
  authPasswordConfirmField.hidden = !isRegister;
  authPasswordConfirm.required = isRegister;
  authTurnstileEl.hidden = !((isRegister || isForgot) && authTurnstileCtrl.enabled);

  if (authMode === 'login') {
    authSubmitBtn.textContent = 'Entrar';
    authPassword.setAttribute('autocomplete', 'current-password');
    forgotToggleBtn.textContent = 'Esqueci minha senha';
  } else if (isRegister) {
    authSubmitBtn.textContent = 'Criar Conta';
    authPassword.setAttribute('autocomplete', 'new-password');
    forgotToggleBtn.textContent = 'Esqueci minha senha';
  } else {
    authSubmitBtn.textContent = 'Enviar link de recuperação';
    forgotToggleBtn.textContent = 'Voltar para o login';
  }
}

function showPendingVerification(email) {
  pendingEmail = email;
  pendingEmailLabel.textContent = email;
  authForm.hidden = true;
  verifyPendingBox.hidden = false;
  setBoxError(authError, '');
}

function startResendCooldown(seconds) {
  let remaining = seconds;
  pendingResendBtn.disabled = true;
  const timer = setInterval(() => {
    remaining -= 1;
    if (remaining <= 0) {
      clearInterval(timer);
      pendingResendBtn.disabled = false;
      pendingResendBtn.textContent = 'Reenviar e-mail';
      return;
    }
    pendingResendBtn.textContent = `Aguarde ${remaining}s`;
  }, 1000);
}

tabLogin.addEventListener('click', () => {
  authMode = 'login';
  updateAuthUI();
});

tabRegister.addEventListener('click', () => {
  authMode = 'register';
  updateAuthUI();
});

forgotToggleBtn.addEventListener('click', () => {
  authMode = authMode === 'forgot' ? 'login' : 'forgot';
  updateAuthUI();
});

pendingBackBtn.addEventListener('click', () => {
  authMode = 'login';
  updateAuthUI();
});

pendingResendBtn.addEventListener('click', async () => {
  if (!pendingEmail) return;
  setBoxError(authError, '');
  pendingResendBtn.disabled = true;
  try {
    await resendVerification(pendingEmail);
    showToast(toastEl, 'E-mail de confirmação reenviado.');
    startResendCooldown(60);
  } catch (err) {
    setBoxError(authError, err.message || 'Erro ao reenviar confirmação.');
    pendingResendBtn.disabled = false;
  }
});

authForm.addEventListener('submit', async (event) => {
  event.preventDefault();
  setBoxError(authError, '');
  authInfo.hidden = true;

  const email = authEmail.value.trim();
  const password = authPassword.value;
  authSubmitBtn.disabled = true;

  try {
    if (authMode === 'register') {
      if (password !== authPasswordConfirm.value) {
        setBoxError(authError, 'As senhas não coincidem.');
        authSubmitBtn.disabled = false;
        return;
      }
      await register(email, password, authName.value, authTurnstileCtrl.getToken());
      authPassword.value = '';
      authPasswordConfirm.value = '';
      authTurnstileCtrl.reset();
      showPendingVerification(email);
      return;
    }
    if (authMode === 'forgot') {
      await forgotPassword(email, authTurnstileCtrl.getToken());
      authTurnstileCtrl.reset();
      authInfo.textContent = 'Se o e-mail existir, enviamos as instruções para redefinir sua senha.';
      authInfo.hidden = false;
      return;
    }

    await login(email, password);
    authPassword.value = '';
    syncView();
    await loadLinks();
  } catch (err) {
    authTurnstileCtrl.reset();
    if (err.status === 403 && String(err.message).toLowerCase().includes('verificado')) {
      showPendingVerification(email);
    } else {
      setBoxError(authError, err.message || 'Falha na autenticação.');
    }
  } finally {
    authSubmitBtn.disabled = false;
  }
});

logoutBtn.addEventListener('click', async () => {
  await logout();
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
        link.is_active
          ? el('button', {
              type: 'button',
              className: 'btn btn-secondary btn-sm',
              text: 'Editar',
              onClick: () => openEditModal(link),
            })
          : null,
        el('button', {
          type: 'button',
          className: 'btn btn-primary btn-sm',
          text: 'Stats',
          onClick: () => loadStats(link.short_code),
        }),
        link.is_active
          ? el('button', {
              type: 'button',
              className: 'btn btn-danger btn-sm',
              text: 'Desativar',
              onClick: () => handleDeactivateLink(link.short_code),
            })
          : null
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

async function handleDeactivateLink(code) {
  if (!window.confirm(`Desativar o link ${code}? Ele deixará de redirecionar imediatamente.`)) {
    return;
  }
  try {
    await deleteLink(code);
    showToast(toastEl, `Link ${code} desativado.`);
    await loadLinks();
  } catch (err) {
    showToast(toastEl, err.message || 'Erro ao desativar link.');
  }
}

accountOpenBtn.addEventListener('click', async () => {
  setBoxError(accountError, '');
  accountCurrentPass.value = '';
  accountNewPass.value = '';
  accountNewPassConfirm.value = '';
  accountModal.hidden = false;
  try {
    const me = await getMe();
    accountEmailLabel.textContent = `${me.email} (${me.status})`;
    accountNameInput.value = me.name || '';
  } catch (err) {
    setBoxError(accountError, err.message || 'Erro ao carregar perfil.');
  }
});

accountCancelBtn.addEventListener('click', () => {
  accountModal.hidden = true;
});

accountForm.addEventListener('submit', async (event) => {
  event.preventDefault();
  setBoxError(accountError, '');
  accountSaveBtn.disabled = true;

  const payload = { name: accountNameInput.value.trim() };
  if (accountNewPass.value) {
    if (accountNewPass.value !== accountNewPassConfirm.value) {
      setBoxError(accountError, 'A nova senha e a confirmação não coincidem.');
      accountSaveBtn.disabled = false;
      return;
    }
    payload.current_password = accountCurrentPass.value;
    payload.new_password = accountNewPass.value;
  }

  try {
    await updateMe(payload);
    accountModal.hidden = true;
    showToast(toastEl, 'Perfil atualizado com sucesso.');
  } catch (err) {
    setBoxError(accountError, err.message || 'Erro ao atualizar perfil.');
  } finally {
    accountSaveBtn.disabled = false;
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

for (const modal of [editModal, qrModal, deleteModal, accountModal]) {
  modal.addEventListener('click', (event) => {
    if (event.target === modal) {
      modal.hidden = true;
    }
  });
}

document.addEventListener('keydown', (event) => {
  if (event.key !== 'Escape') return;
  for (const modal of [editModal, qrModal, deleteModal, accountModal]) {
    if (!modal.hidden) {
      modal.hidden = true;
    }
  }
});

(async () => {
  if (!hasSession()) {
    await restoreSession();
  }
  syncView();
  if (hasSession()) {
    loadLinks();
  }
})();
