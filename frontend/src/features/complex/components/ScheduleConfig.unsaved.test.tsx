import { describe, it, expect, vi, beforeEach } from 'vitest';
import { Link } from 'react-router-dom';
import { renderWithProviders, screen, userEvent, waitFor } from '@/test/test-utils';
import { ScheduleConfig } from './ScheduleConfig';
import { DAYS } from './schedule-config/days';
import type { Schedule } from '@/shared/types/api.types';

const schedules: Schedule[] = DAYS.map((day) => ({
  id: `s-${day}`,
  complex_id: 'c1',
  day,
  open_time: '08:00',
  close_time: '23:00',
  is_closed: false,
}));

const mockMutate = vi.fn();

vi.mock('../hooks/useSchedules', () => ({
  useSchedules: () => ({ data: schedules, isLoading: false, isError: false, refetch: vi.fn() }),
}));

vi.mock('../hooks/useUpdateSchedules', () => ({
  useUpdateSchedules: () => ({ mutate: mockMutate, isPending: false }),
}));

function renderWithExit() {
  renderWithProviders(
    <>
      <ScheduleConfig complexId="c1" slug="padel-club" />
      <Link to="/otra">Salir</Link>
    </>,
  );
}

// Before, the day rows wrote with `setValue` and no `shouldDirty`, so the form never read as
// changed and leaving it threw the edit away without a word.
describe('ScheduleConfig unsaved changes', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('asks before leaving when an opening time was changed', async () => {
    const user = userEvent.setup();
    renderWithExit();
    await user.selectOptions(await screen.findByRole('combobox', { name: 'Lunes: Apertura' }), '09:00');
    await user.click(screen.getByRole('link', { name: 'Salir' }));
    expect(await screen.findByText('Tenés cambios sin guardar')).toBeInTheDocument();
  });

  it('asks before leaving when a day was marked closed', async () => {
    const user = userEvent.setup();
    renderWithExit();
    await user.click(await screen.findByRole('checkbox', { name: 'Lunes: Cerrado' }));
    await user.click(screen.getByRole('link', { name: 'Salir' }));
    expect(await screen.findByText('Tenés cambios sin guardar')).toBeInTheDocument();
  });

  it('leaves without asking when nothing changed', async () => {
    const user = userEvent.setup();
    renderWithExit();
    await screen.findByRole('combobox', { name: 'Lunes: Apertura' });
    await user.click(screen.getByRole('link', { name: 'Salir' }));
    expect(screen.queryByText('Tenés cambios sin guardar')).not.toBeInTheDocument();
  });

  it('leaves without asking once the changes were saved', async () => {
    mockMutate.mockImplementation((_data: unknown, options?: { onSuccess?: () => void }) => {
      options?.onSuccess?.();
    });
    const user = userEvent.setup();
    renderWithExit();
    await user.selectOptions(await screen.findByRole('combobox', { name: 'Lunes: Apertura' }), '09:00');
    await user.click(screen.getByRole('button', { name: /guardar/i }));
    await waitFor(() => {
      expect(mockMutate).toHaveBeenCalledTimes(1);
    });
    await user.click(screen.getByRole('link', { name: 'Salir' }));
    expect(screen.queryByText('Tenés cambios sin guardar')).not.toBeInTheDocument();
  });
});
