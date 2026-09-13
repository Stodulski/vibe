import { describe, it, expect, vi } from 'vitest';
// @vitest-environment node
import { HTTPError } from 'ky';
import type { NormalizedOptions } from 'ky';
import { ApiError } from './ApiError';
import { applyServerFieldErrors, type ServerErrorForm } from './serverFieldErrors';

interface TestForm {
  first_name: string;
  phone: string;
}

function apiError(status: number, body: unknown): ApiError {
  const response = new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
  const request = new Request('https://api.vibe.com.ar/api/v1/auth/google/complete', { method: 'POST' });
  const error = new HTTPError(response, request, {} as NormalizedOptions);
  error.data = body;
  return new ApiError(error);
}

function fakeForm() {
  const form = {
    setError: vi.fn(),
    getValues: vi.fn().mockReturnValue({ first_name: '', phone: '' }),
  };
  return form as unknown as ServerErrorForm<TestForm> & typeof form;
}

/**
 * The exact 422 the backend will send, copied from the contract. A form is
 * where a wrong reading of `errors[]` is most visible: the field messages
 * simply never appear and the person is told nothing about which input the
 * server refused.
 */
const VALIDATION_PROBLEM = {
  type: 'https://vibe.com.ar/problems/validation',
  title: 'Validation Failed',
  status: 422,
  detail: 'the request failed validation',
  instance: '/api/v1/auth/register',
  request_id: '8f14e45f-ceea-467a-9c1d-3f3b4b5a6c7d',
  errors: [
    { field: 'first_name', message: 'must be provided' },
    { field: 'phone', message: 'must be provided' },
  ],
};

describe('applyServerFieldErrors — RFC 9457 problem+json', () => {
  it("puts the backend's real 422 under each field it names", () => {
    const form = fakeForm();
    const applied = applyServerFieldErrors(form, apiError(422, VALIDATION_PROBLEM));

    expect(applied).toBe(true);
    expect(form.setError).toHaveBeenCalledWith('first_name', { type: 'server', message: 'must be provided' });
    expect(form.setError).toHaveBeenCalledWith('phone', { type: 'server', message: 'must be provided' });
    // A field error is on screen, so the generic `detail` would only repeat it.
    expect(form.setError).not.toHaveBeenCalledWith('root', expect.anything());
  });

  it('reads `errors[]` addressed by field', () => {
    const form = fakeForm();
    const applied = applyServerFieldErrors(
      form,
      apiError(422, { title: 'Validation failed', errors: [{ field: 'phone', message: 'invalid' }] }),
    );

    expect(applied).toBe(true);
    expect(form.setError).toHaveBeenCalledWith('phone', { type: 'server', message: 'invalid' });
  });

  it('reads `errors[]` addressed by JSON pointer', () => {
    const form = fakeForm();
    applyServerFieldErrors(
      form,
      apiError(422, { title: 'Validation failed', errors: [{ pointer: '/body/first_name', detail: 'required' }] }),
    );
    expect(form.setError).toHaveBeenCalledWith('first_name', { type: 'server', message: 'required' });
  });

  it('calls onField for each field that got one, so a collapsed group can open', () => {
    const form = fakeForm();
    const onField = vi.fn();
    applyServerFieldErrors(
      form,
      apiError(422, { title: 'Validation failed', errors: [{ field: 'phone', message: 'invalid' }] }),
      { onField },
    );
    expect(onField).toHaveBeenCalledWith('phone');
  });
});

describe('applyServerFieldErrors — what cannot land on a field', () => {
  it('routes an error naming a field this form does not have to `root`', () => {
    const form = fakeForm();
    const applied = applyServerFieldErrors(
      form,
      apiError(422, { title: 'Validation failed', errors: [{ field: 'nickname', message: 'taken' }] }),
    );

    expect(applied).toBe(false);
    expect(form.setError).toHaveBeenCalledWith('root', { type: 'server', message: 'taken' });
  });

  it('honours an explicit `fields` allowlist over the form values', () => {
    const form = fakeForm();
    applyServerFieldErrors(
      form,
      apiError(422, { title: 'Validation failed', errors: [{ field: 'phone', message: 'invalid' }] }),
      { fields: ['first_name'] },
    );
    expect(form.setError).toHaveBeenCalledWith('root', { type: 'server', message: 'invalid' });
  });

  it('puts a message with no field at all on `root`', () => {
    const form = fakeForm();
    applyServerFieldErrors(form, apiError(409, { title: 'slot_taken' }));
    expect(form.setError).toHaveBeenCalledWith('root', { type: 'server', message: 'slot_taken' });
  });

  it('prefers problem+json `detail` over `title` for the root message', () => {
    const form = fakeForm();
    applyServerFieldErrors(form, apiError(409, { title: 'Conflict', detail: 'Ese horario ya fue reservado' }));
    expect(form.setError).toHaveBeenCalledWith('root', { type: 'server', message: 'Ese horario ya fue reservado' });
  });

  it('never sets `root` when a real field already carries the reason', () => {
    const form = fakeForm();
    applyServerFieldErrors(
      form,
      apiError(422, {
        title: 'Validation failed',
        errors: [
          { field: 'phone', message: 'invalid' },
          { field: 'nickname', message: 'taken' },
        ],
      }),
    );
    expect(form.setError).not.toHaveBeenCalledWith('root', expect.anything());
  });

  it("falls back to the caller's message when the body said nothing usable", () => {
    const form = fakeForm();
    applyServerFieldErrors(form, apiError(500, {}), { fallback: 'No pudimos guardar los cambios' });
    expect(form.setError).toHaveBeenCalledWith('root', {
      type: 'server',
      message: 'No pudimos guardar los cambios',
    });
  });

  it('does nothing at all for a failure that never reached the API', () => {
    const form = fakeForm();
    expect(applyServerFieldErrors(form, new Error('Failed to fetch'))).toBe(false);
    expect(form.setError).not.toHaveBeenCalled();
  });
});
