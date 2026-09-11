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
        preset: 'desktop',
        chromeFlags: '--no-sandbox --headless',
        onlyCategories: ['performance', 'accessibility', 'best-practices', 'seo'],
        locale: 'es',
      },
    },
    assert: {
      assertions: {
        // Core Web Vitals thresholds
        'categories:performance': ['error', { minScore: 0.7 }],
        'categories:accessibility': ['warn', { minScore: 0.8 }],
        'categories:best-practices': ['warn', { minScore: 0.8 }],
        'categories:seo': ['warn', { minScore: 0.7 }],

        // Individual Core Web Vitals
        'first-contentful-paint': ['warn', { maxNumericValue: 2000 }],
        'largest-contentful-paint': ['warn', { maxNumericValue: 3000 }],
        'cumulative-layout-shift': ['error', { maxNumericValue: 0.1 }],
        'total-blocking-time': ['warn', { maxNumericValue: 300 }],
        'speed-index': ['warn', { maxNumericValue: 3500 }],
      },
    },
    upload: {
      target: 'filesystem',
      outputDir: './lighthouse-results',
    },
  },
};
