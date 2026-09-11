import { describe, it, expect } from 'vitest';
import { emailField } from './validations';
import { ES_AR } from '@/shared/i18n/es_AR';

describe('emailField', () => {
  it('reports "required", not "invalid", for an empty string', () => {
    const result = emailField.safeParse('');
    expect(result.success).toBe(false);
    if (!result.success) {
      expect(result.error.issues[0]?.message).toBe(ES_AR.validation.emailRequired);
    }
  });

  it('reports "invalid" for a non-empty malformed email', () => {
    const result = emailField.safeParse('not-an-email');
    expect(result.success).toBe(false);
    if (!result.success) {
      expect(result.error.issues[0]?.message).toBe(ES_AR.validation.emailInvalid);
    }
  });

  it('accepts a well-formed email', () => {
    const result = emailField.safeParse('juan@email.com');
    expect(result.success).toBe(true);
  });
});
