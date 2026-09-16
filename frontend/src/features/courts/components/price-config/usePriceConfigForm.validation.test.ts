import { describe, it, expect, vi, beforeEach } from 'vitest';
import { act, waitFor } from '@testing-library/react';
import { courtsApi } from '../../api/courts.api';
import { renderForm, schedule, submit } from './priceConfigHarness';

vi.mock('../../api/courts.api', () => ({
  courtsApi: { updatePrices: vi.fn() },
}));

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

beforeEach(() => {
  vi.clearAllMocks();
});

describe('usePriceConfigForm — what the dialog refuses to send', () => {
  it('rejects a negative full-day price on the field, client-side, without sending a request', async () => {
    const { result } = renderForm();

    act(() => {
      result.current.form.setValue('monday.price', -50);
    });
    await submit(result);

    await waitFor(() => {
      expect(result.current.form.formState.errors.monday?.price?.message).toBe('El precio no puede ser negativo');
    });
    expect(courtsApi.updatePrices).not.toHaveBeenCalled();
  });

  it('refuses a row whose ends are equal', async () => {
    const { result } = renderForm({ schedules: [schedule({ day: 'monday' })] });

    act(() => {
      result.current.form.setValue('monday', {
        price: 100,
        bands: [{ time_from: '10:00', time_to: '10:00', price: 150 }],
      });
    });
    await submit(result);

    await waitFor(() => {
      expect(result.current.form.formState.errors.monday?.bands?.[0]?.time_to?.message).toBe('Rango inválido');
    });
    expect(courtsApi.updatePrices).not.toHaveBeenCalled();
  });

  // A row carves an exception out of the full-day price; the moment one
  // exists, that price has to cover the rest of the day.
  it('requires the full-day price once the day has a row', async () => {
    const { result } = renderForm({ schedules: [schedule({ day: 'saturday' })] });

    act(() => {
      result.current.form.setValue('saturday', {
        price: Number.NaN,
        bands: [{ time_from: '18:00', time_to: '23:00', price: 200 }],
      });
    });
    await submit(result);

    await waitFor(() => {
      expect(result.current.form.formState.errors.saturday?.price?.message).toBe('Poné un precio');
    });
    expect(courtsApi.updatePrices).not.toHaveBeenCalled();
  });

  it('requires a price on every row', async () => {
    const { result } = renderForm({ schedules: [schedule({ day: 'saturday' })] });

    act(() => {
      result.current.form.setValue('saturday', {
        price: 100,
        bands: [{ time_from: '18:00', time_to: '23:00', price: Number.NaN }],
      });
    });
    await submit(result);

    await waitFor(() => {
      expect(result.current.form.formState.errors.saturday?.bands?.[0]?.price?.message).toBe('Falta el precio');
    });
    expect(courtsApi.updatePrices).not.toHaveBeenCalled();
  });
});

describe('usePriceConfigForm — overlapping rows', () => {
  // Two rows over the same hour cannot both be that hour's rate. The message
  // lands on the LATER row's start, which is the end the owner would move.
  it('refuses two overlapping rows in the same day, on the later one', async () => {
    const { result } = renderForm({ schedules: [schedule({ day: 'friday' })] });

    act(() => {
      result.current.form.setValue('friday', {
        price: 100,
        bands: [
          { time_from: '08:00', time_to: '19:00', price: 150 },
          { time_from: '18:00', time_to: '23:00', price: 180 },
        ],
      });
    });
    await submit(result);

    await waitFor(() => {
      expect(result.current.form.formState.errors.friday?.bands?.[1]?.time_from?.message).toBe('Se superpone');
    });
    expect(courtsApi.updatePrices).not.toHaveBeenCalled();
  });

  // The row that reads backwards is the one the overlap check must not get
  // wrong: compared as clock strings, 22:00–01:30 looks like it swallows the
  // morning row it never touches.
  it('accepts an evening row running past midnight beside a morning row', async () => {
    vi.mocked(courtsApi.updatePrices).mockResolvedValueOnce({ prices: [] });
    const { result } = renderForm({
      schedules: [schedule({ day: 'thursday', open_time: '08:00', close_time: '01:30' })],
    });

    act(() => {
      result.current.form.setValue('thursday', {
        price: 100,
        bands: [{ time_from: '22:00', time_to: '01:30', price: 200 }],
      });
    });
    await submit(result);

    expect(result.current.form.formState.errors.thursday).toBeUndefined();
    expect(courtsApi.updatePrices).toHaveBeenCalledWith('c1', 'ct1', {
      prices: [
        { price: 10000, day_type: 'thursday', time_from: '08:00', time_to: '22:00' },
        { price: 20000, day_type: 'thursday', time_from: '22:00', time_to: '01:30' },
      ],
    });
  });
});
