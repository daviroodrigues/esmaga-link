import { resendVerification, verifyEmail } from './api.js';
import { initThemeToggle } from './security.js';

const themeBtn = document.getElementById('theme-toggle');
const statusEl = document.getElementById('verify-status');
const successEl = document.getElementById('verify-success');
const errorEl = document.getElementById('verify-error');
const actionsEl = document.getElementById('verify-actions');
const resendForm = document.getElementById('resend-form');
const resendEmailInput = document.getElementById('resend-email');
const resendBtn = document.getElementById('resend-btn');

initThemeToggle(themeBtn);

const params = new URLSearchParams(window.location.search);
const token = params.get('token') || '';

async function runVerification() {
  if (!token) {
    statusEl.textContent = 'Link de confirmação inválido ou incompleto.';
    resendForm.hidden = false;
    return;
  }

  try {
    await verifyEmail(token);
    statusEl.hidden = true;
    successEl.textContent = 'E-mail confirmado com sucesso! Redirecionando para o painel...';
    successEl.hidden = false;
    actionsEl.hidden = false;
    setTimeout(() => {
      window.location.href = '/dashboard';
    }, 2000);
  } catch (err) {
    statusEl.textContent = 'Não foi possível validar seu e-mail.';
    errorEl.textContent = err.message || 'Link inválido ou expirado.';
    errorEl.hidden = false;
    resendForm.hidden = false;
  }
}

function startCooldown(seconds) {
  let remaining = seconds;
  resendBtn.disabled = true;
  const timer = setInterval(() => {
    remaining -= 1;
    if (remaining <= 0) {
      clearInterval(timer);
      resendBtn.disabled = false;
      resendBtn.textContent = 'Reenviar verificação';
      return;
    }
    resendBtn.textContent = `Aguarde ${remaining}s`;
  }, 1000);
}

resendForm.addEventListener('submit', async (e) => {
  e.preventDefault();
  errorEl.hidden = true;
  successEl.hidden = true;
  const email = resendEmailInput.value.trim();
  resendBtn.disabled = true;
  resendForm.setAttribute('aria-busy', 'true');
  try {
    await resendVerification(email);
    successEl.textContent = 'Se o e-mail estiver pendente, enviamos um novo link de confirmação.';
    successEl.hidden = false;
    startCooldown(60);
  } catch (err) {
    errorEl.textContent = err.message || 'Erro ao reenviar confirmação.';
    errorEl.hidden = false;
    resendBtn.disabled = false;
  } finally {
    resendForm.removeAttribute('aria-busy');
  }
});

runVerification();
