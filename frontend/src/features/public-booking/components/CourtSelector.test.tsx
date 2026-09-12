import { describe, it, expect, vi, beforeEach } from 'vitest';
import { useState, type ComponentProps } from 'react';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { CourtSelector, type SelectedSlot } from './CourtSelector';
import type { AvailabilitySlot, CourtAvailability } from '@/shared/types/api.types';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

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

describe('CourtSelector — the court is asked only when it varies', () => {
  it('assigns the court without asking when there is only one', async () => {
    const user = userEvent.setup();
    render(<Harness {...defaultProps} />);

    await user.click(screen.getByRole('button', { name: /^08:00\b/ }));

    expect(defaultProps.onSelect).toHaveBeenCalledWith(
      expect.objectContaining({ courtId: 'court-1', courtName: 'Cancha 1' }),
    );
  });

  it('assigns without asking when the free courts are interchangeable', async () => {
    const user = userEvent.setup();
    const twins = [
      court('court-1', 'Cancha 1', 'indoor', [slot('08:00', '09:30', 500000)]),
      court('court-2', 'Cancha 2', 'indoor', [slot('08:00', '09:30', 500000)]),
    ];
    render(<Harness {...defaultProps} courts={twins} />);

    await user.click(screen.getByRole('button', { name: /^08:00\b/ }));

    // Same type, same price: there is no decision here, so no question.
    expect(screen.queryByRole('button', { name: 'Cancha 2' })).not.toBeInTheDocument();
    expect(defaultProps.onSelect).toHaveBeenCalledTimes(1);
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

describe('CourtSelector — U-06: a restored hour that never needed a court choice', () => {
  // "Cambiar horario" on the confirm page sends the visitor back to
  // `/:slug` with the previously chosen hour still in `?time=`, regardless of
  // whether that hour ever needed a court question — the confirm page reads
  // it from `slotInfo`, not from whatever `CourtSelector` last wrote. On a
  // single-court hour (or one where every free court is interchangeable),
  // `pendingStartTime` arrives already set to an hour `needsCourtChoice`
  // would say no to. `CourtPicker` renders nothing in that case (by design,
  // for the normal mid-flow path, where the caller has already assigned a
  // court) — so before this fix, the returning visitor's crumb read
  // "Horario 08:00" over a blank panel: no grid, no cards, nothing to tap.
  it('reopens the hour grid, not a blank panel, when the restored hour never needed a court choice', () => {
    // `oneCourt` (single court, three hours): 08:00 has only one free court,
    // so `needsCourtChoice` is false for it.
    render(<CourtSelector {...defaultProps} pendingStartTime="08:00" onPendingStartTimeChange={vi.fn()} />);

    expect(screen.getByRole('button', { name: /^08:00\b/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^11:00\b/ })).toBeInTheDocument();
    expect(screen.queryByText(t.publicBooking.selectCourt)).not.toBeInTheDocument();
  });
});

describe('CourtSelector continue action', () => {
  it('shows the Continue bar once an hour is settled', () => {
    render(<Harness {...defaultProps} selected={selection} />);
    expect(screen.getByRole('button', { name: /continuar/i })).toBeInTheDocument();
  });

  it('calls onContinue when Continue is clicked', async () => {
    const user = userEvent.setup();
    render(<Harness {...defaultProps} selected={selection} />);
    await user.click(screen.getByRole('button', { name: /continuar/i }));
    expect(defaultProps.onContinue).toHaveBeenCalledTimes(1);
  });
});
