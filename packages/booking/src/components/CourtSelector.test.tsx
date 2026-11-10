import { describe, it, expect, vi, beforeEach } from 'vitest';
import { useState, type ComponentProps } from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { CourtSelector, type SelectedSlot } from './CourtSelector';
import type { AvailabilitySlot, CourtAvailability } from '../api/types';
import { BOOKING_MESSAGES } from '../messages';

const t = BOOKING_MESSAGES;

type HarnessProps = Omit<ComponentProps<typeof CourtSelector>, 'pendingStartTime' | 'onPendingStartTimeChange'>;

/**
 * The selector is controlled: the page holds the pending hour so it can show
 * it as a crumb. This stands in for the page, with the crumb reduced to one
 * button that returns to the hours.
 */
function Harness(props: HarnessProps) {
  const [pendingStartTime, setPendingStartTime] = useState<string | null>(null);
  // The page also owns the selection; mirror it so a chosen court is reflected
  // back, while still reporting the call to the test's spy.
  const [selected, setSelected] = useState(props.selected);
  return (
    <>
      {pendingStartTime !== null && (
        <button
          onClick={() => {
            setPendingStartTime(null);
          }}
        >
          Cambiar horario {pendingStartTime}
        </button>
      )}
      <CourtSelector
        {...props}
        selected={selected}
        onSelect={(selection) => {
          setSelected(selection);
          props.onSelect(selection);
        }}
        pendingStartTime={pendingStartTime}
        onPendingStartTimeChange={setPendingStartTime}
      />
    </>
  );
}

// Mock scrollIntoView for the test DOM
Element.prototype.scrollIntoView = vi.fn();

function slot(start: string, end: string, price: number, available = true): AvailabilitySlot {
  const [h = '0', m = '0'] = start.split(':');
  return {
    start_time: start,
    end_time: end,
    start_min: Number(h) * 60 + Number(m),
    duration_minutes: 90,
    price,
    available,
  };
}

function court(
  id: string,
  name: string,
  type: CourtAvailability['court_type'],
  slots: AvailabilitySlot[],
): CourtAvailability {
  return {
    court_id: id,
    court_name: name,
    sport: 'padel',
    court_type: type,
    slots,
  };
}

// One court, three hours, the middle one already taken.
const oneCourt = [
  court('court-1', 'Cancha 1', 'indoor', [
    slot('08:00', '09:30', 500000),
    slot('09:30', '11:00', 500000, false),
    slot('11:00', '12:30', 600000),
  ]),
];

const firstSlot = slot('08:00', '09:30', 500000);

const defaultProps = {
  courts: oneCourt,
  selectedDate: new Date(2024, 2, 15),
  selected: null as SelectedSlot | null,
  onSelect: vi.fn(),
  onContinue: vi.fn(),
  duration: 90 as const,
  onDurationChange: vi.fn(),
};

const selection: SelectedSlot = {
  courtId: 'court-1',
  courtName: 'Cancha 1',
  sport: 'padel',
  courtType: 'indoor',
  slot: firstSlot,
  durationMinutes: firstSlot.duration_minutes,
  totalPrice: firstSlot.price,
  endTime: firstSlot.end_time,
};

beforeEach(() => {
  vi.clearAllMocks();
});

describe('CourtSelector with nothing to offer', () => {
  it('says so when there are no courts', () => {
    render(<Harness {...defaultProps} courts={[]} />);
    expect(screen.getByText(/sin horarios/i)).toBeInTheDocument();
  });

  it('says so when every hour is taken', () => {
    const allTaken = [court('court-1', 'Cancha 1', 'indoor', [slot('08:00', '09:30', 500000, false)])];
    render(<Harness {...defaultProps} courts={allTaken} />);
    expect(screen.getByText(/sin horarios/i)).toBeInTheDocument();
  });
});

describe('CourtSelector — the hour is the question', () => {
  it('offers every bookable hour', () => {
    render(<Harness {...defaultProps} />);
    expect(screen.getByRole('button', { name: /^08:00\b/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^11:00\b/ })).toBeInTheDocument();
  });

  it('leaves a taken hour out entirely rather than showing it disabled', () => {
    render(<Harness {...defaultProps} />);
    // The grid used to draw 09:30 greyed out. An hour nobody can book is not
    // an option, and a button that cannot be pressed is the page arguing with
    // itself — so it is absent, not disabled.
    expect(screen.queryByRole('button', { name: /^09:30,/ })).not.toBeInTheDocument();
  });

  it('leaves the duration control to the page, not to itself', () => {
    // It used to draw one per court card — twelve courts, twelve copies of the
    // same control. The duration is a filter over the whole search, so it
    // moved up beside the date and the sport, and this component stopped
    // owning it. Asserted here because "the control appears once" is only
    // guaranteed while it is rendered in exactly one place.
    const twoCourts = [
      court('court-1', 'Cancha 1', 'indoor', [slot('08:00', '09:30', 500000)]),
      court('court-2', 'Cancha 2', 'indoor', [slot('08:00', '09:30', 500000)]),
    ];
    render(<Harness {...defaultProps} courts={twoCourts} />);

    expect(screen.queryByRole('group', { name: /duración/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^90/ })).not.toBeInTheDocument();
  });
});

