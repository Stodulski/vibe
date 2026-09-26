#!/usr/bin/env node
/**
 * Prints WCAG 2.1 contrast ratios before and after odd/tasks/app-dark-contrast.md,
 * for every layer pair, text tier and control edge the task touches. No
 * network, no browser — plain sRGB math against the literal token values in
 * `src/styles/globals.css` (duplicated here on purpose: this is a report on
 * what changed, not a CSS parser, and the "before" values no longer exist in
 * the file to read back).
 *
 * Usage: node scripts/contrast-report.mjs
 */

// ─── Color math ──────────────────────────────────────────────────────────

/** Parses `#rgb`, `#rrggbb` or `rgba(r,g,b,a)` into { r, g, b, a } (0-255, a 0-1). */
function parseColor(input) {
  const s = input.trim();
  const hex = /^#?([0-9a-f]{3}|[0-9a-f]{6})$/i.exec(s);
  if (hex?.[1]) {
    const h = hex[1];
    const full =
      h.length === 3
        ? h
            .split('')
            .map((c) => c + c)
            .join('')
        : h;
    const r = parseInt(full.slice(0, 2), 16);
    const g = parseInt(full.slice(2, 4), 16);
    const b = parseInt(full.slice(4, 6), 16);
    return { r, g, b, a: 1 };
  }
  const rgba = /^rgba?\(\s*([\d.]+)\s*,\s*([\d.]+)\s*,\s*([\d.]+)\s*(?:,\s*([\d.]+)\s*)?\)$/i.exec(s);
  if (rgba?.[1] && rgba[2] && rgba[3]) {
    return {
      r: Number(rgba[1]),
      g: Number(rgba[2]),
      b: Number(rgba[3]),
      a: rgba[4] === undefined ? 1 : Number(rgba[4]),
    };
  }
  throw new Error(`Unrecognized color: ${input}`);
}

/** Composites `fg` (with alpha) over an opaque `bg`, both as color strings. Returns an opaque {r,g,b}. */
function compositeOver(fg, bg) {
  const f = parseColor(fg);
  const b = parseColor(bg);
  const a = f.a;
  return {
    r: f.r * a + b.r * (1 - a),
    g: f.g * a + b.g * (1 - a),
    b: f.b * a + b.b * (1 - a),
  };
}

function channelToLinear(c) {
  const cs = c / 255;
  return cs <= 0.03928 ? cs / 12.92 : ((cs + 0.055) / 1.055) ** 2.4;
}

/** WCAG relative luminance of an opaque {r,g,b} (0-255). */
function relativeLuminance({ r, g, b }) {
  return 0.2126 * channelToLinear(r) + 0.7152 * channelToLinear(g) + 0.0722 * channelToLinear(b);
}

/** WCAG contrast ratio between two opaque {r,g,b} colors. */
function contrastRatio(c1, c2) {
  const l1 = relativeLuminance(c1);
  const l2 = relativeLuminance(c2);
  const lighter = Math.max(l1, l2);
  const darker = Math.min(l1, l2);
  return (lighter + 0.05) / (darker + 0.05);
}

/** Contrast of `colorStr` (opaque or with alpha) once composited over `bgStr` (must be opaque). */
function ratio(colorStr, bgStr) {
  const fgOpaque = compositeOver(colorStr, bgStr);
  const bgOpaque = parseColor(bgStr);
  return contrastRatio(fgOpaque, bgOpaque);
}

function fmt(r) {
  return `${r.toFixed(2)}:1`;
}

function verdict(r, threshold) {
  return r >= threshold ? 'PASS' : 'FAIL';
}

function line(label, before, after, threshold) {
  const b = ratio(before.fg, before.bg);
  const a = ratio(after.fg, after.bg);
  const thresholdLabel = threshold ? ` (needs ${fmt(threshold)}, ${verdict(a, threshold)})` : '';
  console.log(`  ${label.padEnd(46)} ${fmt(b).padStart(8)}  ->  ${fmt(a).padStart(8)}${thresholdLabel}`);
}

