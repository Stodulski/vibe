import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ES_AR } from '@/shared/i18n/es_AR';
import { makePrice } from '@/test/factories';
import { DayRow } from './DayRow';
import { usePriceConfigForm } from './usePriceConfigForm';
import { ALL_DAYS } from './days';
import { makeCourt, schedule, wrapper } from './priceConfigHarness';
import type { CourtWithPrices } from '@/shared/types/api.types';
import type { DayType } from '@/shared/types/api.types';

vi.mock('../../api/courts.api', () => ({
  courtsApi: { updatePrices: vi.fn() },
}));

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

const t = ES_AR;

/**
 * Mounts one `DayRow` wired to the real `usePriceConfigForm` — resolver
 * included — inside a plain `<form>` with its own submit button, the same
 * way `PriceForm` does. The button is what the overlap spec uses to run full
 * validation (react-hook-form always validates on a submit attempt,
 * regardless of the form's blur/change mode), without reaching into the form
 * instance from outside — a ref written during render is exactly what
 * `react-hooks/refs` refuses, and there is no effect-free way to hand a live
 * RHF instance out through props.
 */
function Harness({
  court = makeCourt(),
  schedules = [schedule({ day: 'friday' })],
  day = 'friday',
}: {
  court?: CourtWithPrices;
  schedules?: ReturnType<typeof schedule>[];
  day?: DayType;
}) {
  const { form, nextBand, onSubmit } = usePriceConfigForm('c1', court, schedules, vi.fn());
  const {
    control,
    register,
    setValue,
    getValues,
    handleSubmit,
    formState: { errors },
  } = form;
  const dayInfo = ALL_DAYS.find((d) => d.value === day);
  if (!dayInfo) throw new Error(`no ALL_DAYS entry for ${day}`);

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        void handleSubmit(onSubmit)();
      }}
    >
      <DayRow
        day={day}
        label={dayInfo.label}
        shortLabel={dayInfo.short}
        control={control}
        register={register}
        setValue={setValue}
        getValues={getValues}
        nextBand={nextBand}
        errors={errors}
      />
      <button type="submit">Guardar</button>
    </form>
  );
}

function renderDay(props: Parameters<typeof Harness>[0] = {}) {
  return render(<Harness {...props} />, { wrapper });
}

describe('DayRow — collapsed by default, exactly the plain row', () => {
  it('shows one editable full-day price field and no differentiated rows', () => {
    renderDay({ day: 'friday' });

    expect(screen.getByRole('spinbutton', { name: /^precio viernes$/i })).toBeInTheDocument();
    expect(screen.queryByText(t.courts.newPrice)).not.toBeInTheDocument();
    expect(screen.queryByText('Desde')).not.toBeInTheDocument();
    const chevron = screen.getByRole('button', { name: /mostrar franjas de viernes/i });
    expect(chevron).toHaveAttribute('aria-expanded', 'false');
  });

  it('says how many differentiated rows a day already has without expanding it', () => {
    const court = makeCourt([
      makePrice({ day_type: 'friday', time_from: '08:00', time_to: '20:00', price: 12000 }),
      makePrice({ day_type: 'friday', time_from: '20:00', time_to: '23:00', price: 20000 }),
    ]);
    renderDay({ court, day: 'friday' });

    // Still collapsed, still just the one price field — the shorter, later
    // band became the row; the longer one is the full-day price shown here.
    expect(screen.getByRole('spinbutton', { name: /^precio viernes$/i })).toHaveValue(120);
    expect(screen.queryByText(t.courts.newPrice)).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: /mostrar franjas de viernes \(1\)/i })).toHaveAttribute(
      'aria-expanded',
      'false',
    );
  });
});

