// @vitest-environment node
import { z } from 'zod';
import {
  paginationMetadataSchema,
  paginatedResponseSchema,
  messageResponseSchema,
  errorResponseSchema,
} from './envelope.schema';

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

describe('errorResponseSchema', () => {
  it('accepts a bare string error', () => {
    expect(errorResponseSchema.safeParse({ error: 'boom' }).success).toBe(true);
  });

  it('accepts the field-map error a 422 answers with', () => {
    // `openapi.yaml`'s `ValidationError`: field name to message, where a
    // message may be free text or one of the stable machine codes the client
    // localizes (`slug_taken`, `deposit_exceeds_price`, …). The schema used
    // to expect `{ message, details? }` instead — a shape the API never sent.
    const result = errorResponseSchema.safeParse({
      error: { slug: 'slug_taken', deposit_percentage: 'deposit_percentage_over_100' },
    });
    expect(result.success).toBe(true);
  });

  it('rejects an error object that is neither a string nor a field map', () => {
    const result = errorResponseSchema.safeParse({ error: { message: 'invalid', details: { slug: 'taken' } } });
    expect(result.success).toBe(false);
  });
});
