/**
 * Wrapper para geração de QR Code em <canvas> e exportação PNG com margem.
 * Requer window.QRCode carregado via CDN (qrcode@1.5.1).
 */

export async function renderQRCode(canvas, url, size = 180) {
  if (!canvas || typeof window.QRCode === 'undefined') {
    return;
  }

  await window.QRCode.toCanvas(canvas, url, {
    width: size,
    margin: 2,
    color: {
      dark: '#0f172a',
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
