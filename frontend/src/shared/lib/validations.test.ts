import { describe, it, expect } from 'vitest';
import {
  emailField,
  optionalEmailField,
  phoneField,
  firstNameField,
  lastNameField,
  passwordField,
} from './validations';
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

describe('whitespace is not a value (trim before validation)', () => {
  it('trims a padded email before checking it', () => {
    const result = emailField.safeParse('  juan@email.com  ');
    expect(result.success).toBe(true);
    if (result.success) expect(result.data).toBe('juan@email.com');
  });

  it('reports "required" for a whitespace-only email', () => {
    const result = emailField.safeParse('   ');
    expect(result.success).toBe(false);
    if (!result.success) expect(result.error.issues[0]?.message).toBe(ES_AR.validation.emailRequired);
  });

  it('treats a whitespace-only optional email as empty', () => {
    const result = optionalEmailField.safeParse('   ');
    expect(result.success).toBe(true);
    if (result.success) expect(result.data).toBe('');
  });

  it('still rejects a malformed optional email, padded or not', () => {
    expect(optionalEmailField.safeParse('  not-an-email  ').success).toBe(false);
  });

  it('trims a padded phone before checking its shape', () => {
    const result = phoneField.safeParse('  +541155550000  ');
    expect(result.success).toBe(true);
    if (result.success) expect(result.data).toBe('+541155550000');
  });

  it('trims a padded first and last name', () => {
    expect(firstNameField.parse('  Afa  ')).toBe('Afa');
    expect(lastNameField.parse('  Garcia  ')).toBe('Garcia');
  });

  it('reports "required" for a whitespace-only first name', () => {
    const result = firstNameField.safeParse('   ');
    expect(result.success).toBe(false);
    if (!result.success) expect(result.error.issues[0]?.message).toBe(ES_AR.validation.nameRequired);
  });

  it('does not trim a password, whose spaces can be part of the secret', () => {
    expect(passwordField.parse('  secreto1  ')).toBe('  secreto1  ');
  });
});
