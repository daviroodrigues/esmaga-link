import {
  createLink,
  deleteAccount,
  deleteLink,
  editLink,
  forgotPassword,
  getLinkStats,
  getMe,
  getPublicConfig,
  hasSession,
  listMyLinks,
  login,
  logout,
  onUnauthorized,
  register,
  resendVerification,
  restoreSession,
  setLinkStatus,
  updateMe,
} from './api.js';
import { downloadQRCodePNG, renderQRCode } from './qrcode.js';
import { mountTurnstile } from './turnstile.js';
import {
  applyPortfolioVisibility,
  buildShortUrl,
  clearElement,
  createModalController,
  createSafeAnchor,
  el,
  flashButtonLabel,
  formatDate,
  initPasswordToggles,
  initThemeToggle,
  isDarkTheme,
  isSafeHttpUrl,
  showToast,
} from './security.js';

const themeBtn = document.getElementById('theme-toggle');
const accountOpenBtn = document.getElementById('account-open-btn');
const logoutBtn = document.getElementById('logout-btn');
const deleteAccOpenBtn = document.getElementById('delete-account-open-btn');
const toastEl = document.getElementById('toast');

const authSection = document.getElementById('auth-section');
const authHeading = document.getElementById('auth-heading');
const authSubtitle = document.getElementById('auth-subtitle');
const authTabs = document.getElementById('auth-tabs');
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
const registerTurnstileEl = document.getElementById('register-turnstile');
const forgotTurnstileEl = document.getElementById('forgot-turnstile');
const authSubmitBtn = document.getElementById('auth-submit-btn');
const forgotToggleBtn = document.getElementById('forgot-toggle-btn');
const forgotBackWrap = document.getElementById('forgot-back-wrap');
const forgotBackBtn = document.getElementById('forgot-back-btn');
const authError = document.getElementById('auth-error');
const authInfo = document.getElementById('auth-info');
const verifyPendingBox = document.getElementById('verify-pending-box');
const pendingEmailLabel = document.getElementById('pending-email-label');
const pendingBackBtn = document.getElementById('pending-back-btn');
const pendingResendBtn = document.getElementById('pending-resend-btn');

const summaryTotal = document.getElementById('summary-total');
const summaryActive = document.getElementById('summary-active');

const createForm = document.getElementById('dash-create-form');
const createUrlInput = document.getElementById('dash-url-input');
const createBtn = document.getElementById('dash-create-btn');
const createError = document.getElementById('dash-create-error');
const refreshLinksBtn = document.getElementById('refresh-links-btn');
const linksEmpty = document.getElementById('links-empty');
const linksTableWrap = document.getElementById('links-table-wrap');
const linksTbody = document.getElementById('links-tbody');

const statsSection = document.getElementById('stats-section');
const statsCodeLabel = document.getElementById('stats-code-label');
const statsDaysSelect = document.getElementById('stats-days-select');
const statsError = document.getElementById('stats-error');
const statsTotalClicks = document.getElementById('stats-total-clicks');
const statsAvgClicks = document.getElementById('stats-avg-clicks');
const statsTopDevice = document.getElementById('stats-top-device');
const timelineCanvas = document.getElementById('chart-timeline');
const devicesCanvas = document.getElementById('chart-devices');
const devicesCanvasWrap = document.getElementById('devices-canvas-wrap');
const devicesEmpty = document.getElementById('devices-empty');
const referrersList = document.getElementById('stats-referrers');
const countriesList = document.getElementById('stats-countries');

const editModal = document.getElementById('edit-modal');
const editForm = document.getElementById('edit-form');
const editCodeLabel = document.getElementById('edit-code-label');
const editUrlInput = document.getElementById('edit-url-input');
const editError = document.getElementById('edit-error');
const editCancelBtn = document.getElementById('edit-cancel-btn');
const editSaveBtn = document.getElementById('edit-save-btn');

const qrModal = document.getElementById('qr-modal');
const qrCodeLabel = document.getElementById('qr-code-label');
const modalQrCanvas = document.getElementById('modal-qr-canvas');
const qrCloseBtn = document.getElementById('qr-close-btn');
const qrDownloadBtn = document.getElementById('qr-download-btn');

