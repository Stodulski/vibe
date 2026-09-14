import { describe, it, expect } from 'vitest';
// @vitest-environment node
import {
  loginSchema,
  registerSchema,
  forgotPasswordSchema,
  resetPasswordSchema,
  googleCompleteSchema,
  loginRedirectStateSchema,
  googleCompleteStateSchema,
} from './auth.schema';

describe('loginSchema', () => {
  it('accepts a valid email and a non-empty password', () => {
    expect(loginSchema.safeParse({ email: 'juan@test.com', password: 'secret' }).success).toBe(true);
  });

  it('rejects an empty email', () => {
    expect(loginSchema.safeParse({ email: '', password: 'secret' }).success).toBe(false);
  });

  it('rejects a malformed email', () => {
    expect(loginSchema.safeParse({ email: 'not-an-email', password: 'secret' }).success).toBe(false);
  });

  it('rejects an empty password', () => {
    expect(loginSchema.safeParse({ email: 'juan@test.com', password: '' }).success).toBe(false);
  });
});

describe('registerSchema', () => {
  const validRegister = {
    first_name: 'Juan',
    last_name: 'Garcia',
    email: 'juan@test.com',
    phone: '+541155550000',
    password: 'Sup3rSecret!',
    confirm_password: 'Sup3rSecret!',
  };

  it('accepts a fully valid registration', () => {
    expect(registerSchema.safeParse(validRegister).success).toBe(true);
  });

  it('rejects an empty first_name', () => {
    expect(registerSchema.safeParse({ ...validRegister, first_name: '' }).success).toBe(false);
  });

  it('rejects a malformed email', () => {
    expect(registerSchema.safeParse({ ...validRegister, email: 'not-an-email' }).success).toBe(false);
  });

  it('rejects a phone missing the leading country code plus sign', () => {
    expect(registerSchema.safeParse({ ...validRegister, phone: '1155550000' }).success).toBe(false);
  });

  it('rejects a password shorter than 8 characters', () => {
    const result = registerSchema.safeParse({ ...validRegister, password: 'short', confirm_password: 'short' });
    expect(result.success).toBe(false);
  });

  it('rejects when confirm_password does not match password, flagging confirm_password', () => {
    const result = registerSchema.safeParse({ ...validRegister, confirm_password: 'somethingElse!' });
    expect(result.success).toBe(false);
    if (!result.success) {
      expect(result.error.issues[0]?.path).toEqual(['confirm_password']);
    }
  });
});

describe('forgotPasswordSchema', () => {
  it('accepts a valid email', () => {
    expect(forgotPasswordSchema.safeParse({ email: 'juan@test.com' }).success).toBe(true);
  });

  it('rejects an empty email', () => {
    expect(forgotPasswordSchema.safeParse({ email: '' }).success).toBe(false);
  });
});

describe('resetPasswordSchema', () => {
  it('accepts a password within the length bounds', () => {
    expect(resetPasswordSchema.safeParse({ password: 'Sup3rSecret!' }).success).toBe(true);
  });

  it('rejects a password shorter than 8 characters', () => {
    expect(resetPasswordSchema.safeParse({ password: 'short' }).success).toBe(false);
  });

  it('rejects a password longer than 72 characters', () => {
    expect(resetPasswordSchema.safeParse({ password: 'a'.repeat(73) }).success).toBe(false);
  });
});

describe('googleCompleteSchema', () => {
  const valid = { first_name: 'Juan', last_name: 'Garcia', phone: '+541155550000' };

  it('accepts a complete profile', () => {
    expect(googleCompleteSchema.safeParse(valid).success).toBe(true);
  });

  it('rejects an empty last_name', () => {
    expect(googleCompleteSchema.safeParse({ ...valid, last_name: '' }).success).toBe(false);
  });

  it('rejects an invalid phone', () => {
    expect(googleCompleteSchema.safeParse({ ...valid, phone: 'not-a-phone' }).success).toBe(false);
  });
});

/**
 * `/register/google` carries one `location.state` that two different readers
 * parse: `GoogleCompletePage` takes the profile out of it, and
 * `useAuthSuccessHandler` — after the profile is completed — takes the `from`
 * `useGoogleExchange` added so a Google sign-up returns to the page the
 * visitor was originally heading for. Neither may choke on the other's keys,
 * which is a property of Zod's strip-by-default objects rather than anything
 * either schema states, so it is pinned here.
 */
describe('/register/google router state carries both the profile and the destination', () => {
  const profileState = {
    profile_token: 'a-profile-token',
    profile: { email: 'nuevo@test.com', first_name: 'Nuevo', last_name: 'Usuario' },
  };
  const withFrom = { ...profileState, from: { pathname: '/bookings' } };

  it('lets googleCompleteStateSchema read the profile past the extra from', () => {
    const parsed = googleCompleteStateSchema.safeParse(withFrom);
    expect(parsed.success).toBe(true);
    expect(parsed.data?.profile_token).toBe('a-profile-token');
  });

  it('lets loginRedirectStateSchema read the destination past the profile keys', () => {
    const parsed = loginRedirectStateSchema.safeParse(withFrom);
    expect(parsed.success).toBe(true);
    expect(parsed.data?.from.pathname).toBe('/bookings');
  });

  // No destination to carry: the state is the plain profile hand-off it has
  // always been, and the redirect reader simply finds nothing.
  it('reports no destination when the state carries only the profile', () => {
    expect(googleCompleteStateSchema.safeParse(profileState).success).toBe(true);
    expect(loginRedirectStateSchema.safeParse(profileState).success).toBe(false);
  });

  it('reports no destination for a from that is not shaped like one', () => {
    expect(loginRedirectStateSchema.safeParse({ ...profileState, from: '/bookings' }).success).toBe(false);
    expect(loginRedirectStateSchema.safeParse({ ...profileState, from: { pathname: 7 } }).success).toBe(false);
  });
});
