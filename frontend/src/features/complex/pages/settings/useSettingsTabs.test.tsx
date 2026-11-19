import { describe, it, expect } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { useNavigate } from 'react-router-dom';
import { useSettingsTabs } from './useSettingsTabs';
import { createWrapper } from '@/test/test-utils';
import { makeComplex } from '@/test/factories';
import { server } from '@/test/msw/server';

function Harness() {
  const { activeTab } = useSettingsTabs(null);
  const navigate = useNavigate();
  return (
    <div>
      <span>active:{activeTab}</span>
      <button
        onClick={() => {
          void navigate('?tab=billing');
        }}
      >
        go-billing
      </button>
    </div>
  );
}

function renderHarness(initialEntries: string[]) {
  return render(<Harness />, { wrapper: createWrapper(initialEntries) });
}

describe('useSettingsTabs', () => {
  it('opens the tab named by ?tab=', () => {
    renderHarness(['/settings?tab=billing']);
    expect(screen.getByText('active:billing')).toBeInTheDocument();
  });

  it('falls back to general when the param is missing', () => {
    renderHarness(['/settings']);
    expect(screen.getByText('active:general')).toBeInTheDocument();
  });

  it('resolves a merged-away tab to its new section', () => {
    renderHarness(['/settings?tab=mercadopago']);
    expect(screen.getByText('active:billing')).toBeInTheDocument();
  });

  // The bug this guards against: a `useState(initialTab)` only reads the
  // URL once, so a URL change reaching the same mounted instance — the back
  // button, or another link setting ?tab= — never updated `activeTab`.
  it('follows a URL change on the same mounted instance, not just the initial read', () => {
    renderHarness(['/settings?tab=general']);
    expect(screen.getByText('active:general')).toBeInTheDocument();

    fireEvent.click(screen.getByText('go-billing'));
    expect(screen.getByText('active:billing')).toBeInTheDocument();
  });
});

function instagramStatus(enabled: boolean) {
  return http.get('*/complexes/:complexId/instagram', () =>
    HttpResponse.json({
      enabled,
      connected: false,
      needs_reconnect: false,
      username: null,
      account_type: null,
      expires_at: null,
    }),
  );
}

function TabsHarness() {
  const { tabs, activeTab } = useSettingsTabs(makeComplex({ id: 'c1', slug: 'club' }));
  return (
    <div>
      <span>active:{activeTab}</span>
      <ul>
        {tabs.map((tab) => (
          <li key={tab.value}>tab:{tab.value}</li>
        ))}
      </ul>
    </div>
  );
}

describe('useSettingsTabs Instagram tab', () => {
  it('offers no Instagram tab while the feature is disabled', async () => {
    server.use(instagramStatus(false));
    render(<TabsHarness />, { wrapper: createWrapper(['/settings']) });
    await waitFor(() => {
      expect(screen.getByText('tab:billing')).toBeInTheDocument();
    });
    // Give the status query time to answer before asserting absence.
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(screen.queryByText('tab:instagram')).not.toBeInTheDocument();
  });

  it('offers the Instagram tab when the status says enabled', async () => {
    server.use(instagramStatus(true));
    render(<TabsHarness />, { wrapper: createWrapper(['/settings']) });
    expect(await screen.findByText('tab:instagram')).toBeInTheDocument();
  });

  // A bookmarked or callback link must not open a hidden tab.
  it('sends ?tab=instagram to General when the feature is disabled', async () => {
    server.use(instagramStatus(false));
    render(<TabsHarness />, { wrapper: createWrapper(['/settings?tab=instagram']) });
    expect(await screen.findByText('active:general')).toBeInTheDocument();
  });

  it('opens ?tab=instagram when the feature is enabled', async () => {
    server.use(instagramStatus(true));
    render(<TabsHarness />, { wrapper: createWrapper(['/settings?tab=instagram']) });
    expect(await screen.findByText('tab:instagram')).toBeInTheDocument();
    expect(screen.getByText('active:instagram')).toBeInTheDocument();
  });
});
