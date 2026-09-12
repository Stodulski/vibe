import { describe, it, expect } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { useNavigate } from 'react-router-dom';
import { useSettingsTabs } from './useSettingsTabs';
import { createWrapper } from '@/test/test-utils';

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
