import { getPublicConfig } from './api.js';
import {
  applyPortfolioVisibility,
  clearElement,
  el,
  initThemeToggle,
  isDarkTheme,
} from './security.js';

const themeBtn = document.getElementById('theme-toggle');
const disabledSection = document.getElementById('portfolio-disabled');
const contentWrap = document.getElementById('portfolio-content');

initThemeToggle(themeBtn);

const STANDARD_ALPHABET = '0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ';
const DEMO_SEED_STORAGE_KEY = 'esmaga:demo-alphabet-seed';

function generateRandomSeed() {
  if (window.crypto && typeof window.crypto.getRandomValues === 'function') {
    const bytes = new Uint8Array(4);
    window.crypto.getRandomValues(bytes);
    const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
    return `demo-${hex}`;
  }
  const fallback = Math.floor(Math.random() * 0xffffffff)
    .toString(16)
    .padStart(8, '0');
  return `demo-${fallback}`;
}

function loadOrCreateDemoSeed() {
  try {
    const stored = window.localStorage.getItem(DEMO_SEED_STORAGE_KEY);
    if (stored && stored.trim() !== '') {
      return stored;
    }
    const created = generateRandomSeed();
    window.localStorage.setItem(DEMO_SEED_STORAGE_KEY, created);
    return created;
  } catch {
    return generateRandomSeed();
  }
}

function saveDemoSeed(seed) {
  try {
    if (seed) {
      window.localStorage.setItem(DEMO_SEED_STORAGE_KEY, seed);
    } else {
      window.localStorage.removeItem(DEMO_SEED_STORAGE_KEY);
    }
  } catch {
    // Ignore storage restrictions (e.g. private browsing / blocked storage).
  }
}

function hashSeed32(str) {
  let h = 2166136261;
  const bytes = new TextEncoder().encode(str);
  for (let i = 0; i < bytes.length; i++) {
    h ^= bytes[i];
    h = Math.imul(h, 16777619);
  }
  return h >>> 0;
}

