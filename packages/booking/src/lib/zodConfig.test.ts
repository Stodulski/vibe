import { describe, it, expect } from 'vitest';
import { z } from 'zod';
import './zodConfig';

describe('zodConfig', () => {
  it('disables the JIT so Zod never probes eval under the CSP', () => {
    expect(z.config().jitless).toBe(true);
  });

  it('still parses objects through the interpreter', () => {
    const schema = z.object({ id: z.string(), count: z.number() });
    expect(schema.parse({ id: 'a', count: 1 })).toEqual({ id: 'a', count: 1 });
    expect(schema.safeParse({ id: 1 }).success).toBe(false);
  });
});
