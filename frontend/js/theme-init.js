(() => {
  try {
    const saved = localStorage.getItem('esmaga_theme');
    if (saved === 'dark' || saved === 'light') {
      document.documentElement.setAttribute('data-theme', saved);
    }
  } catch {
  }
})();