function mulberry32(seed) {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

function shuffleAlphabetWithSeed(source, seed) {
  if (!seed) return source;
  const rand = mulberry32(hashSeed32(seed));
  const chars = source.split('');
  for (let i = chars.length - 1; i > 0; i--) {
    const j = Math.floor(rand() * (i + 1));
    [chars[i], chars[j]] = [chars[j], chars[i]];
  }
  return chars.join('');
}

const ERAS = [
  {
    name: 'Era 5',
    codeLength: 5,
    size: 916132832n,
    a: 566165401n,
    c: 271828183n,
  },
  {
    name: 'Era 6',
    codeLength: 6,
    size: 56800235584n,
    a: 3141592701n,
    c: 2718281829n,
  },
  {
    name: 'Era 7',
    codeLength: 7,
    size: 3521614606208n,
    a: 2176510000001n,
    c: 1414213562373n,
  },
];

const MAX_SEQ_ID = ERAS.reduce((acc, era) => acc + era.size, 0n);

function lcgNth(seed, a, c, m, n) {
  let resK = 1n;
  let resB = 0n;
  let curK = a % m;
  let curB = c % m;
  let steps = n;

  while (steps > 0n) {
    if ((steps & 1n) === 1n) {
      resK = (curK * resK) % m;
      resB = ((curK * resB) % m + curB) % m;
    }
    curB = ((curK * curB) % m + curB) % m;
    curK = (curK * curK) % m;
    steps >>= 1n;
  }

  return ((resK * (seed % m)) % m + resB) % m;
}

function encodeBase62(num, alphabet, minPadLen) {
  const base = BigInt(alphabet.length);
  if (num === 0n) {
    return alphabet[0].repeat(Math.max(1, minPadLen));
  }
  let n = num;
  let out = '';
  while (n > 0n) {
    out = alphabet[Number(n % base)] + out;
    n /= base;
  }
  if (out.length < minPadLen) {
    out = alphabet[0].repeat(minPadLen - out.length) + out;
  }
  return out;
}

function decodeBase62(str, alphabet) {
  const base = BigInt(alphabet.length);
  let res = 0n;
  for (let i = 0; i < str.length; i++) {
    const idx = alphabet.indexOf(str[i]);
    if (idx === -1) return null;
    res = res * base + BigInt(idx);
  }
  return res;
}

function gcdBig(a, b) {
  let x = a;
  let y = b;
  while (y !== 0n) {
    const t = y;
    y = x % y;
    x = t;
  }
  return x;
}

function transformSeqID(seqID, permutedAlphabet) {
  let remaining = seqID > 0n ? seqID - 1n : 0n;
  for (const era of ERAS) {
    if (remaining < era.size) {
      const obfuscated = lcgNth(0n, era.a, era.c, era.size, remaining);
      return {
        seqID,
        era,
        localIndex: remaining,
        obfuscated,
        standardCode: encodeBase62(obfuscated, STANDARD_ALPHABET, era.codeLength),
        permutedCode: encodeBase62(obfuscated, permutedAlphabet, era.codeLength),
      };
    }
    remaining -= era.size;
  }
  return null;
}

function formatBigIntBR(val) {
  return val.toString().replace(/\B(?=(\d{3})+(?!\d))/g, '.');
}

function initRingSimulator() {
  const RING_M = 248;
  const RING_A = 125;
  const RING_C = 153;
  const BATCH_SIZE = 10;

  const canvas = document.getElementById('ring-canvas');
  const ctx = canvas.getContext('2d');
  const modeEsmagaBtn = document.getElementById('mode-esmaga');
  const modeSeqBtn = document.getElementById('mode-seq');
  const modeRandBtn = document.getElementById('mode-rand');
  const modeDesc = document.getElementById('ring-mode-desc');
  const stepBtn = document.getElementById('ring-step-btn');
  const playBtn = document.getElementById('ring-play-btn');
  const resetBtn = document.getElementById('ring-reset-btn');
  const statGenerated = document.getElementById('ring-stat-generated');
  const statCollisions = document.getElementById('ring-stat-collisions');
  const statCoverage = document.getElementById('ring-stat-coverage');
  const lastBatchEl = document.getElementById('ring-last-batch');

  const MODE_DESCRIPTIONS = {
    esmaga:
      'Modo Esmaga Link: reserva lotes de 10 IDs, embaralha a ordem dentro do lote e aplica T(x) = (125·x + 153) mod 248 (Hull-Dobell com passo ≈ φ⁻¹). Visita todas as 248 posições com zero colisões.',
    seq:
      'Modo Sequencial (AUTO_INCREMENT): ocupa posições vizinhas em arco contínuo. Zero colisões, mas expõe facilmente o volume de links criados e permite enumeração trivial.',
    rand:
      'Modo Aleatório (Hash truncado / PRNG): escolhe posições ao acaso. Pelo Paradoxo do Aniversário, colisões começam a surgir rapidamente (em torno de √248 ≈ 16 IDs), exigindo verificação no banco.',
  };

  let mode = 'esmaga';
  let seqCursor = 0;
  let generatedCount = 0;
  let collisionCount = 0;
  let visited = new Set();
  let previousPoints = [];
  let currentBatch = [];
  let playTimer = null;

  function lcgSmall(n) {
    let x = 0;
    for (let i = 0; i < n; i++) {
      x = (RING_A * x + RING_C) % RING_M;
    }
    return x;
  }

  function shuffleArray(arr) {
    const copy = arr.slice();
    for (let i = copy.length - 1; i > 0; i--) {
      const j = Math.floor(Math.random() * (i + 1));
      [copy[i], copy[j]] = [copy[j], copy[i]];
    }
    return copy;
  }

  function stopAutoPlay() {
    if (playTimer) {
      clearInterval(playTimer);
      playTimer = null;
    }
    playBtn.textContent = 'Animar';
    playBtn.setAttribute('aria-pressed', 'false');
  }

  function resetState() {
    stopAutoPlay();
    seqCursor = 0;
    generatedCount = 0;
    collisionCount = 0;
    visited = new Set();
    previousPoints = [];
    currentBatch = [];
    stepBtn.disabled = false;
    updateReadout();
    drawRing();
  }

  function stepBatch() {
    if (generatedCount >= RING_M) {
      stopAutoPlay();
      stepBtn.disabled = true;
      return;
    }

    for (const item of currentBatch) {
      previousPoints.push(item);
    }
    currentBatch = [];

    const count = Math.min(BATCH_SIZE, RING_M - generatedCount);
    let batchIndices = [];
    for (let i = 0; i < count; i++) {
      batchIndices.push(seqCursor + i);
    }
    seqCursor += count;

    if (mode === 'esmaga') {
      batchIndices = shuffleArray(batchIndices);
    }

    for (const idx of batchIndices) {
      let pos;
      if (mode === 'esmaga') {
        pos = lcgSmall(idx);
      } else if (mode === 'seq') {
        pos = idx % RING_M;
      } else {
        pos = Math.floor(Math.random() * RING_M);
      }

      const collided = visited.has(pos);
      if (collided) {
        collisionCount += 1;
      } else {
        visited.add(pos);
      }
      generatedCount += 1;
      currentBatch.push({ pos, collided });
    }

    if (generatedCount >= RING_M) {
      stopAutoPlay();
      stepBtn.disabled = true;
    }

    updateReadout();
    drawRing();
  }

  function updateReadout() {
    modeDesc.textContent = MODE_DESCRIPTIONS[mode];
    statGenerated.textContent = `${generatedCount} / ${RING_M}`;
    statCollisions.textContent = String(collisionCount);
    const pct = Math.round((visited.size / RING_M) * 100);
    statCoverage.textContent = `${pct}%`;

    canvas.setAttribute(
      'aria-label',
      `Anel finito no modo ${mode}: ${generatedCount} IDs gerados, ${visited.size} posições únicas ocupadas (${pct}%), ${collisionCount} colisões.`
    );

    clearElement(lastBatchEl);
    if (currentBatch.length === 0) {
      lastBatchEl.appendChild(
        el('span', { className: 'muted', text: 'Clique em "Gerar lote" para observar a dispersão.' })
      );
      return;
    }

    for (const item of currentBatch) {
      lastBatchEl.appendChild(
        el('span', {
          className: item.collided ? 'code-chip code-chip-collision' : 'code-chip',
          text: item.collided ? `#${item.pos} (colisão)` : `#${item.pos}`,
        })
      );
    }
  }

  function drawRing() {
    const dpr = window.devicePixelRatio || 1;
    const size = 320;
    if (canvas.width !== size * dpr) {
      canvas.width = size * dpr;
      canvas.height = size * dpr;
    }
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, size, size);

    const dark = isDarkTheme();
    const cx = size / 2;
    const cy = size / 2;
    const radius = 128;

    const ringStroke = dark ? '#383630' : '#dcd8ce';
    const emerald = dark ? '#2fbf8c' : '#0f5c47';
    const warm = dark ? '#e68a4d' : '#9a3f0c';
    const warmTrail = dark ? '#4a3a2d' : '#e3cec0';
    const danger = dark ? '#f28b82' : '#b42318';
    const textColor = dark ? '#f2f1ed' : '#181715';
    const mutedColor = dark ? '#9e9b93' : '#5c5852';

    ctx.beginPath();
    ctx.arc(cx, cy, radius, 0, Math.PI * 2);
    ctx.strokeStyle = ringStroke;
    ctx.lineWidth = 2;
    ctx.stroke();

    for (let i = 0; i < 12; i++) {
      const ang = (i / 12) * Math.PI * 2 - Math.PI / 2;
      ctx.beginPath();
      ctx.moveTo(cx + Math.cos(ang) * (radius - 6), cy + Math.sin(ang) * (radius - 6));
      ctx.lineTo(cx + Math.cos(ang) * (radius + 6), cy + Math.sin(ang) * (radius + 6));
      ctx.strokeStyle = ringStroke;
      ctx.lineWidth = 1;
      ctx.stroke();
    }

    const posToXY = (pos) => {
      const angle = (pos / RING_M) * Math.PI * 2 - Math.PI / 2;
      return {
        x: cx + Math.cos(angle) * radius,
        y: cy + Math.sin(angle) * radius,
      };
    };

    if (currentBatch.length > 1) {
      ctx.beginPath();
      currentBatch.forEach((item, idx) => {
        const pt = posToXY(item.pos);
        if (idx === 0) ctx.moveTo(pt.x, pt.y);
        else ctx.lineTo(pt.x, pt.y);
      });
      ctx.strokeStyle = warmTrail;
      ctx.lineWidth = 1.4;
      ctx.stroke();
    }

    for (const item of previousPoints) {
      const pt = posToXY(item.pos);
      ctx.beginPath();
      ctx.arc(pt.x, pt.y, item.collided ? 4.2 : 3.2, 0, Math.PI * 2);
      ctx.fillStyle = item.collided ? danger : emerald;
      ctx.fill();
    }

    for (const item of currentBatch) {
      const pt = posToXY(item.pos);
      ctx.beginPath();
      ctx.arc(pt.x, pt.y, item.collided ? 6 : 5, 0, Math.PI * 2);
      ctx.fillStyle = item.collided ? danger : warm;
      ctx.fill();
    }

    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillStyle = textColor;
    ctx.font = '700 22px system-ui, -apple-system, sans-serif';
    ctx.fillText(`ℤ_${RING_M}`, cx, cy - 10);
    ctx.fillStyle = mutedColor;
    ctx.font = '500 12px system-ui, -apple-system, sans-serif';
    const sub =
      mode === 'esmaga'
        ? 'Hull-Dobell + φ⁻¹'
        : mode === 'seq'
          ? 'Sequencial'
          : 'Aleatório (PRNG)';
    ctx.fillText(sub, cx, cy + 12);
  }

  function setMode(nextMode) {
    mode = nextMode;
    modeEsmagaBtn.setAttribute('aria-pressed', String(mode === 'esmaga'));
    modeSeqBtn.setAttribute('aria-pressed', String(mode === 'seq'));
    modeRandBtn.setAttribute('aria-pressed', String(mode === 'rand'));
    resetState();
    stepBatch();
  }

  modeEsmagaBtn.addEventListener('click', () => setMode('esmaga'));
  modeSeqBtn.addEventListener('click', () => setMode('seq'));
  modeRandBtn.addEventListener('click', () => setMode('rand'));

  stepBtn.addEventListener('click', stepBatch);
  resetBtn.addEventListener('click', resetState);

  playBtn.addEventListener('click', () => {
    if (playTimer) {
      stopAutoPlay();
      return;
    }
    if (generatedCount >= RING_M) {
      resetState();
    }
    playBtn.textContent = 'Pausar';
    playBtn.setAttribute('aria-pressed', 'true');
    stepBatch();
    playTimer = setInterval(stepBatch, 650);
  });

  document.addEventListener('esmaga:themechange', drawRing);

  setMode('esmaga');
}

