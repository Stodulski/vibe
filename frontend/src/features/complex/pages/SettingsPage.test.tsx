import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

vi.mock('@/shared/hooks/usePageTitle', () => ({ usePageTitle: vi.fn() }));
vi.mock('@/shared/tour/useTourReady', () => ({ useTourReady: vi.fn() }));
vi.mock('@/shared/components/common/PageHeader', () => ({
  PageHeader: ({ title }: { title: string }) => <h1>{title}</h1>,
}));
vi.mock('@/shared/components/common/Skeletons', () => ({
  SkeletonSettings: () => <div data-testid="skeleton">Loading...</div>,
}));
vi.mock('@/shared/components/common/ConfirmDialog', () => ({ ConfirmDialog: () => null }));
vi.mock('@/shared/components/ui/button', async () => {
  const { passthrough } = await import('@/test/ui-mocks');
  return { Button: passthrough('button') };
});
vi.mock('@/features/complex/hooks/useSelectedComplex', () => ({
  useSelectedComplex: vi.fn().mockReturnValue({
    complex: { id: 'c1', name: 'Test Club', slug: 'test-club' },
    isLoading: false,
  }),
}));
vi.mock('@/features/complex/hooks/useComplexes', () => ({
  useComplexes: vi.fn().mockReturnValue({ isError: false, refetch: vi.fn() }),
}));
vi.mock('@/features/complex/hooks/useDeleteComplex', () => ({
  useDeleteComplex: () => ({ mutate: vi.fn(), isPending: false }),
}));
vi.mock('@/features/complex/components/ComplexForm', () => ({
  ComplexForm: () => <div data-testid="complex-form">ComplexForm</div>,
}));
vi.mock('@/features/complex/components/ScheduleConfig', () => ({ ScheduleConfig: () => null }));
vi.mock('@/features/complex/components/MPConnectCard', () => ({ MPConnectCard: () => null }));
vi.mock('@/features/complex/components/ImageUpload', () => ({ ImageUpload: () => null }));
vi.mock('@/features/complex/api/complex.api', () => ({
  complexApi: { getPublicComplex: vi.fn() },
}));

import { useSelectedComplex, useComplexes } from '@/features/complex';
import { makeComplex } from '@/test/factories';
import { createWrapper } from '@/test/test-utils';

describe('SettingsPage', () => {
  it('renders page title', async () => {
    const SettingsPage = (await import('./SettingsPage')).default;
    render(<SettingsPage />, { wrapper: createWrapper() });
    expect(screen.getByText('Configuración')).toBeInTheDocument();
  });

  it('shows skeleton when loading', async () => {
    vi.mocked(useSelectedComplex).mockReturnValue({
      complex: null,
      complexes: [],
      selectedComplexId: null,
      needsOnboarding: false,
      isLoading: true,
    });
    const SettingsPage = (await import('./SettingsPage')).default;
    render(<SettingsPage />, { wrapper: createWrapper() });
    expect(screen.getByTestId('skeleton')).toBeInTheDocument();
  });

  it('shows general tab by default', async () => {
    const complex = makeComplex({ id: 'c1', name: 'Test Club', slug: 'test-club' });
    vi.mocked(useSelectedComplex).mockReturnValue({
      complex,
      complexes: [complex],
      selectedComplexId: complex.id,
      needsOnboarding: false,
      isLoading: false,
    });
    const SettingsPage = (await import('./SettingsPage')).default;
    render(<SettingsPage />, { wrapper: createWrapper() });
    expect(screen.getByTestId('complex-form')).toBeInTheDocument();
  });
});

function mockNoComplexSelected() {
  vi.mocked(useSelectedComplex).mockReturnValue({
    complex: null,
    complexes: [],
    selectedComplexId: null,
    needsOnboarding: false,
    isLoading: false,
  });
}

describe('SettingsPage error state', () => {
  it('shows the error state with a retry action instead of the skeleton when the query fails', async () => {
    mockNoComplexSelected();
    vi.mocked(useComplexes).mockReturnValue({
      isError: true,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useComplexes>);
    const SettingsPage = (await import('./SettingsPage')).default;
    render(<SettingsPage />, { wrapper: createWrapper() });
    expect(screen.getByText('No pudimos cargar los datos.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Reintentar' })).toBeInTheDocument();
    expect(screen.queryByTestId('skeleton')).not.toBeInTheDocument();
    expect(screen.queryByTestId('complex-form')).not.toBeInTheDocument();
  });

  it('calls refetch when the retry button is clicked', async () => {
    const refetch = vi.fn();
    mockNoComplexSelected();
    vi.mocked(useComplexes).mockReturnValue({ isError: true, refetch } as unknown as ReturnType<typeof useComplexes>);
    const SettingsPage = (await import('./SettingsPage')).default;
    render(<SettingsPage />, { wrapper: createWrapper() });
    await userEvent.click(screen.getByRole('button', { name: 'Reintentar' }));
    expect(refetch).toHaveBeenCalledTimes(1);
  });
});
