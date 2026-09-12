// @vitest-environment node
import { personalInfoSchema } from './personalInfo.schema';

const validForm = {
  first_name: 'Juan',
  last_name: 'Garcia',
  email: 'juan@test.com',
  phone: '+541155550000',
};

describe('personalInfoSchema', () => {
  it('accepts a fully valid profile', () => {
    expect(personalInfoSchema.safeParse(validForm).success).toBe(true);
  });

  it('rejects an empty first_name', () => {
    expect(personalInfoSchema.safeParse({ ...validForm, first_name: '' }).success).toBe(false);
  });

  it('rejects an empty last_name', () => {
    expect(personalInfoSchema.safeParse({ ...validForm, last_name: '' }).success).toBe(false);
  });

  it('rejects a malformed email', () => {
    expect(personalInfoSchema.safeParse({ ...validForm, email: 'not-an-email' }).success).toBe(false);
  });

  it('rejects an empty email', () => {
    expect(personalInfoSchema.safeParse({ ...validForm, email: '' }).success).toBe(false);
  });

  it('rejects a phone missing the leading country code plus sign', () => {
    expect(personalInfoSchema.safeParse({ ...validForm, phone: '1155550000' }).success).toBe(false);
  });

  it('rejects a first_name longer than 100 characters', () => {
    const result = personalInfoSchema.safeParse({ ...validForm, first_name: 'a'.repeat(101) });
    expect(result.success).toBe(false);
  });
});
