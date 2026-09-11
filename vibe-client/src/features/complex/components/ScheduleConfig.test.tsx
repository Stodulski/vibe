import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen, waitFor } from '@/test/test-utils';
import { ScheduleConfig } from './ScheduleConfig';
import type { Schedule } from '@/shared/types/api.types';

const mockSchedules: Schedule[] = [
  {
    id: 's1',
    complex_id: 'c1',
    day: 'monday',
    open_time: '08:00',
    close_time: '23:00',
    is_closed: false,
  },
  {
    id: 's2',
    complex_id: 'c1',
    day: 'tuesday',
    open_time: '08:00',
    close_time: '23:00',
    is_closed: false,
  },
  {
    id: 's3',
    complex_id: 'c1',
    day: 'wednesday',
    open_time: '08:00',
    close_time: '23:00',
    is_closed: false,
  },
  {
    id: 's4',
    complex_id: 'c1',
    day: 'thursday',
    open_time: '08:00',
    close_time: '23:00',
    is_closed: false,
  },
  {
    id: 's5',
    complex_id: 'c1',
    day: 'friday',
    open_time: '08:00',
    close_time: '23:00',
    is_closed: false,
  },
  {
    id: 's6',
    complex_id: 'c1',
    day: 'saturday',
    open_time: '09:00',
    close_time: '22:00',
    is_closed: false,
  },
  {
    id: 's7',
    complex_id: 'c1',
    day: 'sunday',
    open_time: '09:00',
    close_time: '20:00',
    is_closed: true,
  },
];

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

describe('ScheduleConfig', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseSchedules.mockReturnValue({
      data: mockSchedules,
      isLoading: false,
      isError: false,
      refetch: mockRefetch,
    });
  });

  it('renders all day names', async () => {
    renderWithProviders(<ScheduleConfig complexId="c1" slug="padel-club" />);
    await waitFor(() => {
      expect(screen.getAllByText(/lunes/i).length).toBeGreaterThan(0);
      expect(screen.getAllByText(/martes/i).length).toBeGreaterThan(0);
      expect(screen.getAllByText(/mi.rcoles/i).length).toBeGreaterThan(0);
      expect(screen.getAllByText(/jueves/i).length).toBeGreaterThan(0);
      expect(screen.getAllByText(/viernes/i).length).toBeGreaterThan(0);
      expect(screen.getAllByText(/s.bado/i).length).toBeGreaterThan(0);
      expect(screen.getAllByText(/domingo/i).length).toBeGreaterThan(0);
    });
  });

  it('renders save button', () => {
    renderWithProviders(<ScheduleConfig complexId="c1" slug="padel-club" />);
    expect(screen.getByRole('button', { name: /guardar/i })).toBeInTheDocument();
  });

  it('gives each day exactly one "closed" control, named by that day', async () => {
    renderWithProviders(<ScheduleConfig complexId="c1" slug="padel-club" />);
    await waitFor(() => {
      expect(screen.getAllByRole('checkbox')).toHaveLength(7);
    });
    // Exactly one, not two. Desktop and mobile used to be separate components
    // that were both always in the DOM, so every day shipped two checkboxes
    // carrying the identical accessible name and a screen reader read the week
    // twice. One row now reorders itself with flex instead of duplicating.
    for (const day of ['Lunes', 'Martes', 'Miércoles', 'Jueves', 'Viernes', 'Sábado', 'Domingo']) {
      expect(screen.getAllByRole('checkbox', { name: `${day}: Cerrado` })).toHaveLength(1);
    }
  });

  it('hides the opening and closing times of a day marked closed', async () => {
    renderWithProviders(<ScheduleConfig complexId="c1" slug="padel-club" />);
    // Sunday is closed in the fixture; Monday is open.
    await waitFor(() => {
      expect(screen.getByRole('combobox', { name: 'Lunes: Apertura' })).toBeInTheDocument();
    });
    expect(screen.queryByRole('combobox', { name: 'Domingo: Apertura' })).not.toBeInTheDocument();
    expect(screen.queryByRole('combobox', { name: 'Domingo: Cierre' })).not.toBeInTheDocument();
  });

  it('renders the loaded hours, not the 08:00-23:00 defaults', async () => {
    renderWithProviders(<ScheduleConfig complexId="c1" slug="padel-club" />);
    await waitFor(() => {
      expect(screen.getByRole('combobox', { name: 'Sábado: Apertura' })).toHaveValue('09:00');
    });
    expect(screen.getByRole('combobox', { name: 'Sábado: Cierre' })).toHaveValue('22:00');
  });
});
