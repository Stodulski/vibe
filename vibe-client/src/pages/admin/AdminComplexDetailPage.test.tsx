import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { makeConsumedHttpError } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useAdminComplexDetail } from '@/features/admin';
import AdminComplexDetailPage from './AdminComplexDetailPage';

const t = ES_AR;

vi.mock('@/shared/hooks/usePageTitle', () => ({
  usePageTitle: vi.fn(),
}));
vi.mock('@/features/admin/hooks/useAdminComplexDetail', () => ({
  useAdminComplexDetail: vi.fn().mockReturnValue({ data: undefined, isLoading: true, isError: false }),
}));
vi.mock('@/features/admin/components/ComplexDetailPanel', () => ({
  ComplexDetailPanel: () => <div data-testid="complex-detail-panel" />,
}));

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/admin/complexes/c1']}>
      <Routes>
        <Route path="/admin/complexes/:id" element={<AdminComplexDetailPage />} />
        <Route path="/admin/complexes" element={<div>Complexes list</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

// Finding M10: a failed query used to redirect silently, treating a 404 and
// a 500 the same way. Now only a genuine 404 redirects; anything else shows
// a retryable error.
describe('AdminComplexDetailPage — not found vs error', () => {
  it('redirects to the complexes list on a 404', async () => {
    vi.mocked(useAdminComplexDetail).mockReturnValueOnce({
      data: undefined,
      isLoading: false,
      isError: true,
      error: await makeConsumedHttpError(404, {}),
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useAdminComplexDetail>);

    renderPage();

    expect(await screen.findByText('Complexes list')).toBeInTheDocument();
  });

  it('shows a retryable error, not a redirect, on a 500', async () => {
    const refetch = vi.fn();
    vi.mocked(useAdminComplexDetail).mockReturnValueOnce({
      data: undefined,
      isLoading: false,
      isError: true,
      error: await makeConsumedHttpError(500, {}),
      refetch,
    } as unknown as ReturnType<typeof useAdminComplexDetail>);

    renderPage();

    expect(await screen.findByText(t.admin.complexes.loadError)).toBeInTheDocument();
    expect(screen.queryByText('Complexes list')).not.toBeInTheDocument();

    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: t.common.refresh }));
    expect(refetch).toHaveBeenCalledTimes(1);
  });

  it('renders the panel once data resolves', async () => {
    vi.mocked(useAdminComplexDetail).mockReturnValueOnce({
      data: { complex: { id: 'c1' } },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useAdminComplexDetail>);

    renderPage();

    expect(await screen.findByTestId('complex-detail-panel')).toBeInTheDocument();
  });
});