const deleteLinkModal = document.getElementById('delete-link-modal');
const deleteLinkCodeLabel = document.getElementById('delete-link-code-label');
const deleteLinkError = document.getElementById('delete-link-error');
const deleteLinkCancelBtn = document.getElementById('delete-link-cancel-btn');
const deleteLinkConfirmBtn = document.getElementById('delete-link-confirm-btn');

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

const modalCtrl = createModalController([
  editModal,
  qrModal,
  deleteLinkModal,
  deleteModal,
  accountModal,
]);

let authMode = 'login';
let pendingEmail = '';
let registerTurnstileCtrl = { enabled: false, getToken: () => '', reset: () => {} };
let forgotTurnstileCtrl = { enabled: false, getToken: () => '', reset: () => {} };
let cachedLinks = [];
let selectedStatsCode = null;
let lastStatsPayload = null;
let lastStatsDays = 7;
let editingCode = null;
let deletingLinkCode = null;
let deleteLinkCountdownTimer = null;
let qrCurrentCode = null;
let timelineChart = null;
let devicesChart = null;

initThemeToggle(themeBtn);
initPasswordToggles();
getPublicConfig().then((cfg) => applyPortfolioVisibility(cfg));
Promise.all([
  mountTurnstile(registerTurnstileEl, { action: 'register' }),
  mountTurnstile(forgotTurnstileEl, { action: 'forgot_password' }),
]).then(([regCtrl, forgotCtrl]) => {
  registerTurnstileCtrl = regCtrl;
  forgotTurnstileCtrl = forgotCtrl;
  updateAuthUI();
});

function setBoxError(box, msg, relatedInput = null) {
  if (!msg) {
    box.hidden = true;
    box.textContent = '';
    if (relatedInput) relatedInput.removeAttribute('aria-invalid');
    return;
  }
  box.textContent = msg;
  box.hidden = false;
  if (relatedInput) relatedInput.setAttribute('aria-invalid', 'true');
}

function syncView() {
  const loggedIn = hasSession();
  authSection.hidden = loggedIn;
  appSection.hidden = !loggedIn;
  accountOpenBtn.hidden = !loggedIn;
  logoutBtn.hidden = !loggedIn;

  if (!loggedIn) {
    statsSection.hidden = true;
    selectedStatsCode = null;
    lastStatsPayload = null;
    destroyCharts();
  }
}

onUnauthorized(() => {
  syncView();
  setBoxError(authError, 'Sua sessão expirou. Entre novamente para continuar.');
});

function updateAuthUI() {
  verifyPendingBox.hidden = true;
  authForm.hidden = false;
  authInfo.hidden = true;
  setBoxError(authError, '');

  const isLogin = authMode === 'login';
  const isRegister = authMode === 'register';
  const isForgot = authMode === 'forgot';

  authTabs.hidden = isForgot;
  tabLogin.classList.toggle('active', isLogin);
  tabRegister.classList.toggle('active', isRegister);
  tabLogin.setAttribute('aria-selected', String(isLogin));
  tabRegister.setAttribute('aria-selected', String(isRegister));
  tabLogin.tabIndex = isLogin ? 0 : -1;
  tabRegister.tabIndex = isRegister ? 0 : -1;
  authForm.setAttribute('aria-labelledby', isRegister ? 'tab-register' : 'tab-login');

  authNameField.hidden = !isRegister;
  authPasswordField.hidden = isForgot;
  authPassword.required = !isForgot;
  authPasswordConfirmField.hidden = !isRegister;
  authPasswordConfirm.required = isRegister;
  forgotToggleBtn.hidden = !isLogin;
  forgotBackWrap.hidden = !isForgot;
  registerTurnstileEl.hidden = !(isRegister && registerTurnstileCtrl.enabled);
  forgotTurnstileEl.hidden = !(isForgot && forgotTurnstileCtrl.enabled);

  if (isLogin) {
    authHeading.textContent = 'Acesse sua conta';
    authSubtitle.textContent = 'Gerencie seus links, altere destinos e acompanhe os cliques.';
    authSubmitBtn.textContent = 'Entrar';
    authPassword.setAttribute('autocomplete', 'current-password');
  } else if (isRegister) {
    authHeading.textContent = 'Crie sua conta';
    authSubtitle.textContent = 'Gratuito para gerenciar seus links e visualizar estatísticas.';
    authSubmitBtn.textContent = 'Criar conta';
    authPassword.setAttribute('autocomplete', 'new-password');
  } else {
    authHeading.textContent = 'Recuperar senha';
    authSubtitle.textContent = 'Informe seu e-mail para receber o link de redefinição.';
    authSubmitBtn.textContent = 'Enviar link de recuperação';
  }
}