function initPipelineLab() {
  const form = document.getElementById('lab-form');
  const seqInput = document.getElementById('lab-seq-input');
  const randomBtn = document.getElementById('lab-random-btn');
  const shuffleToggle = document.getElementById('lab-shuffle-toggle');
  const seedInput = document.getElementById('lab-seed-input');
  const seedRandomBtn = document.getElementById('lab-seed-random-btn');
  const alphabetDisplay = document.getElementById('lab-alphabet-display');
  const errorBox = document.getElementById('lab-error');

  let currentSeed = loadOrCreateDemoSeed();
  let permutedAlphabet = shuffleAlphabetWithSeed(STANDARD_ALPHABET, currentSeed);
  seedInput.value = currentSeed;

  const eraVal = document.getElementById('lab-era-val');
  const eraMeta = document.getElementById('lab-era-meta');
  const obfVal = document.getElementById('lab-obf-val');
  const obfMeta = document.getElementById('lab-obf-meta');
  const proofBadges = document.getElementById('lab-proof-badges');
  const codeVal = document.getElementById('lab-code-val');
  const codeMeta = document.getElementById('lab-code-meta');
  const revVal = document.getElementById('lab-rev-val');
  const revMeta = document.getElementById('lab-rev-meta');
  const neighborsTbody = document.getElementById('lab-neighbors-tbody');

  function setLabError(msg) {
    if (!msg) {
      errorBox.hidden = true;
      errorBox.textContent = '';
      seqInput.removeAttribute('aria-invalid');
      return;
    }
    errorBox.textContent = msg;
    errorBox.hidden = false;
    seqInput.setAttribute('aria-invalid', 'true');
  }

  function parseSeqInput(raw) {
    const clean = String(raw || '').replace(/[\s._,]/g, '');
    if (!/^\d+$/.test(clean)) return null;
    try {
      const val = BigInt(clean);
      if (val < 1n || val > MAX_SEQ_ID) return null;
      return val;
    } catch {
      return null;
    }
  }

  function renderAlphabetDisplay(activeAlphabet) {
    clearElement(alphabetDisplay);
    alphabetDisplay.appendChild(
      el('span', {
        className: 'alphabet-zero',
        text: activeAlphabet[0],
        attrs: { title: 'Índice 0 do alfabeto (usado no padding à esquerda)' },
      })
    );
    alphabetDisplay.appendChild(document.createTextNode(activeAlphabet.slice(1)));
  }

  function renderPipeline(seqID) {
    const result = transformSeqID(seqID, permutedAlphabet);
    if (!result) {
      setLabError(`Informe um número inteiro entre 1 e ${formatBigIntBR(MAX_SEQ_ID)}.`);
      return;
    }
    setLabError('');

    const usePermuted = shuffleToggle.checked;
    const activeAlphabet = usePermuted ? permutedAlphabet : STANDARD_ALPHABET;
    const activeCode = usePermuted ? result.permutedCode : result.standardCode;
    const otherCode = usePermuted ? result.standardCode : result.permutedCode;

    seedInput.disabled = !usePermuted;
    seedRandomBtn.disabled = !usePermuted;
    renderAlphabetDisplay(activeAlphabet);

    eraVal.textContent = `${result.era.name} (${result.era.codeLength} caracteres)`;
    eraMeta.textContent = `Índice local n = ${formatBigIntBR(result.localIndex)} no universo M = 62^${result.era.codeLength} = ${formatBigIntBR(result.era.size)}.`;

    obfVal.textContent = formatBigIntBR(result.obfuscated);
    obfMeta.textContent = `T^n(0) mod M com A = ${formatBigIntBR(result.era.a)} e C = ${formatBigIntBR(result.era.c)}.`;

    clearElement(proofBadges);
    const gcdOk = gcdBig(result.era.c, result.era.size) === 1n;
    const primesOk = (result.era.a - 1n) % 62n === 0n;
    const fourOk = (result.era.a - 1n) % 4n === 0n;
    if (gcdOk) proofBadges.appendChild(el('span', { className: 'proof-badge', text: 'gcd(C, M) = 1' }));
    if (primesOk) proofBadges.appendChild(el('span', { className: 'proof-badge', text: '2, 31 | (A-1)' }));
    if (fourOk) proofBadges.appendChild(el('span', { className: 'proof-badge', text: '4 | (A-1)' }));

    const zeroNote =
      result.obfuscated === 0n
        ? ` Como n = 0 gera T^0(0) = 0, o código é preenchido com o dígito 0 deste alfabeto ("${activeAlphabet[0]}").`
        : '';
    codeVal.textContent = `/r/${activeCode}`;
    codeMeta.textContent = usePermuted
      ? `Alfabeto gerado a partir da seed demonstrativa "${currentSeed || '(vazia — padrão)'}" (não usa a seed de produção). No Base62 ordenado seria "${otherCode}".${zeroNote}`
      : `Alfabeto Base62 padrão (0-9a-zA-Z). Com a seed demonstrativa atual fica "${otherCode}".${zeroNote}`;

    const decoded = decodeBase62(activeCode, activeAlphabet);
    const exactMatch = decoded === result.obfuscated;
    revVal.textContent = exactMatch ? formatBigIntBR(decoded) : 'Erro';
    revMeta.textContent = exactMatch
      ? `Decode("${activeCode}") === ${formatBigIntBR(result.obfuscated)} em memória, sem consulta ao banco.`
      : 'Falha na validação de decodificação.';

    clearElement(neighborsTbody);
    const candidates = [seqID - 1n, seqID, seqID + 1n].filter((id) => id >= 1n && id <= MAX_SEQ_ID);
    for (const id of candidates) {
      const r = transformSeqID(id, permutedAlphabet);
      const isCurrent = id === seqID;
      const tr = el(
        'tr',
        { className: isCurrent ? 'row-selected' : '' },
        el('td', { className: 'mono tabular-nums', text: formatBigIntBR(id) }),
        el('td', { text: r.era.name }),
        el('td', { className: 'mono tabular-nums', text: formatBigIntBR(r.obfuscated) }),
        el('td', { className: 'mono', text: r.standardCode }),
        el('td', { className: 'mono', text: r.permutedCode })
      );
      neighborsTbody.appendChild(tr);
    }
  }

  function applySeed(nextSeed) {
    currentSeed = nextSeed;
    saveDemoSeed(currentSeed);
    permutedAlphabet = shuffleAlphabetWithSeed(STANDARD_ALPHABET, currentSeed);
    const parsed = parseSeqInput(seqInput.value);
    if (parsed !== null) {
      renderPipeline(parsed);
    } else {
      renderAlphabetDisplay(shuffleToggle.checked ? permutedAlphabet : STANDARD_ALPHABET);
    }
  }

  form.addEventListener('submit', (event) => {
    event.preventDefault();
    const parsed = parseSeqInput(seqInput.value);
    if (parsed === null) {
      setLabError(`Informe um número inteiro entre 1 e ${formatBigIntBR(MAX_SEQ_ID)}.`);
      seqInput.focus();
      return;
    }
    renderPipeline(parsed);
  });

  shuffleToggle.addEventListener('change', () => {
    const parsed = parseSeqInput(seqInput.value);
    if (parsed !== null) {
      renderPipeline(parsed);
    } else {
      const usePermuted = shuffleToggle.checked;
      seedInput.disabled = !usePermuted;
      seedRandomBtn.disabled = !usePermuted;
      renderAlphabetDisplay(usePermuted ? permutedAlphabet : STANDARD_ALPHABET);
    }
  });

  seedInput.addEventListener('input', () => {
    applySeed(seedInput.value);
  });

  seedRandomBtn.addEventListener('click', () => {
    const nextSeed = generateRandomSeed();
    seedInput.value = nextSeed;
    applySeed(nextSeed);
  });

  randomBtn.addEventListener('click', () => {
    const randVal = BigInt(Math.floor(Math.random() * 900_000_000) + 1);
    seqInput.value = randVal.toString();
    renderPipeline(randVal);
  });

  const presetBtns = document.querySelectorAll('[data-preset]');
  for (const btn of presetBtns) {
    btn.addEventListener('click', () => {
      const val = btn.getAttribute('data-preset');
      seqInput.value = val;
      const parsed = parseSeqInput(val);
      if (parsed !== null) renderPipeline(parsed);
    });
  }

  renderPipeline(42n);
}

(async () => {
  const cfg = await getPublicConfig();
  const enabled = applyPortfolioVisibility(cfg);
  if (!enabled) {
    disabledSection.hidden = false;
    contentWrap.hidden = true;
    return;
  }

  disabledSection.hidden = true;
  contentWrap.hidden = false;
  initRingSimulator();
  initPipelineLab();
})();
