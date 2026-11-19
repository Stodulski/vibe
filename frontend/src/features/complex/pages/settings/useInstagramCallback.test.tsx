import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { useLocation } from 'react-router-dom';
import { toast } from 'sonner';
import { ES_AR } from '@/shared/i18n/es_AR';
import { createWrapper } from '@/test/test-utils';
import { useInstagramCallback } from './useInstagramCallback';

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

const t = ES_AR.instagram;

function Harness() {
  useInstagramCallback('c1');
  const location = useLocation();
  return <span>search:{location.search}</span>;
}

function renderAt(url: string) {
  return render(<Harness />, { wrapper: createWrapper([url]) });
}

describe('useInstagramCallback', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('confirms a connection and leaves only ?tab=instagram in the URL', async () => {
    renderAt('/settings?tab=instagram&instagram=connected');
    await waitFor(() => {
      expect(screen.getByText('search:?tab=instagram')).toBeInTheDocument();
    });
    expect(toast.success).toHaveBeenCalledWith(t.connectSuccess, expect.anything());
    expect(toast.error).not.toHaveBeenCalled();
  });

  it.each(['expired', 'denied', 'failed', 'unavailable'] as const)(
    'shows the message for reason=%s and clears the callback params',
    async (reason) => {
      renderAt(`/settings?tab=instagram&instagram=error&reason=${reason}`);
      await waitFor(() => {
        expect(screen.getByText('search:?tab=instagram')).toBeInTheDocument();
      });
      expect(toast.error).toHaveBeenCalledWith(t.callbackErrors[reason], expect.anything());
      expect(toast.success).not.toHaveBeenCalled();
    },
  );

  it('falls back to the generic failure for an unknown reason', async () => {
    renderAt('/settings?tab=instagram&instagram=error&reason=something-new');
    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith(t.callbackErrors.failed, expect.anything());
    });
  });

  it('does nothing on a plain settings URL', async () => {
    renderAt('/settings?tab=billing');
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(screen.getByText('search:?tab=billing')).toBeInTheDocument();
    expect(toast.error).not.toHaveBeenCalled();
    expect(toast.success).not.toHaveBeenCalled();
  });
});
