import { getSavedFormData, hasCompleteSavedData, STORAGE_KEY } from './savedFormData';

describe('getSavedFormData', () => {
  beforeEach(() => {
    localStorage.clear();
    sessionStorage.clear();
  });

  it('returns an empty object when nothing is saved', () => {
    expect(getSavedFormData()).toEqual({});
  });

  it('parses saved data from localStorage', () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ client_first_name: 'Juan' }));
    expect(getSavedFormData()).toEqual({ client_first_name: 'Juan' });
  });

  it('returns an empty object on malformed JSON (approval: same as before the guard-ladder refactor)', () => {
    localStorage.setItem(STORAGE_KEY, 'not-json{');
    expect(getSavedFormData()).toEqual({});
  });

  it('returns an empty object when the saved value is not an object (e.g. a bare number)', () => {
    localStorage.setItem(STORAGE_KEY, '42');
    expect(getSavedFormData()).toEqual({});
  });

  it('returns an empty object when a field is a number instead of a string', () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ client_first_name: 'Juan', client_phone: 1122334455 }));
    expect(getSavedFormData()).toEqual({});
  });

  it('returns an empty object when a field is null instead of a string', () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ client_first_name: null }));
    expect(getSavedFormData()).toEqual({});
  });
});

describe('hasCompleteSavedData', () => {
  it('returns true when all required fields are present and non-blank', () => {
    expect(
      hasCompleteSavedData({
        client_first_name: 'Juan',
        client_last_name: 'Perez',
        client_phone: '1122334455',
        client_email: 'juan@example.com',
      }),
    ).toBe(true);
  });

  it('returns false when a field is missing', () => {
    expect(
      hasCompleteSavedData({
        client_first_name: 'Juan',
        client_last_name: 'Perez',
        client_phone: '1122334455',
      }),
    ).toBe(false);
  });

  it('returns false when a field is blank/whitespace-only', () => {
    expect(
      hasCompleteSavedData({
        client_first_name: '   ',
        client_last_name: 'Perez',
        client_phone: '1122334455',
        client_email: 'juan@example.com',
      }),
    ).toBe(false);
  });
});
