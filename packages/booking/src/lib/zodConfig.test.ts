import { describe, it, expect } from 'vitest';
import { z } from 'zod';
import './zodConfig';

describe('zodConfig', () => {
  it('disables the JIT so Zod never probes eval under the CSP', () => {
    expect(z.config().jitless).toBe(true);
  });
});
