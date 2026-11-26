import { initThemeToggle } from './security.js';

initThemeToggle(document.getElementById('theme-toggle'));

const retryBtn = document.getElementById('retry-btn');
if (retryBtn) {
  retryBtn.addEventListener('click', () => {
    window.location.reload();
  });
}
