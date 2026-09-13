import { describe, it, expect } from 'vitest';
// @vitest-environment node
import { z } from 'zod';
import { paginationMetadataSchema, paginatedResponseSchema, messageResponseSchema } from './envelope.schema';

describe('paginationMetadataSchema', () => {
  it('accepts a page with every optional field present', () => {
    const result = paginationMetadataSchema.safeParse({
      next_cursor: 'abc123',
      has_more: true,
      total_count: 42,
    });
    expect(result.success).toBe(true);
  });

  it('accepts a page with only the required field', () => {
    const result = paginationMetadataSchema.safeParse({ has_more: false });
    expect(result.success).toBe(true);
  });

  it('rejects a page missing has_more', () => {
    const result = paginationMetadataSchema.safeParse({ next_cursor: 'abc' });
    expect(result.success).toBe(false);
  });
});

describe('paginatedResponseSchema', () => {
  it('validates data/metadata against the supplied item schema', () => {
    const schema = paginatedResponseSchema(z.object({ id: z.string() }));
    const result = schema.safeParse({
      data: [{ id: '1' }, { id: '2' }],
      metadata: { has_more: false },
    });
    expect(result.success).toBe(true);
  });
});

describe('messageResponseSchema', () => {
  it('accepts a bare message', () => {
    expect(messageResponseSchema.safeParse({ message: 'listo' }).success).toBe(true);
  });

  it('allows extra keys alongside message', () => {
    const result = messageResponseSchema.safeParse({ message: 'listo', extra: 1 });
    expect(result.success).toBe(true);
  });
});
