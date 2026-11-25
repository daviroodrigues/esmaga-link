import { resetPassword } from './api.js';
import { initPasswordToggles, initThemeToggle } from './security.js';

const themeBtn = document.getElementById('theme-toggle');
const statusEl = document.getElementById('reset-status');
const successEl = document.getElementById('reset-success');
const errorEl = document.getElementById('reset-error');
const form = document.getElementById('reset-form');
const passInput = document.getElementById('reset-password');
const confirmInput = document.getElementById('reset-password-confirm');
const submitBtn = document.getElementById('reset-btn');

initThemeToggle(themeBtn);
initPasswordToggles();

const params = new URLSearchParams(window.location.search);
const token = params.get('token') || '';

if (!token) {
  errorEl.textContent = 'Token de redefinição ausente na URL.';
  errorEl.hidden = false;
  form.hidden = true;
}

form.addEventListener('submit', async (e) => {
  e.preventDefault();
  errorEl.hidden = true;

  const newPass = passInput.value;
  if (newPass !== confirmInput.value) {
    errorEl.textContent = 'As senhas não coincidem.';
    errorEl.hidden = false;
    return;
  }

  submitBtn.disabled = true;
  try {
    await resetPassword(token, newPass);
    form.hidden = true;
    statusEl.hidden = true;
    successEl.textContent = 'Senha redefinida com sucesso! Redirecionando para o login...';
    successEl.hidden = false;
    setTimeout(() => {
      window.location.href = '/dashboard';
    }, 2000);
  } catch (err) {
    errorEl.textContent = err.message || 'Erro ao redefinir senha.';
    errorEl.hidden = false;
    submitBtn.disabled = false;
  }
});