describe('DayRow — expanding, adding, deleting a differentiated row', () => {
  it('reveals "Nuevo precio" and the existing rows once expanded', async () => {
    const user = userEvent.setup();
    const court = makeCourt([
      makePrice({ day_type: 'friday', time_from: '08:00', time_to: '20:00', price: 12000 }),
      makePrice({ day_type: 'friday', time_from: '20:00', time_to: '23:00', price: 20000 }),
    ]);
    renderDay({ court, day: 'friday' });

    await user.click(screen.getByRole('button', { name: /mostrar franjas de viernes \(1\)/i }));

    expect(screen.getByText(t.courts.newPrice)).toBeInTheDocument();
    expect(screen.getByRole('spinbutton', { name: /viernes, franja 1: precio/i })).toHaveValue(200);
    // The full-day price field never disappears behind the disclosure.
    expect(screen.getByRole('spinbutton', { name: /^precio viernes$/i })).toBeInTheDocument();
  });

  it('adding a row keeps the day expanded and shows the new row', async () => {
    const user = userEvent.setup();
    const court = makeCourt([
      makePrice({ day_type: 'friday', time_from: '08:00', time_to: '20:00', price: 12000 }),
      makePrice({ day_type: 'friday', time_from: '20:00', time_to: '23:00', price: 20000 }),
    ]);
    renderDay({ court, day: 'friday' });
    await user.click(screen.getByRole('button', { name: /mostrar franjas de viernes \(1\)/i }));

    await user.click(screen.getByText(t.courts.newPrice));

    expect(screen.getByRole('spinbutton', { name: /viernes, franja 1: precio/i })).toBeInTheDocument();
    expect(screen.getByRole('spinbutton', { name: /viernes, franja 2: precio/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /ocultar franjas de viernes \(2\)/i })).toHaveAttribute(
      'aria-expanded',
      'true',
    );
  });

  it('deleting the only row leaves the day valid and collapsible, full-day price untouched', async () => {
    const user = userEvent.setup();
    const court = makeCourt([
      makePrice({ day_type: 'friday', time_from: '08:00', time_to: '20:00', price: 12000 }),
      makePrice({ day_type: 'friday', time_from: '20:00', time_to: '23:00', price: 20000 }),
    ]);
    renderDay({ court, day: 'friday' });
    await user.click(screen.getByRole('button', { name: /mostrar franjas de viernes \(1\)/i }));

    await user.click(screen.getByRole('button', { name: /eliminar franja.*viernes, franja 1/i }));

    expect(screen.queryByRole('spinbutton', { name: /franja/i })).not.toBeInTheDocument();
    expect(screen.getByRole('spinbutton', { name: /^precio viernes$/i })).toHaveValue(120);

    await user.click(screen.getByRole('button', { name: /ocultar franjas de viernes/i }));
    expect(screen.getByRole('spinbutton', { name: /^precio viernes$/i })).toHaveValue(120);
  });
});

describe('DayRow — overlap between two differentiated rows is still refused', () => {
  it('flags the later row and force-opens a day collapsed by hand', async () => {
    const user = userEvent.setup();
    // Longest (08:00–20:00) becomes the full-day price; the other two — which
    // overlap each other, 20:00–22:00 and 21:00–23:00 — become rows.
    const court = makeCourt([
      makePrice({ day_type: 'friday', time_from: '08:00', time_to: '20:00', price: 10000 }),
      makePrice({ day_type: 'friday', time_from: '20:00', time_to: '22:00', price: 15000 }),
      makePrice({ day_type: 'friday', time_from: '21:00', time_to: '23:00', price: 18000 }),
    ]);
    renderDay({ court, day: 'friday' });

    // Collapsed by hand first — the case a purely data-driven expand/collapse
    // could not represent: an error hidden on a day the owner closed.
    expect(screen.getByRole('button', { name: /mostrar franjas de viernes \(2\)/i })).toHaveAttribute(
      'aria-expanded',
      'false',
    );

    await user.click(screen.getByRole('button', { name: /guardar/i }));

    expect(await screen.findByText(t.validation.bandsOverlap)).toBeInTheDocument();
    // The overlap error forced the day back open instead of hiding it.
    expect(screen.getByRole('button', { name: /ocultar franjas de viernes \(2\)/i })).toHaveAttribute(
      'aria-expanded',
      'true',
    );
  });
});
