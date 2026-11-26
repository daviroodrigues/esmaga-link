export async function renderQRCode(canvas, url, size = 180) {
  if (!canvas || typeof window.QRCode === 'undefined') {
    return;
  }

  await window.QRCode.toCanvas(canvas, url, {
    width: size,
    margin: 2,
    color: {
      dark: '#1c1917',
      light: '#ffffff',
    },
    errorCorrectionLevel: 'M',
  });
}

export function downloadQRCodePNG(canvas, shortCode) {
  if (!canvas) return;
  const safeName = String(shortCode || 'link').replace(/[^a-zA-Z0-9_-]/g, '');
  const dataUrl = canvas.toDataURL('image/png');
  const a = document.createElement('a');
  a.href = dataUrl;
  a.download = `esmaga-${safeName}.png`;
  document.body.appendChild(a);
  a.click();
  a.remove();
}