describe('CourtSelector — the court is always asked after the hour', () => {
  it('shows the only court after the hour, and tapping it continues', async () => {
    const user = userEvent.setup();
    render(<Harness {...defaultProps} />);

    await user.click(screen.getByRole('button', { name: /^08:00\b/ }));
    expect(defaultProps.onSelect).not.toHaveBeenCalled();
    expect(defaultProps.onContinue).not.toHaveBeenCalled();

    await user.click(screen.getByRole('button', { name: /^Cancha 1\b/ }));
    expect(defaultProps.onContinue).toHaveBeenCalledWith(
      expect.objectContaining({ courtId: 'court-1', courtName: 'Cancha 1' }),
    );
  });

  it('lists every free court even when they are interchangeable', async () => {
    const user = userEvent.setup();
    const twins = [
      court('court-1', 'Cancha 1', 'indoor', [slot('08:00', '09:30', 500000)]),
      court('court-2', 'Cancha 2', 'indoor', [slot('08:00', '09:30', 500000)]),
    ];
    render(<Harness {...defaultProps} courts={twins} />);

    await user.click(screen.getByRole('button', { name: /^08:00\b/ }));

    expect(screen.getByRole('button', { name: /^Cancha 1\b/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Cancha 2\b/ })).toBeInTheDocument();
    expect(defaultProps.onSelect).not.toHaveBeenCalled();
  });

  it('asks which court when they differ, and books the one chosen', async () => {
    const user = userEvent.setup();
    const mixed = [
      court('court-1', 'Cancha 1', 'indoor', [slot('08:00', '09:30', 500000)]),
      court('court-2', 'Cancha 2', 'outdoor', [slot('08:00', '09:30', 400000)]),
    ];
    render(<Harness {...defaultProps} courts={mixed} />);

    await user.click(screen.getByRole('button', { name: /^08:00\b/ }));
    expect(defaultProps.onSelect).not.toHaveBeenCalled();

    // The card's accessible name carries type, sport and price after the
    // court's name, so match the name at the start.
    await user.click(screen.getByRole('button', { name: /^Cancha 2\b/ }));
    expect(defaultProps.onSelect).toHaveBeenCalledWith(
      expect.objectContaining({ courtId: 'court-2', courtType: 'outdoor' }),
    );
  });
});

describe('CourtSelector — "Última cancha"', () => {
  it('is not said at a venue with a single court', () => {
    render(<Harness {...defaultProps} />);
    expect(screen.queryByText(t.publicBooking.lastCourtLeft)).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '08:00' })).toBeInTheDocument();
  });

  it('is said when only one of several courts is still free at that hour', () => {
    const twoCourts = [
      court('court-1', 'Cancha 1', 'indoor', [slot('08:00', '09:30', 500000), slot('11:00', '12:30', 500000)]),
      court('court-2', 'Cancha 2', 'indoor', [slot('11:00', '12:30', 500000)]),
    ];
    render(<Harness {...defaultProps} courts={twoCourts} />);
    expect(screen.getByRole('button', { name: `08:00, ${t.publicBooking.lastCourtLeft}` })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '11:00' })).toBeInTheDocument();
  });
});

