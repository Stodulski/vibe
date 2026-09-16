import { describe, it, expect } from 'vitest';
// @vitest-environment node
import { makeUser } from '@/test/factories';
import {
  userSchema,
  authResponseSchema,
  refreshResponseSchema,
  updateMeResponseSchema,
  currentUserResponseSchema,
} from './auth.schema';

describe('userSchema', () => {
  it('validates a realistic User fixture', () => {
    expect(userSchema.safeParse(makeUser()).success).toBe(true);
  });

  it('rejects an unknown role', () => {
    const result = userSchema.safeParse(makeUser({ role: 'ghost' as never }));
    expect(result.success).toBe(false);
  });
});

describe('authResponseSchema', () => {
  it('validates authApi.login response shape', () => {
    const result = authResponseSchema.safeParse({ user: makeUser(), csrf_token: 'tok' });
    expect(result.success).toBe(true);
  });
});

describe('refreshResponseSchema', () => {
  it('validates auth/refresh response shape', () => {
    expect(refreshResponseSchema.safeParse({ csrf_token: 'tok' }).success).toBe(true);
  });
});

describe('currentUserResponseSchema', () => {
  it('validates authApi.getMe response shape with a null pending_email', () => {
    const result = currentUserResponseSchema.safeParse({ user: makeUser(), csrf_token: 'tok', pending_email: null });
    expect(result.success).toBe(true);
  });

  it('accepts a non-null pending_email', () => {
    const result = currentUserResponseSchema.safeParse({
      user: makeUser(),
      csrf_token: 'tok',
      pending_email: 'new@test.com',
    });
    expect(result.success).toBe(true);
  });

  it('rejects a response missing pending_email', () => {
    const result = currentUserResponseSchema.safeParse({ user: makeUser(), csrf_token: 'tok' });
    expect(result.success).toBe(false);
  });
});

describe('updateMeResponseSchema', () => {
  it('validates authApi.updateMe response shape', () => {
    expect(
      updateMeResponseSchema.safeParse({ user: makeUser(), pending_email: null, email_change: 'none' }).success,
    ).toBe(true);
  });

  it('accepts a non-null pending_email', () => {
    expect(
      updateMeResponseSchema.safeParse({ user: makeUser(), pending_email: 'new@test.com', email_change: 'requested' })
        .success,
    ).toBe(true);
  });

  it('accepts every email_change outcome', () => {
    for (const outcome of ['none', 'requested', 'failed']) {
      const result = updateMeResponseSchema.safeParse({ user: makeUser(), pending_email: null, email_change: outcome });
      expect(result.success).toBe(true);
    }
  });

  it('rejects a response missing pending_email', () => {
    expect(updateMeResponseSchema.safeParse({ user: makeUser(), email_change: 'none' }).success).toBe(false);
  });

  it('rejects a response missing email_change', () => {
    expect(updateMeResponseSchema.safeParse({ user: makeUser(), pending_email: null }).success).toBe(false);
  });

  it('rejects an unknown email_change value', () => {
    expect(
      updateMeResponseSchema.safeParse({ user: makeUser(), pending_email: null, email_change: 'unknown' }).success,
    ).toBe(false);
  });
});