// ─── Tokens ──────────────────────────────────────────────────────────────

const BEFORE = {
  page: '#0b0b0b',
  card: '#111111',
  elevated: '#181818',
  overlay: '#1e1e1e', // old bg-overlay ("elevated card hover"), not a real modal tier
  highlight: '#252525',
  shadcnBackground: '#030303', // oklch(0.10 0 0), measured — see odd/tasks/app-dark-contrast.md
  shadcnPopover: '#070707', // oklch(0.13 0 0), measured
  borderSubtle: 'rgba(255,255,255,0.06)',
  borderDefault: 'rgba(255,255,255,0.10)',
  borderStrong: 'rgba(255,255,255,0.15)',
  borderInteractive: 'rgba(255,255,255,0.35)',
  backdrop: 'rgba(0,0,0,0.5)',
  inputFill: 'rgba(11,11,11,0.6)', // bg-bg-base/60 composited over its own page value
  successBg: '#052e16',
  warningBg: '#1a1700',
  errorBg: '#1c0a0a',
  infoBg: '#0c1929',
  heat0: 'rgba(255,255,255,0.02)',
  heat1: 'rgba(20,184,166,0.10)',
  controlIdleFill: '#111111', // bg-bg-subtle (old value)
  controlIdleBorder: 'rgba(255,255,255,0.06)', // border-border-subtle (old value)
  controlSelectedFill: 'rgba(29,185,84,0.10)', // bg-primary-500/10
  cerradoOpacity: 0.3, // opacity-30 on the whole button, applied on top of text-error-text
};

const AFTER = {
  page: '#0b0b0b',
  card: '#171717',
  elevated: '#1e1e1e',
  overlay: '#212121', // modal/popover tier — tuned 3 steps darker than the requested #242525 (see globals.css)
  highlight: '#2d2d2d', // hover tier — brightest, on purpose (see globals.css)
  background: '#0b0b0b', // shadcn --background, now the exact page value
  popover: '#212121', // shadcn --popover, now the exact modal-tier value
  borderSubtle: 'rgba(255,255,255,0.08)',
  borderDefault: 'rgba(255,255,255,0.14)',
  borderStrong: 'rgba(255,255,255,0.22)',
  borderInteractive: 'rgba(255,255,255,0.35)',
  backdrop: 'rgba(0,0,0,0.7)',
  inputFill: '#0b0b0b', // opaque bg-bg-base
  sidebarFill: '#111111',
  successBg: '#0a5c2c',
  warningBg: '#342e00',
  errorBg: '#381414',
  infoBg: '#183252',
  heat0: 'rgba(255,255,255,0.03)',
  heat1: 'rgba(20,184,166,0.22)',
  controlIdleFill: '#171717', // bg-bg-subtle (new value)
  controlIdleBorder: 'rgba(255,255,255,0.35)', // border-border-interactive
  controlSelectedFill: 'rgba(29,185,84,0.20)', // bg-primary-500/20
  chartAccent: '#14b8a6',
};

const TEXT = {
  primary: '#ffffff',
  secondary: '#a0a0a0',
  tertiary: '#8a8a8a',
  disabled: '#525252',
};

const STATUS_TEXT = {
  success: '#4ade80',
  warning: '#facc15',
  error: '#f87171',
  info: '#7dd3fc',
};

// ─── Report ──────────────────────────────────────────────────────────────

console.log('app-dark-contrast — WCAG 2.1 contrast report (before -> after)\n');