function showPendingVerification(email) {
  pendingEmail = email;
  pendingEmailLabel.textContent = email;
  authHeading.textContent = 'Verifique sua caixa de entrada';
  authSubtitle.textContent = 'Falta só confirmar seu endereço de e-mail.';
  authTabs.hidden = true;
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

authTabs.addEventListener('keydown', (event) => {
  if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;
  event.preventDefault();
  if (event.key === 'Home' || (event.key === 'ArrowLeft' && authMode === 'register') || (event.key === 'ArrowRight' && authMode === 'register')) {
    authMode = 'login';
    updateAuthUI();
    tabLogin.focus();
  } else {
    authMode = 'register';
    updateAuthUI();
    tabRegister.focus();
  }
});

forgotToggleBtn.addEventListener('click', () => {
  authMode = 'forgot';
  updateAuthUI();
  authEmail.focus();
});

forgotBackBtn.addEventListener('click', () => {
  authMode = 'login';
  updateAuthUI();
  authEmail.focus();
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
  authForm.setAttribute('aria-busy', 'true');

  try {
    if (authMode === 'register') {
      if (password !== authPasswordConfirm.value) {
        setBoxError(authError, 'As senhas não coincidem.', authPasswordConfirm);
        authPasswordConfirm.focus();
        return;
      }
      if (registerTurnstileCtrl.enabled && !registerTurnstileCtrl.getToken()) {
        setBoxError(authError, 'Aguarde a verificação anti-bot concluir e tente novamente.');
        return;
      }
      await register(email, password, authName.value, registerTurnstileCtrl.getToken());
      authPassword.value = '';
      authPasswordConfirm.value = '';
      registerTurnstileCtrl.reset();
      showPendingVerification(email);
      return;
    }
    if (authMode === 'forgot') {
      if (forgotTurnstileCtrl.enabled && !forgotTurnstileCtrl.getToken()) {
        setBoxError(authError, 'Aguarde a verificação anti-bot concluir e tente novamente.');
        return;
      }
      await forgotPassword(email, forgotTurnstileCtrl.getToken());
      forgotTurnstileCtrl.reset();
      authInfo.textContent = 'Se o e-mail estiver cadastrado, enviamos as instruções para redefinir sua senha.';
      authInfo.hidden = false;
      return;
    }

    await login(email, password);
    authPassword.value = '';
    syncView();
    await loadLinks();
  } catch (err) {
    if (authMode === 'register') {
      registerTurnstileCtrl.reset();
    } else if (authMode === 'forgot') {
      forgotTurnstileCtrl.reset();
    }
    if (err.status === 403 && String(err.message).toLowerCase().includes('verificado')) {
      showPendingVerification(email);
    } else {
      setBoxError(authError, err.message || 'Falha na autenticação.');
    }
  } finally {
    authSubmitBtn.disabled = false;
    authForm.removeAttribute('aria-busy');
  }
});

logoutBtn.addEventListener('click', async () => {
  await logout();
  syncView();
  showToast(toastEl, 'Sessão encerrada.');
});

async function copyUrl(text, triggerBtn = null) {
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

function openEditModal(link) {
  editingCode = link.short_code;
  editCodeLabel.textContent = link.short_code;
  editUrlInput.value = link.original_url;
  setBoxError(editError, '', editUrlInput);
  modalCtrl.open(editModal, { initialFocus: editUrlInput });
}

async function openQrModal(code) {
  qrCurrentCode = code;
  qrCodeLabel.textContent = code;
  modalQrCanvas.setAttribute('aria-label', `QR Code do link curto ${code}`);
  modalCtrl.open(qrModal, { initialFocus: qrDownloadBtn });
  await renderQRCode(modalQrCanvas, buildShortUrl(code), 220);
}

function clearDeleteLinkCountdown() {
  if (deleteLinkCountdownTimer) {
    clearInterval(deleteLinkCountdownTimer);
    deleteLinkCountdownTimer = null;
  }
}

function openDeleteLinkModal(code) {
  clearDeleteLinkCountdown();
  deletingLinkCode = code;
  deleteLinkCodeLabel.textContent = code;
  setBoxError(deleteLinkError, '');

  let remaining = 3;
  deleteLinkConfirmBtn.disabled = true;
  deleteLinkConfirmBtn.textContent = `Aguarde ${remaining}s`;

  modalCtrl.open(deleteLinkModal, { initialFocus: deleteLinkCancelBtn });

  deleteLinkCountdownTimer = setInterval(() => {
    if (deleteLinkModal.hidden) {
      clearDeleteLinkCountdown();
      deleteLinkConfirmBtn.textContent = 'Excluir permanentemente';
      return;
    }
    remaining -= 1;
    if (remaining <= 0) {
      clearDeleteLinkCountdown();
      deleteLinkConfirmBtn.disabled = false;
      deleteLinkConfirmBtn.textContent = 'Excluir permanentemente';
      return;
    }
    deleteLinkConfirmBtn.textContent = `Aguarde ${remaining}s`;
  }, 1000);
}

function createTrashIcon() {
  const ns = 'http://www.w3.org/2000/svg';
  const svg = document.createElementNS(ns, 'svg');
  svg.setAttribute('viewBox', '0 0 24 24');
  svg.setAttribute('fill', 'none');
  svg.setAttribute('stroke', 'currentColor');
  svg.setAttribute('stroke-width', '2');
  svg.setAttribute('stroke-linecap', 'round');
  svg.setAttribute('stroke-linejoin', 'round');
  svg.setAttribute('aria-hidden', 'true');

  const parts = [
    ['polyline', { points: '3 6 5 6 21 6' }],
    ['path', { d: 'M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6' }],
    ['path', { d: 'M10 11v6' }],
    ['path', { d: 'M14 11v6' }],
    ['path', { d: 'M9 6V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2' }],
  ];

  for (const [tag, attrs] of parts) {
    const node = document.createElementNS(ns, tag);
    for (const [k, v] of Object.entries(attrs)) {
      node.setAttribute(k, v);
    }
    svg.appendChild(node);
  }
  return svg;
}

async function handleToggleLinkStatus(link, switchEl, labelEl) {
  const nextActive = !link.is_active;
  switchEl.disabled = true;
  switchEl.setAttribute('aria-busy', 'true');
  switchEl.setAttribute('aria-checked', String(nextActive));
  if (labelEl) {
    labelEl.textContent = nextActive ? 'Ativando...' : 'Pausando...';
  }
  try {
    await setLinkStatus(link.short_code, nextActive);
    showToast(toastEl, nextActive ? `Link ${link.short_code} ativado.` : `Link ${link.short_code} pausado.`);
    await loadLinks();
  } catch (err) {
    showToast(toastEl, err.message || 'Erro ao alterar status do link.');
    switchEl.disabled = false;
    switchEl.removeAttribute('aria-busy');
    switchEl.setAttribute('aria-checked', String(link.is_active));
    if (labelEl) {
      labelEl.textContent = link.is_active ? 'Ativo' : 'Pausado';
    }
  }
}

function updateSummaryCounters(links) {
  const list = Array.isArray(links) ? links : [];
  const activeCount = list.filter((l) => l.is_active).length;
  summaryTotal.textContent = list.length.toLocaleString('pt-BR');
  summaryActive.textContent = activeCount.toLocaleString('pt-BR');
}

function renderLinksTable(links) {
  cachedLinks = Array.isArray(links) ? links : [];
  updateSummaryCounters(cachedLinks);
  clearElement(linksTbody);

  if (cachedLinks.length === 0) {
    linksEmpty.hidden = false;
    linksTableWrap.hidden = true;
    statsSection.hidden = true;
    return;
  }

  linksEmpty.hidden = true;
  linksTableWrap.hidden = false;

  for (const link of cachedLinks) {
    const shortUrl = buildShortUrl(link.short_code);
    const isSelected = selectedStatsCode === link.short_code;

    const statusLabel = el('span', {
      className: `status-label ${link.is_active ? 'status-label-active' : 'status-label-inactive'}`,
      text: link.is_active ? 'Ativo' : 'Pausado',
    });

    const statusSwitch = el(
      'button',
      {
        type: 'button',
        className: 'link-switch',
        attrs: {
          role: 'switch',
          'aria-checked': String(Boolean(link.is_active)),
          'aria-label': link.is_active
            ? `Desativar link ${link.short_code}`
            : `Ativar link ${link.short_code}`,
          'data-tooltip': link.is_active ? 'Desativar link' : 'Ativar link',
        },
        onClick: (event) => handleToggleLinkStatus(link, event.currentTarget, statusLabel),
      },
      el('span', { className: 'link-switch-thumb', attrs: { 'aria-hidden': 'true' } })
    );

    const statusControl = el('div', { className: 'status-control' }, statusSwitch, statusLabel);

    const copyBtn = el('button', {
      type: 'button',
      className: 'btn btn-secondary btn-sm',
      text: 'Copiar',
      attrs: {
        'aria-label': `Copiar link curto ${link.short_code}`,
        'data-tooltip': 'Copiar link curto',
      },
    });
    copyBtn.addEventListener('click', () => copyUrl(shortUrl, copyBtn));

    const statsBtn = el('button', {
      type: 'button',
      className: isSelected ? 'btn btn-primary btn-sm' : 'btn btn-secondary btn-sm',
      text: 'Métricas',
      attrs: {
        'aria-label': `Ver estatísticas do link ${link.short_code}`,
        'aria-pressed': String(isSelected),
        'data-tooltip': 'Ver estatísticas de cliques',
      },
      onClick: () => loadStats(link.short_code),
    });

    const deleteBtn = el(
      'button',
      {
        type: 'button',
        className: 'icon-btn icon-btn-sm icon-btn-danger',
        attrs: {
          'aria-label': `Excluir permanentemente o link ${link.short_code}`,
          'data-tooltip': 'Excluir link permanentemente',
          'data-tooltip-align': 'end',
        },
        onClick: () => openDeleteLinkModal(link.short_code),
      },
      createTrashIcon()
    );

    const tr = el(
      'tr',
      { className: isSelected ? 'row-selected' : '' },
      el('td', { attrs: { 'data-label': 'Link curto' } }, createSafeAnchor(shortUrl, link.short_code, 'short-code-link')),
      el(
        'td',
        { className: 'url-cell', title: link.original_url, attrs: { 'data-label': 'URL de destino' } },
        createSafeAnchor(link.original_url, link.original_url)
      ),
      el('td', { attrs: { 'data-label': 'Status' } }, statusControl),
      el('td', { className: 'date-cell', text: formatDate(link.created_at), attrs: { 'data-label': 'Criado em' } }),
      el(
        'td',
        { className: 'actions-cell' },
        el(
          'div',
          { className: 'actions-group' },
          copyBtn,
          el('button', {
            type: 'button',
            className: 'btn btn-secondary btn-sm',
            text: 'QR Code',
            attrs: {
              'aria-label': `Abrir QR Code do link ${link.short_code}`,
              'data-tooltip': 'Abrir QR Code',
            },
            onClick: () => openQrModal(link.short_code),
          }),
          el('button', {
            type: 'button',
            className: 'btn btn-secondary btn-sm',
            text: 'Editar',
            attrs: {
              'aria-label': `Editar destino do link ${link.short_code}`,
              'data-tooltip': 'Editar URL de destino',
            },
            onClick: () => openEditModal(link),
          }),
          statsBtn,
          deleteBtn
        )
      )
    );
    linksTbody.appendChild(tr);
  }
}

async function loadLinks() {
  refreshLinksBtn.disabled = true;
  try {
    const links = await listMyLinks();
    if (links && links.length > 0) {
      const targetCode =
        selectedStatsCode && links.some((l) => l.short_code === selectedStatsCode)
          ? selectedStatsCode
          : links[0].short_code;
      selectedStatsCode = targetCode;
      renderLinksTable(links);
      await loadStats(targetCode);
    } else {
      selectedStatsCode = null;
      renderLinksTable([]);
    }
  } catch (err) {
    showToast(toastEl, err.message || 'Erro ao carregar links.');
  } finally {
    refreshLinksBtn.disabled = false;
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

function buildDateRange(days) {
  const iso = [];
  const display = [];
  const now = new Date();
  for (let i = days - 1; i >= 0; i--) {
    const d = new Date(now.getFullYear(), now.getMonth(), now.getDate() - i);
    const yyyy = d.getFullYear();
    const mm = String(d.getMonth() + 1).padStart(2, '0');
    const dd = String(d.getDate()).padStart(2, '0');
    iso.push(`${yyyy}-${mm}-${dd}`);
    display.push(`${dd}/${mm}`);
  }
  return { iso, display };
}

function renderBreakdownList(listEl, dataMap, emptyLabel) {
  clearElement(listEl);
  const entries = Object.entries(dataMap || {})
    .sort((a, b) => b[1] - a[1])
    .slice(0, 8);

  if (entries.length === 0) {
    listEl.appendChild(el('li', { className: 'muted', text: emptyLabel }));
    return;
  }

  const maxVal = Math.max(...entries.map(([, count]) => count), 1);

  for (const [key, count] of entries) {
    const pct = Math.max(4, Math.round((count / maxVal) * 100));
    const fill = el('div', { className: 'breakdown-bar-fill' });
    fill.style.width = `${pct}%`;

    listEl.appendChild(
      el(
        'li',
        { className: 'breakdown-item' },
        el(
          'div',
          { className: 'breakdown-row' },
          el('span', { className: 'breakdown-name', text: key, title: key }),
          el('span', { className: 'breakdown-count', text: count.toLocaleString('pt-BR') })
        ),
        el('div', { className: 'breakdown-bar-track', attrs: { 'aria-hidden': 'true' } }, fill)
      )
    );
  }
}

function getChartThemeTokens() {
  const dark = isDarkTheme();
  return {
    accent: dark ? '#2fbf8c' : '#0f5c47',
    text: dark ? '#f2f1ed' : '#181715',
    muted: dark ? '#9e9b93' : '#5c5852',
    grid: dark ? '#2e2d29' : '#e2dfd7',
    surface: dark ? '#191917' : '#ffffff',
    palette: dark
      ? ['#2fbf8c', '#e68a4d', '#5c9ce6', '#b28ce2', '#8a867e']
      : ['#0f5c47', '#9a3f0c', '#1d4ed8', '#6d28d9', '#78716c'],
  };
}

function renderCharts(stats, days) {
  if (typeof window.Chart === 'undefined') return;
  destroyCharts();

  const theme = getChartThemeTokens();
  const { iso, display } = buildDateRange(days);
  const dateCounts = iso.map((day) => (stats.clicks_by_date && stats.clicks_by_date[day]) || 0);
  const totalInRange = dateCounts.reduce((acc, n) => acc + n, 0);

  timelineCanvas.setAttribute(
    'aria-label',
    `Gráfico de cliques nos últimos ${days} dias: total de ${totalInRange} cliques.`
  );

  timelineChart = new window.Chart(timelineCanvas.getContext('2d'), {
    type: 'line',
    data: {
      labels: display,
      datasets: [
        {
          label: 'Cliques',
          data: dateCounts,
          borderColor: theme.accent,
          backgroundColor: theme.accent,
          borderWidth: 2,
          pointRadius: 3,
          pointHoverRadius: 5,
          pointBackgroundColor: theme.accent,
          tension: 0.15,
          fill: false,
        },
      ],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      plugins: {
        legend: { display: false },
        tooltip: {
          backgroundColor: theme.surface,
          titleColor: theme.text,
          bodyColor: theme.text,
          borderColor: theme.grid,
          borderWidth: 1,
        },
      },
      scales: {
        x: {
          grid: { display: false },
          ticks: { color: theme.muted, maxRotation: 0, autoSkipPadding: 14 },
        },
        y: {
          beginAtZero: true,
          grid: { color: theme.grid },
          ticks: { color: theme.muted, precision: 0 },
        },
      },
    },
  });

  const deviceEntries = Object.entries(stats.clicks_by_device || {}).filter(([, v]) => v > 0);
  if (deviceEntries.length === 0) {
    devicesCanvasWrap.hidden = true;
    devicesEmpty.hidden = false;
    return;
  }

  devicesCanvasWrap.hidden = false;
  devicesEmpty.hidden = true;

  const deviceLabels = deviceEntries.map(([k]) => k);
  const deviceValues = deviceEntries.map(([, v]) => v);
  const deviceSummary = deviceEntries.map(([k, v]) => `${k}: ${v}`).join(', ');
  devicesCanvas.setAttribute('aria-label', `Distribuição por dispositivo: ${deviceSummary}.`);

  devicesChart = new window.Chart(devicesCanvas.getContext('2d'), {
    type: 'doughnut',
    data: {
      labels: deviceLabels,
      datasets: [
        {
          data: deviceValues,
          backgroundColor: theme.palette,
          borderColor: theme.surface,
          borderWidth: 2,
        },
      ],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      cutout: '62%',
      plugins: {
        legend: {
          position: 'bottom',
          labels: {
            color: theme.text,
            boxWidth: 12,
            padding: 14,
          },
        },
        tooltip: {
          backgroundColor: theme.surface,
          titleColor: theme.text,
          bodyColor: theme.text,
          borderColor: theme.grid,
          borderWidth: 1,
        },
      },
    },
  });
}

document.addEventListener('esmaga:themechange', () => {
  if (lastStatsPayload && !statsSection.hidden) {
    renderCharts(lastStatsPayload, lastStatsDays);
  }
});

async function loadStats(code) {
  const prevCode = selectedStatsCode;
  selectedStatsCode = code;
  if (prevCode !== code && cachedLinks.length > 0) {
    renderLinksTable(cachedLinks);
  }

  const days = Number(statsDaysSelect.value) || 7;
  statsSection.hidden = false;
  statsCodeLabel.textContent = code;
  setBoxError(statsError, '');

  try {
    const stats = await getLinkStats(code, days);
    lastStatsPayload = stats;
    lastStatsDays = days;

    const total = Number(stats.total_clicks ?? 0);
    statsTotalClicks.textContent = total.toLocaleString('pt-BR');
    statsAvgClicks.textContent = (total / days).toLocaleString('pt-BR', {
      maximumFractionDigits: 1,
    });

    const topDeviceEntry = Object.entries(stats.clicks_by_device || {}).sort((a, b) => b[1] - a[1])[0];
    statsTopDevice.textContent = topDeviceEntry && topDeviceEntry[1] > 0 ? topDeviceEntry[0] : '—';

    renderCharts(stats, days);
    renderBreakdownList(referrersList, stats.top_referrers, 'Nenhuma origem registrada neste período');
    renderBreakdownList(countriesList, stats.clicks_by_country, 'Nenhum país registrado neste período');
  } catch (err) {
    setBoxError(statsError, err.message || 'Erro ao carregar estatísticas.');
  }
}

createForm.addEventListener('submit', async (event) => {
  event.preventDefault();
  setBoxError(createError, '', createUrlInput);

  const rawUrl = createUrlInput.value.trim();
  if (!isSafeHttpUrl(rawUrl)) {
    setBoxError(createError, 'Informe uma URL válida iniciando com http:// ou https://', createUrlInput);
    createUrlInput.focus();
    return;
  }

  createBtn.disabled = true;
  createBtn.textContent = 'Criando...';
  createForm.setAttribute('aria-busy', 'true');
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
    createBtn.textContent = 'Criar link';
    createForm.removeAttribute('aria-busy');
  }
});

createUrlInput.addEventListener('input', () => {
  if (!createError.hidden) {
    setBoxError(createError, '', createUrlInput);
  }
});

refreshLinksBtn.addEventListener('click', loadLinks);

statsDaysSelect.addEventListener('change', () => {
  if (selectedStatsCode) {
    loadStats(selectedStatsCode);
  }
});

editCancelBtn.addEventListener('click', () => {
  modalCtrl.close(editModal);
});

editForm.addEventListener('submit', async (event) => {
  event.preventDefault();
  setBoxError(editError, '', editUrlInput);

  const nextUrl = editUrlInput.value.trim();
  if (!isSafeHttpUrl(nextUrl)) {
    setBoxError(editError, 'Informe uma URL válida iniciando com http:// ou https://', editUrlInput);
    editUrlInput.focus();
    return;
  }

  editSaveBtn.disabled = true;
  editSaveBtn.textContent = 'Salvando...';
  try {
    await editLink(editingCode, nextUrl);
    modalCtrl.close(editModal);
    await loadLinks();
    showToast(toastEl, 'URL de destino atualizada.');
  } catch (err) {
    setBoxError(editError, err.message || 'Erro ao editar link.');
  } finally {
    editSaveBtn.disabled = false;
    editSaveBtn.textContent = 'Salvar destino';
  }
});

qrCloseBtn.addEventListener('click', () => {
  modalCtrl.close(qrModal);
});

qrDownloadBtn.addEventListener('click', () => {
  if (qrCurrentCode) {
    downloadQRCodePNG(modalQrCanvas, qrCurrentCode);
    showToast(toastEl, 'Download do QR Code iniciado.');
  }
});

deleteLinkCancelBtn.addEventListener('click', () => {
  clearDeleteLinkCountdown();
  modalCtrl.close(deleteLinkModal);
});

deleteLinkConfirmBtn.addEventListener('click', async () => {
  if (!deletingLinkCode) return;
  clearDeleteLinkCountdown();
  setBoxError(deleteLinkError, '');
  deleteLinkConfirmBtn.disabled = true;
  deleteLinkConfirmBtn.textContent = 'Excluindo...';
  try {
    await deleteLink(deletingLinkCode);
    const code = deletingLinkCode;
    deletingLinkCode = null;
    if (selectedStatsCode === code) {
      selectedStatsCode = null;
    }
    modalCtrl.close(deleteLinkModal);
    showToast(toastEl, `Link ${code} excluído permanentemente.`);
    await loadLinks();
  } catch (err) {
    setBoxError(deleteLinkError, err.message || 'Erro ao excluir link.');
    deleteLinkConfirmBtn.disabled = false;
  } finally {
    deleteLinkConfirmBtn.textContent = 'Excluir permanentemente';
  }
});

accountOpenBtn.addEventListener('click', async () => {
  setBoxError(accountError, '');
  accountCurrentPass.value = '';
  accountNewPass.value = '';
  accountNewPassConfirm.value = '';
  modalCtrl.open(accountModal, { initialFocus: accountNameInput });
  try {
    const me = await getMe();
    const statusLabel = me.status === 'active' ? 'Ativa' : me.status;
    accountEmailLabel.textContent = `${me.email} • ${statusLabel}`;
    accountNameInput.value = me.name || '';
  } catch (err) {
    setBoxError(accountError, err.message || 'Erro ao carregar perfil.');
  }
});

accountCancelBtn.addEventListener('click', () => {
  modalCtrl.close(accountModal);
});

accountForm.addEventListener('submit', async (event) => {
  event.preventDefault();
  setBoxError(accountError, '', accountNewPassConfirm);
  accountSaveBtn.disabled = true;

  const payload = { name: accountNameInput.value.trim() };
  if (accountNewPass.value) {
    if (accountNewPass.value !== accountNewPassConfirm.value) {
      setBoxError(accountError, 'A nova senha e a confirmação não coincidem.', accountNewPassConfirm);
      accountNewPassConfirm.focus();
      accountSaveBtn.disabled = false;
      return;
    }
    payload.current_password = accountCurrentPass.value;
    payload.new_password = accountNewPass.value;
  }

  try {
    await updateMe(payload);
    modalCtrl.close(accountModal);
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
  modalCtrl.open(deleteModal, { initialFocus: deleteCancelBtn, preserveTrigger: true });
});

deleteCancelBtn.addEventListener('click', () => {
  modalCtrl.close(deleteModal);
});

deleteConfirmBtn.addEventListener('click', async () => {
  setBoxError(deleteError, '');
  deleteConfirmBtn.disabled = true;
  deleteConfirmBtn.textContent = 'Excluindo...';
  try {
    await deleteAccount(keepLinksCheckbox.checked);
    modalCtrl.close(deleteModal, { restoreFocus: false });
    syncView();
    showToast(toastEl, 'Conta excluída com sucesso.');
  } catch (err) {
    setBoxError(deleteError, err.message || 'Erro ao excluir conta.');
  } finally {
    deleteConfirmBtn.disabled = false;
    deleteConfirmBtn.textContent = 'Excluir minha conta';
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
