module.exports = {
  ci: {
    collect: {
      // /complejo-publico-e2e is gone. The job runs the built frontend and no
      // API, so that page asks for its slots, waits for a request that nobody
      // answers, and paints its "Buscando horarios..." skeleton. Measured, it
      // reported a 17.3 s LCP with 60 ms of spread across runs: not a slow
      // page, a timeout with a stopwatch on it. Bringing it back means running
      // the API beside the preview server, the way `make e2e` does.
      url: ['http://localhost:5173/login', 'http://localhost:5173/register'],
      numberOfRuns: 3,
      settings: {
        // Owners book courts from their phones. Lighthouse's own valid
        // `preset` choices are 'perf', 'experimental' and 'desktop' — there
        // is no 'mobile' value because mobile (device emulation + network/
        // CPU throttling) is what collecting with no preset already does;
        // 'desktop' was the opt-out from that default (PERF-11).
        chromeFlags: '--no-sandbox --headless',
        // Throttle for real instead of simulating it. Lighthouse's default
        // (Lantern) estimates the metrics from a dependency graph, and it
        // cannot see that the brand mark in index.html's #app-skeleton paints
        // before any JavaScript runs: it finds the logo in the final DOM — the
        // one React rendered — and models when THAT would paint, which is
        // after the whole entry graph. Same page, same throttling, three ways
        // of measuring: simulated said 2894 ms and blamed React's logo,
        // applied said 1633 ms and blamed the skeleton's, and the browser's own
        // LCP API said 880 ms. Applied is also far steadier here — 34 ms of
        // spread across three runs, against swings of several hundred that
        // made this check pass and fail on identical code.
        throttlingMethod: 'devtools',
        onlyCategories: ['performance', 'accessibility', 'best-practices', 'seo'],
        locale: 'es',
      },
    },
    assert: {
      // Compare the median run, not the best one. LHCI aggregates optimistically
      // by default, so a numeric assertion passes when ANY run squeaks under the
      // threshold — this check was green on a page whose median LCP was 3686 ms
      // because one run of three came in at 2160 ms.
      aggregationMethod: 'median',
      assertions: {
        // Core Web Vitals thresholds
        'categories:performance': ['error', { minScore: 0.85 }],
        // The audit's own bar, and an error rather than a warning: a
        // regression here has to fail something (A11Y-12). Note that it
        // fails the merge, not just the log: the job carries no
        // continue-on-error, whatever this comment used to claim.
        'categories:accessibility': ['error', { minScore: 0.95 }],
        'categories:best-practices': ['warn', { minScore: 0.8 }],
        'categories:seo': ['warn', { minScore: 0.7 }],

        // Individual Core Web Vitals
        'first-contentful-paint': ['warn', { maxNumericValue: 2000 }],
        'largest-contentful-paint': ['error', { maxNumericValue: 2500 }],
        'cumulative-layout-shift': ['error', { maxNumericValue: 0.1 }],
        'total-blocking-time': ['warn', { maxNumericValue: 300 }],
        // Warns now, where it used to sit quietly under the bar, and the
        // threshold is deliberately left alone: measured rather than
        // simulated, these pages report about 6.2 s. Speed Index scores how
        // long the viewport keeps changing, and this shell shows a skeleton
        // and then swaps it for the real screen, so the pixels move twice by
        // design. Whether that is worth fixing — or whether the number is
        // measuring the fade the skeleton was given on purpose — is its own
        // question; moving the threshold to silence it would answer it by
        // pretending.
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