describe('CourtSelector — the question owns the screen', () => {
  it('replaces the hours with the court cards while the question is open', async () => {
    const user = userEvent.setup();
    const mixed = [
      court('court-1', 'Cancha 1', 'indoor', [slot('08:00', '09:30', 500000)]),
      court('court-2', 'Cancha 2', 'outdoor', [slot('08:00', '09:30', 400000)]),
    ];
    render(<Harness {...defaultProps} courts={mixed} />);

    await user.click(screen.getByRole('button', { name: /^08:00\b/ }));

    // Hours and courts were both on screen once, and a visitor could tap a
    // second hour while a court from the first stayed highlighted.
    expect(screen.queryByRole('button', { name: /^08:00\b/ })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Cancha 1\b/ })).toBeInTheDocument();
    // And the page was told which hour is open, so it can show the crumb.
    expect(screen.getByRole('button', { name: 'Cambiar horario 08:00' })).toBeInTheDocument();
  });

  it('continues in the same tap that chooses the court, with no button', async () => {
    const user = userEvent.setup();
    const mixed = [
      court('court-1', 'Cancha 1', 'indoor', [slot('08:00', '09:30', 500000)]),
      court('court-2', 'Cancha 2', 'outdoor', [slot('08:00', '09:30', 400000)]),
    ];
    render(<Harness {...defaultProps} courts={mixed} />);

    await user.click(screen.getByRole('button', { name: /^08:00\b/ }));
    expect(screen.queryByRole('button', { name: /continuar/i })).not.toBeInTheDocument();
    expect(defaultProps.onContinue).not.toHaveBeenCalled();

    await user.click(screen.getByRole('button', { name: /^Cancha 2\b/ }));
    // The selection travels with the call: the page has not stored it yet.
    expect(defaultProps.onContinue).toHaveBeenCalledWith(expect.objectContaining({ courtId: 'court-2' }));
  });

  it('returns to the hours when the page clears the pending hour', async () => {
    const user = userEvent.setup();
    const mixed = [
      court('court-1', 'Cancha 1', 'indoor', [slot('08:00', '09:30', 500000)]),
      court('court-2', 'Cancha 2', 'outdoor', [slot('08:00', '09:30', 400000)]),
    ];
    render(<Harness {...defaultProps} courts={mixed} />);

    await user.click(screen.getByRole('button', { name: /^08:00\b/ }));
    await user.click(screen.getByRole('button', { name: /^Cambiar horario/ }));

    expect(screen.getByRole('button', { name: /^08:00\b/ })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^Cancha 1\b/ })).not.toBeInTheDocument();
  });

  it('renders the hours instead of a stale court question when its hour is no longer on offer', () => {
    const mixed = [
      court('court-1', 'Cancha 1', 'indoor', [slot('08:00', '09:30', 500000)]),
      court('court-2', 'Cancha 2', 'outdoor', [slot('08:00', '09:30', 400000)]),
    ];
    const onPendingStartTimeChange = vi.fn();
    // The page still says 08:00 is open, but a refetch no longer offers it.
    render(
      <CourtSelector
        {...defaultProps}
        courts={mixed.map((c) => ({ ...c, slots: [slot('11:00', '12:30', 500000)] }))}
        pendingStartTime="08:00"
        onPendingStartTimeChange={onPendingStartTimeChange}
      />,
    );

    // The correction is derived by the page from the same availability this
    // grid is built from (`isStartTimeStillOffered` in `useSlotInvalidation.ts`),
    // not written back up from here — so this component never calls back.
    expect(onPendingStartTimeChange).not.toHaveBeenCalled();
    expect(screen.getByRole('button', { name: /^11:00\b/ })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^Cancha/ })).not.toBeInTheDocument();
  });
});

describe('CourtSelector — U-06: an hour restored from the confirm page', () => {
  // "Cambiar horario" on the confirm page sends the visitor back with the
  // chosen hour in `?time=`, so `pendingStartTime` arrives already set.
  it('reopens the court question for that hour, even with a single court', () => {
    render(<CourtSelector {...defaultProps} pendingStartTime="08:00" onPendingStartTimeChange={vi.fn()} />);

    expect(screen.getByText(t.publicBooking.selectCourt)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Cancha 1\b/ })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^11:00\b/ })).not.toBeInTheDocument();
  });

  it('brings the reopened court question into view, since the page loads at the top', async () => {
    const scrollIntoView = vi.spyOn(Element.prototype, 'scrollIntoView').mockImplementation(() => undefined);
    render(<CourtSelector {...defaultProps} pendingStartTime="08:00" onPendingStartTimeChange={vi.fn()} />);

    await waitFor(() => {
      expect(scrollIntoView).toHaveBeenCalledTimes(1);
    });
    expect(scrollIntoView).toHaveBeenCalledWith({ block: 'start' });
    scrollIntoView.mockRestore();
  });

  it('brings the hours into view when the page opens at #horarios ("Cambiar horario")', async () => {
    window.history.replaceState(null, '', '/c/club-norte?duration=90#horarios');
    const scrollIntoView = vi.spyOn(Element.prototype, 'scrollIntoView').mockImplementation(() => undefined);
    render(<CourtSelector {...defaultProps} pendingStartTime={null} onPendingStartTimeChange={vi.fn()} />);

    await waitFor(() => {
      expect(scrollIntoView).toHaveBeenCalledTimes(1);
    });
    expect(scrollIntoView.mock.contexts[0]).toBe(screen.getByTestId('time-options'));
    expect(screen.getByRole('button', { name: /^08:00\b/ })).toBeInTheDocument();
    scrollIntoView.mockRestore();
    window.history.replaceState(null, '', '/');
  });

  it('does not scroll when the hours open with nothing chosen', async () => {
    const scrollIntoView = vi.spyOn(Element.prototype, 'scrollIntoView').mockImplementation(() => undefined);
    render(<CourtSelector {...defaultProps} pendingStartTime={null} onPendingStartTimeChange={vi.fn()} />);

    await new Promise((resolve) => {
      requestAnimationFrame(resolve);
    });
    expect(scrollIntoView).not.toHaveBeenCalled();
    scrollIntoView.mockRestore();
  });
});

describe('CourtSelector — no Continue button', () => {
  it('shows no Continue button even with a settled selection', () => {
    render(<Harness {...defaultProps} selected={selection} />);
    expect(screen.queryByRole('button', { name: /continuar/i })).not.toBeInTheDocument();
  });
});
