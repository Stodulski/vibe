// @vitest-environment node
import { makeUser } from '@/test/factories';
import { userSchema, authResponseSchema, refreshResponseSchema, userEnvelopeSchema } from './auth.schema';

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

describe('userEnvelopeSchema', () => {
  it('validates authApi.getMe / updateMe response shape', () => {
    expect(userEnvelopeSchema.safeParse({ user: makeUser() }).success).toBe(true);
  });
});
