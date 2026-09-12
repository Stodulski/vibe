import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { makeConsumedHttpError } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useAdminUserDetail } from '@/features/admin';
import AdminUserDetailPage from './AdminUserDetailPage';

const t = ES_AR;

vi.mock('@/shared/hooks/usePageTitle', () => ({
  usePageTitle: vi.fn(),
}));
vi.mock('@/features/admin/hooks/useAdminUserDetail', () => ({
  useAdminUserDetail: vi.fn().mockReturnValue({ data: undefined, isLoading: true, isError: false }),
}));
vi.mock('@/features/admin/components/UserDetailPanel', () => ({
  UserDetailPanel: () => <div data-testid="user-detail-panel" />,
}));

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/admin/users/u1']}>
      <Routes>
        <Route path="/admin/users/:id" element={<AdminUserDetailPage />} />
        <Route path="/admin/users" element={<div>Users list</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

// Finding M10: a failed query used to redirect silently, treating a 404 and
// a 500 the same way. Now only a genuine 404 redirects; anything else shows
// a retryable error.
describe('AdminUserDetailPage — not found vs error', () => {
  it('redirects to the users list on a 404', async () => {
    vi.mocked(useAdminUserDetail).mockReturnValueOnce({
      data: undefined,
      isLoading: false,
      isError: true,
      error: await makeConsumedHttpError(404, {}),
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useAdminUserDetail>);

    renderPage();

    expect(await screen.findByText('Users list')).toBeInTheDocument();
  });

  it('shows a retryable error, not a redirect, on a 500', async () => {
    const refetch = vi.fn();
    vi.mocked(useAdminUserDetail).mockReturnValueOnce({
      data: undefined,
      isLoading: false,
      isError: true,
      error: await makeConsumedHttpError(500, {}),
      refetch,
    } as unknown as ReturnType<typeof useAdminUserDetail>);

    renderPage();

    expect(await screen.findByText(t.admin.users.loadError)).toBeInTheDocument();
    expect(screen.queryByText('Users list')).not.toBeInTheDocument();

    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: t.common.refresh }));
    expect(refetch).toHaveBeenCalledTimes(1);
  });

  it('renders the panel once data resolves', async () => {
    vi.mocked(useAdminUserDetail).mockReturnValueOnce({
      data: { user: { id: 'u1' }, complexes: [] },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useAdminUserDetail>);

    renderPage();

    expect(await screen.findByTestId('user-detail-panel')).toBeInTheDocument();
  });
});
