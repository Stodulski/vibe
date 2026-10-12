// @vitest-environment node
import { describe, it, expect } from 'vitest';
import { http, HttpResponse } from 'msw';
import { server } from '../test/msw/server';
import { createPublicBookingApi } from './public-booking.api';
import { createBookingClient } from './client';
import { getProblem } from '../lib/ApiError';

const publicBookingApi = createPublicBookingApi(createBookingClient('http://localhost/api/v1'));

describe('publicBookingApi — next available day', () => {
  it('getNextAvailable calls GET next-available with the from date and the duration', async () => {
    let receivedFrom: string | null = null;
    let receivedDuration: string | null = null;
    server.use(
      http.get('*/public/complexes/:slug/next-available', ({ request }) => {
        const params = new URL(request.url).searchParams;
        receivedFrom = params.get('from');
        receivedDuration = params.get('duration');
        return HttpResponse.json({ date: '2026-03-21' });
      }),
    );

    await expect(publicBookingApi.getNextAvailable('test-club', '2026-03-20', 90)).resolves.toEqual({
      date: '2026-03-21',
    });
    expect(receivedFrom).toBe('2026-03-20');
    expect(receivedDuration).toBe('90');
  });

  it('getNextAvailable leaves the duration out when none was chosen, and accepts a null date', async () => {
    let receivedDuration: string | null = 'unset';
    server.use(
      http.get('*/public/complexes/:slug/next-available', ({ request }) => {
        receivedDuration = new URL(request.url).searchParams.get('duration');
        return HttpResponse.json({ date: null });
      }),
    );

    await expect(publicBookingApi.getNextAvailable('test-club', '2026-03-20', null)).resolves.toEqual({
      date: null,
    });
    expect(receivedDuration).toBeNull();
  });
});

describe('publicBookingApi — waitlist', () => {
  it('joinWaitlist posts the date and the email as JSON and resolves on 204', async () => {
    let receivedBody: unknown;
    let receivedContentType: string | null = null;
    server.use(
      http.post('*/public/complexes/:slug/waitlist', async ({ request }) => {
        receivedBody = await request.json();
        receivedContentType = request.headers.get('Content-Type');
        return new HttpResponse(null, { status: 204 });
      }),
    );

    await expect(
      publicBookingApi.joinWaitlist('test-club', { date: '2026-03-20', email: 'ana@example.com' }),
    ).resolves.toBeUndefined();
    expect(receivedBody).toEqual({ date: '2026-03-20', email: 'ana@example.com' });
    expect(receivedContentType).toContain('application/json');
  });

  it('joinWaitlist rejects with the problem body, so the form can show the email field error', async () => {
    server.use(
      http.post('*/public/complexes/:slug/waitlist', () =>
        HttpResponse.json(
          {
            type: 'https://vibe.com.ar/problems/validation',
            title: 'Datos inválidos',
            status: 422,
            errors: [{ field: 'email', message: 'Ingresá un email válido' }],
          },
          { status: 422, headers: { 'Content-Type': 'application/problem+json' } },
        ),
      ),
    );

    const error: unknown = await publicBookingApi
      .joinWaitlist('test-club', { date: '2026-03-20', email: 'nope' })
      .catch((caught: unknown) => caught);

    expect(getProblem(error)?.errors).toEqual([{ field: 'email', message: 'Ingresá un email válido' }]);
  });
});
