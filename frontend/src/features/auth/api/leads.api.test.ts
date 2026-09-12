// @vitest-environment node
import { http, HttpResponse } from 'msw';
import { server } from '@/test/msw/server';
import {
  captureAbandonedRegistrationLead,
  captureAbandonedRegistrationLeadBeacon,
  type AbandonedLead,
} from './leads.api';

/** Captures the JSON body `captureAbandonedRegistrationLead` sends, once the fire-and-forget POST lands. */
async function capturedLeadBody(lead: AbandonedLead): Promise<unknown> {
  let receivedBody: unknown;
  server.use(
    http.post('*/public/leads/abandoned-registration', async ({ request }) => {
      receivedBody = await request.json();
      return HttpResponse.json({});
    }),
  );

  captureAbandonedRegistrationLead(lead);
  await new Promise((resolve) => setTimeout(resolve, 0));
  return receivedBody;
}

describe('leads.api — captureAbandonedRegistrationLead', () => {
  it('sends the register source by default', async () => {
    const body = await capturedLeadBody({ email: 'juan@test.com' });
    expect(body).toEqual({ email: 'juan@test.com', source: 'register' });
  });

  it('sends the google source when asked', async () => {
    const body = await capturedLeadBody({ email: 'juan@test.com', source: 'google' });
    expect(body).toEqual({ email: 'juan@test.com', source: 'google' });
  });

  it('sends the partial profile and drops the empty fields', async () => {
    const body = await capturedLeadBody({
      email: 'juan@test.com',
      first_name: ' Juan ',
      last_name: '',
      phone: '+5411',
    });
    expect(body).toEqual({ email: 'juan@test.com', source: 'register', first_name: 'Juan', phone: '+5411' });
  });

  it('swallows a failed request', async () => {
    server.use(http.post('*/public/leads/abandoned-registration', () => HttpResponse.error()));

    expect(() => {
      captureAbandonedRegistrationLead({ email: 'juan@test.com' });
    }).not.toThrow();
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
});

// A sibling describe, not nested in the one above: max-lines-per-function
// counts a describe callback's whole body, so this stays a genuinely
// separate top-level call.
describe('leads.api — captureAbandonedRegistrationLeadBeacon', () => {
  it('posts a text/plain body carrying the source', async () => {
    const sendBeacon = vi.fn().mockReturnValue(true);
    vi.stubGlobal('navigator', { sendBeacon });
    vi.stubGlobal(
      'Blob',
      class MockBlob {
        parts: string[];
        options: { type: string };
        constructor(parts: string[], options: { type: string }) {
          this.parts = parts;
          this.options = options;
        }
      },
    );

    captureAbandonedRegistrationLeadBeacon({ email: 'juan@test.com', source: 'google', phone: '+5411' });

    expect(sendBeacon).toHaveBeenCalledTimes(1);
    const [url, blob] = sendBeacon.mock.calls[0] as [string, { parts: string[]; options: { type: string } }];
    expect(url).toMatch(/\/public\/leads\/abandoned-registration$/);
    expect(blob.options.type).toBe('text/plain');
    expect(JSON.parse(blob.parts[0] ?? '')).toEqual({ email: 'juan@test.com', source: 'google', phone: '+5411' });

    vi.unstubAllGlobals();
    await Promise.resolve();
  });
});
