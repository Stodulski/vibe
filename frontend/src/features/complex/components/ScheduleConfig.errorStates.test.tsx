import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen } from '@/test/test-utils';
import { ScheduleConfig } from './ScheduleConfig';

const mockMutate = vi.fn();
const mockRefetch = vi.fn();
const mockUseSchedules = vi.fn();

vi.mock('../hooks/useSchedules', () => ({
  useSchedules: (...args: unknown[]) => mockUseSchedules(...args) as unknown,
}));

vi.mock('../hooks/useUpdateSchedules', () => ({
  useUpdateSchedules: () => ({
    mutate: mockMutate,
    isPending: false,
  }),
}));

// Finding A1: `enabled: !!slug` and a failed request both leave `data`
// `undefined` while `isLoading` reads `false`. The buggy component fell
// through to a form seeded with `DEFAULT_SCHEDULES` (08:00-23:00, all open)
// and an enabled "Guardar" that would submit those fabricated hours over
// real ones. None of the states below may render the form.
describe('ScheduleConfig failure states', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('does not render the form while there is no slug yet', () => {
    mockUseSchedules.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: false,
      refetch: mockRefetch,
    });
    renderWithProviders(<ScheduleConfig complexId="c1" slug={undefined} />);
    expect(screen.queryByRole('button', { name: /guardar/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('combobox')).not.toBeInTheDocument();
  });

  it('renders an error state with retry when the schedules query fails, never the form', () => {
    mockUseSchedules.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      refetch: mockRefetch,
    });
    renderWithProviders(<ScheduleConfig complexId="c1" slug="padel-club" />);

    expect(screen.queryByRole('button', { name: /guardar/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('combobox')).not.toBeInTheDocument();

    const retryButton = screen.getByRole('button', { name: /actualizar/i });
    retryButton.click();
    expect(mockRefetch).toHaveBeenCalledTimes(1);
  });

  it('renders a loading state, not the form, while the query is pending', () => {
    mockUseSchedules.mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      refetch: mockRefetch,
    });
    renderWithProviders(<ScheduleConfig complexId="c1" slug="padel-club" />);
    expect(screen.queryByRole('button', { name: /guardar/i })).not.toBeInTheDocument();
  });
});
