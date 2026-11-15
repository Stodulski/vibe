import { describe, it, expect, vi, beforeEach } from 'vitest';
import { Link } from 'react-router-dom';
import { renderWithProviders, screen, userEvent, waitFor } from '@/test/test-utils';
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

  it('asks before leaving when an opening time was changed', async () => {
    const user = userEvent.setup();
    renderWithProviders(
      <>
        <ScheduleConfig complexId="c1" slug="padel-club" />
        <Link to="/otra">Salir</Link>
      </>,
    );
    await user.selectOptions(await screen.findByRole('combobox', { name: 'Lunes: Apertura' }), '09:00');
    await user.click(screen.getByRole('link', { name: 'Salir' }));
    expect(await screen.findByText('Tenés cambios sin guardar')).toBeInTheDocument();
  });

  it('asks before leaving when a day was marked closed', async () => {
    const user = userEvent.setup();
    renderWithProviders(
      <>
        <ScheduleConfig complexId="c1" slug="padel-club" />
        <Link to="/otra">Salir</Link>
      </>,
    );
    await user.click(await screen.findByRole('checkbox', { name: 'Lunes: Cerrado' }));
    await user.click(screen.getByRole('link', { name: 'Salir' }));
    expect(await screen.findByText('Tenés cambios sin guardar')).toBeInTheDocument();
  });

  it('leaves without asking when nothing changed', async () => {
    const user = userEvent.setup();
    renderWithProviders(
      <>
        <ScheduleConfig complexId="c1" slug="padel-club" />
        <Link to="/otra">Salir</Link>
      </>,
    );
    await screen.findByRole('combobox', { name: 'Lunes: Apertura' });
    await user.click(screen.getByRole('link', { name: 'Salir' }));
    expect(screen.queryByText('Tenés cambios sin guardar')).not.toBeInTheDocument();
  });

  it('leaves without asking once the changes were saved', async () => {
    mockMutate.mockImplementation((_data: unknown, options?: { onSuccess?: () => void }) => {
      options?.onSuccess?.();
    });
    const user = userEvent.setup();
    renderWithProviders(
      <>
        <ScheduleConfig complexId="c1" slug="padel-club" />
        <Link to="/otra">Salir</Link>
      </>,
    );
    await user.selectOptions(await screen.findByRole('combobox', { name: 'Lunes: Apertura' }), '09:00');
    await user.click(screen.getByRole('button', { name: /guardar/i }));
    await waitFor(() => {
      expect(mockMutate).toHaveBeenCalledTimes(1);
    });
    await user.click(screen.getByRole('link', { name: 'Salir' }));
    expect(screen.queryByText('Tenés cambios sin guardar')).not.toBeInTheDocument();
  });
});
