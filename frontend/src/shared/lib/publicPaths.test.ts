import { describe, expect, it } from 'vitest';
import { publicComplexPath } from './publicPaths';

describe('publicComplexPath', () => {
  it('puts a complex storefront under the /c/ prefix, so its slug never shares a path with a platform route', () => {
    expect(publicComplexPath('los-alamos')).toBe('/c/los-alamos');
  });
});
