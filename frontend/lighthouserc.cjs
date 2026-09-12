module.exports = {
  ci: {
    collect: {
      url: [
        'http://localhost:5173/login',
        'http://localhost:5173/register',
        'http://localhost:5173/complejo-publico-e2e',
      ],
      numberOfRuns: 3,
      settings: {
        // Owners book courts from their phones. Lighthouse's own valid
        // `preset` choices are 'perf', 'experimental' and 'desktop' — there
        // is no 'mobile' value because mobile (device emulation + network/
        // CPU throttling) is what collecting with no preset already does;
        // 'desktop' was the opt-out from that default (PERF-11).
        chromeFlags: '--no-sandbox --headless',
        onlyCategories: ['performance', 'accessibility', 'best-practices', 'seo'],
        locale: 'es',
      },
    },
    assert: {
      assertions: {
        // Core Web Vitals thresholds
        'categories:performance': ['error', { minScore: 0.85 }],
        'categories:accessibility': ['warn', { minScore: 0.8 }],
        'categories:best-practices': ['warn', { minScore: 0.8 }],
        'categories:seo': ['warn', { minScore: 0.7 }],

        // Individual Core Web Vitals
        'first-contentful-paint': ['warn', { maxNumericValue: 2000 }],
        'largest-contentful-paint': ['error', { maxNumericValue: 2500 }],
        'cumulative-layout-shift': ['error', { maxNumericValue: 0.1 }],
        'total-blocking-time': ['warn', { maxNumericValue: 300 }],
        'speed-index': ['warn', { maxNumericValue: 3500 }],
        // INP replaced FID as a Core Web Vital, but it needs a real
        // interaction during the trace to produce a value. This collection
        // is a plain page load (no scripted click/keypress), so the audit
        // never runs — confirmed locally: all 3 runs on all 3 URLs report
        // 0/didn't-run, which LHCI treats as a hard "auditRan" failure, not
        // a pass. 'warn' avoids gating CI on a metric this collection setup
        // cannot produce; scripting a real interaction (a Lighthouse user
        // flow) would let this move to 'error', but that's a bigger change
        // than this finding's scope.
        'interaction-to-next-paint': ['warn', { maxNumericValue: 200 }],
      },
    },
    upload: {
      target: 'filesystem',
      outputDir: './lighthouse-results',
    },
  },
};
