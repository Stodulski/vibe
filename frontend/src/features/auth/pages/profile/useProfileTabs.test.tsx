import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter, useNavigate } from 'react-router-dom';
import { useProfileTabs } from './useProfileTabs';

function Harness() {
  const { activeTab } = useProfileTabs();
  const navigate = useNavigate();
  return (
    <div>
      <span>active:{activeTab}</span>
      <button
        onClick={() => {
          void navigate('?tab=security');
        }}
      >
        go-security
      </button>
    </div>
  );
}

function renderHarness(initialEntries: string[]) {
  return render(
    <MemoryRouter initialEntries={initialEntries}>
      <Harness />
    </MemoryRouter>,
  );
}

describe('useProfileTabs', () => {
  it('opens the tab named by ?tab=', () => {
    renderHarness(['/profile?tab=security']);
    expect(screen.getByText('active:security')).toBeInTheDocument();
  });

  it('falls back to personal for an unknown tab param', () => {
    renderHarness(['/profile?tab=bogus']);
    expect(screen.getByText('active:personal')).toBeInTheDocument();
  });

  it('falls back to personal when the param is missing', () => {
    renderHarness(['/profile']);
    expect(screen.getByText('active:personal')).toBeInTheDocument();
  });

  // The bug this guards against: a `useState(initialTab)` only reads the
  // URL once, so a URL change reaching the same mounted instance — the back
  // button, or another link setting ?tab= — never updated `activeTab`.
  it('follows a URL change on the same mounted instance, not just the initial read', () => {
    renderHarness(['/profile?tab=personal']);
    expect(screen.getByText('active:personal')).toBeInTheDocument();

    fireEvent.click(screen.getByText('go-security'));
    expect(screen.getByText('active:security')).toBeInTheDocument();
  });
});
