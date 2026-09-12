import { describe, expect, it } from 'vitest';

import { assertBuildEnv } from './assertBuildEnv';

describe('assertBuildEnv', () => {
  it('accepts a build environment with an absolute VITE_APP_URL', () => {
    expect(() => {
      assertBuildEnv({ VITE_APP_URL: 'https://app.vibe.com.ar' });
    }).not.toThrow();
  });

  it('fails the build when VITE_APP_URL is missing or blank', () => {
    expect(() => {
      assertBuildEnv({});
    }).toThrow(/VITE_APP_URL/);
    expect(() => {
      assertBuildEnv({ VITE_APP_URL: '   ' });
    }).toThrow(/VITE_APP_URL/);
  });

  it('fails the build when VITE_APP_URL is not an absolute URL', () => {
    expect(() => {
      assertBuildEnv({ VITE_APP_URL: 'app.vibe.com.ar' });
    }).toThrow(/absolute URL/);
  });
});
