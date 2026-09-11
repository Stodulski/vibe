// @vitest-environment node
const { mockPost } = vi.hoisted(() => ({
  mockPost: vi.fn(),
}));

vi.mock('@/shared/lib/ky', () => ({
  default: { post: mockPost },
}));

import { captureAbandonedRegistrationLead, captureAbandonedRegistrationLeadBeacon } from './leads.api';

describe('leads.api', () => {
  beforeEach(() => {
    mockPost.mockReset();
    mockPost.mockReturnValue(Promise.resolve());
  });

  it('captureAbandonedRegistrationLead sends the register source by default', () => {
    captureAbandonedRegistrationLead({ email: 'juan@test.com' });

    expect(mockPost).toHaveBeenCalledWith('public/leads/abandoned-registration', {
      json: { email: 'juan@test.com', source: 'register' },
    });
  });

  it('captureAbandonedRegistrationLead sends the google source when asked', () => {
    captureAbandonedRegistrationLead({ email: 'juan@test.com', source: 'google' });

    expect(mockPost).toHaveBeenCalledWith('public/leads/abandoned-registration', {
      json: { email: 'juan@test.com', source: 'google' },
    });
  });

  it('captureAbandonedRegistrationLead sends the partial profile and drops the empty fields', () => {
    captureAbandonedRegistrationLead({ email: 'juan@test.com', first_name: ' Juan ', last_name: '', phone: '+5411' });

    expect(mockPost).toHaveBeenCalledWith('public/leads/abandoned-registration', {
      json: { email: 'juan@test.com', source: 'register', first_name: 'Juan', phone: '+5411' },
    });
  });

  it('captureAbandonedRegistrationLead swallows a failed request', async () => {
    mockPost.mockReturnValueOnce(Promise.reject(new Error('down')));

    expect(() => {
      captureAbandonedRegistrationLead({ email: 'juan@test.com' });
    }).not.toThrow();
    await Promise.resolve();
  });

  it('captureAbandonedRegistrationLeadBeacon posts a text/plain body carrying the source', async () => {
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