console.log('Elevation ladder vs the page (#0b0b0b):');
line('card vs page', { fg: BEFORE.card, bg: BEFORE.page }, { fg: AFTER.card, bg: AFTER.page });
line('elevated vs page', { fg: BEFORE.elevated, bg: BEFORE.page }, { fg: AFTER.elevated, bg: AFTER.page });
line('overlay/modal vs page', { fg: BEFORE.overlay, bg: BEFORE.page }, { fg: AFTER.overlay, bg: AFTER.page });
line('highlight/hover vs page', { fg: BEFORE.highlight, bg: BEFORE.page }, { fg: AFTER.highlight, bg: AFTER.page });
line('sidebar fill vs page', { fg: BEFORE.page, bg: BEFORE.page }, { fg: AFTER.sidebarFill, bg: AFTER.page });

console.log('\nFloating layers (dialog/sheet/alert-dialog):');
line(
  'content bg vs page (was --background)',
  { fg: BEFORE.shadcnBackground, bg: BEFORE.page },
  { fg: AFTER.popover, bg: AFTER.page },
);
line('backdrop (bg-black/N) vs page', { fg: BEFORE.backdrop, bg: BEFORE.page }, { fg: AFTER.backdrop, bg: AFTER.page });
{
  // The backdrop itself is translucent black over whatever was on the page
  // before the dialog opened — composite it over the page first to get the
  // opaque color a viewer actually sees behind the dialog, then compare the
  // dialog's own content background against that.
  const beforeBackdropOpaque = compositeOver(BEFORE.backdrop, BEFORE.page);
  const afterBackdropOpaque = compositeOver(AFTER.backdrop, AFTER.page);
  const beforeContentRatio = contrastRatio(parseColor(BEFORE.shadcnBackground), beforeBackdropOpaque);
  const afterContentRatio = contrastRatio(parseColor(AFTER.popover), afterBackdropOpaque);
  console.log(
    `  ${'content bg vs its own backdrop'.padEnd(46)} ${fmt(beforeContentRatio).padStart(8)}  ->  ${fmt(afterContentRatio).padStart(8)}`,
  );
}
line(
  'shadcn --popover vs page (select/dropdown)',
  { fg: BEFORE.shadcnPopover, bg: BEFORE.page },
  { fg: AFTER.popover, bg: AFTER.page },
);

console.log('\nInputs & selects (fill vs the surfaces they sit on):');
line('fill vs card', { fg: BEFORE.inputFill, bg: BEFORE.card }, { fg: AFTER.inputFill, bg: AFTER.card }, 1.0);
line('fill vs modal', { fg: BEFORE.inputFill, bg: BEFORE.overlay }, { fg: AFTER.inputFill, bg: AFTER.overlay }, 1.0);
line(
  'border-interactive vs fill (own edge, 3:1)',
  { fg: BEFORE.borderInteractive, bg: BEFORE.page },
  { fg: AFTER.borderInteractive, bg: AFTER.inputFill },
  3,
);

console.log('\nText tiers vs each surface (AA normal text = 4.5:1):');
for (const [tierName, tierColor] of Object.entries(TEXT)) {
  if (tierName === 'disabled') continue; // deliberately fails AA — see globals.css doc comment
  for (const [surfaceName, beforeBg, afterBg] of [
    ['page', BEFORE.page, AFTER.page],
    ['card', BEFORE.card, AFTER.card],
    ['elevated', BEFORE.elevated, AFTER.elevated],
    ['modal', BEFORE.overlay, AFTER.overlay],
  ]) {
    line(`text-${tierName} vs ${surfaceName}`, { fg: tierColor, bg: beforeBg }, { fg: tierColor, bg: afterBg }, 4.5);
  }
}

console.log('\nControl edges (non-text UI boundary, needs 3:1) vs each surface:');
for (const [surfaceName, beforeBg, afterBg] of [
  ['page', BEFORE.page, AFTER.page],
  ['card', BEFORE.card, AFTER.card],
  ['modal', BEFORE.overlay, AFTER.overlay],
  ['hover', BEFORE.highlight, AFTER.highlight],
]) {
  line(
    `border-interactive vs ${surfaceName}`,
    { fg: BEFORE.borderInteractive, bg: beforeBg },
    { fg: AFTER.borderInteractive, bg: afterBg },
    3,
  );
}

