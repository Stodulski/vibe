// @vitest-environment node
import {
  loginSchema,
  registerSchema,
  forgotPasswordSchema,
  resetPasswordSchema,
  googleCompleteSchema,
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