console.log('\nStatus banners (fill vs page, and their own text on the fill):');
for (const status of ['success', 'warning', 'error', 'info']) {
  const beforeBg = BEFORE[`${status}Bg`];
  const afterBg = AFTER[`${status}Bg`];
  line(`${status}-bg vs page`, { fg: beforeBg, bg: BEFORE.page }, { fg: afterBg, bg: AFTER.page });
  line(
    `${status}-text on ${status}-bg`,
    { fg: STATUS_TEXT[status], bg: beforeBg },
    { fg: STATUS_TEXT[status], bg: afterBg },
    4.5,
  );
}

console.log('\nChart & heatmap data colors:');
line(
  'chart-accent (teal) vs page (non-text, 3:1)',
  { fg: '#14b8a6', bg: BEFORE.page },
  { fg: AFTER.chartAccent, bg: AFTER.page },
  3,
);
line(
  'chart-accent (teal) vs card (non-text, 3:1)',
  { fg: '#14b8a6', bg: BEFORE.card },
  { fg: AFTER.chartAccent, bg: AFTER.card },
  3,
);
{
  // Both steps are translucent washes painted on the same cell backdrop —
  // composite each over that backdrop (the page, since `MobileHeatmap` now
  // sits on a flat, cardless section below `sm`) to get the two opaque
  // colors a viewer actually compares, then take their contrast.
  const cellBg = '#0b0b0b';
  const beforeStep0 = compositeOver(BEFORE.heat0, cellBg);
  const beforeStep1 = compositeOver(BEFORE.heat1, cellBg);
  const afterStep0 = compositeOver(AFTER.heat0, cellBg);
  const afterStep1 = compositeOver(AFTER.heat1, cellBg);
  const beforeHeatRatio = contrastRatio(beforeStep0, beforeStep1);
  const afterHeatRatio = contrastRatio(afterStep0, afterStep1);
  console.log(
    `  ${'heatmap: empty (step 0) vs lowest tier (step 1)'.padEnd(46)} ${fmt(beforeHeatRatio).padStart(8)}  ->  ${fmt(afterHeatRatio).padStart(8)}`,
  );
}

console.log('\nPublic booking controls (date / slot / court buttons):');
line(
  'idle fill vs page',
  { fg: BEFORE.controlIdleFill, bg: BEFORE.page },
  { fg: AFTER.controlIdleFill, bg: AFTER.page },
);
line(
  'idle border vs page (was decorative, now a boundary — needs 3:1)',
  { fg: BEFORE.controlIdleBorder, bg: BEFORE.page },
  { fg: AFTER.controlIdleBorder, bg: AFTER.page },
  3,
);
line(
  'selected fill vs idle fill',
  { fg: BEFORE.controlSelectedFill, bg: BEFORE.controlIdleFill },
  { fg: AFTER.controlSelectedFill, bg: AFTER.controlIdleFill },
);
{
  // "Cerrado" used to sit inside a button carrying `opacity-30` on the whole
  // element, which faded the label's own already-AA text color down with it.
  // After: no wrapper opacity (see `.opacity-self` in globals.css) — the
  // label is `text-error-text` at full strength.
  const beforeCerradoOnPage = compositeOver(`rgba(248,113,113,${BEFORE.cerradoOpacity})`, BEFORE.page);
  const beforeRatio = contrastRatio(beforeCerradoOnPage, parseColor(BEFORE.page));
  const afterRatio = ratio(STATUS_TEXT.error, AFTER.page);
  console.log(
    `  ${'"Cerrado" text vs page'.padEnd(46)} ${fmt(beforeRatio).padStart(8)}  ->  ${fmt(afterRatio).padStart(8)} (needs ${fmt(4.5)}, ${verdict(afterRatio, 4.5)})`,
  );
}

console.log('\nDone. Ratios are computed, not estimated — see the parseColor/contrastRatio helpers above.');
