import { describe, it, expect, vi, beforeEach } from 'vitest';
import { act } from '@testing-library/react';
import { makePrice } from '@/test/factories';
import { courtsApi } from '../../api/courts.api';
import { makeCourt, renderForm, schedule, submit } from './priceConfigHarness';

vi.mock('../../api/courts.api', () => ({
  courtsApi: { updatePrices: vi.fn() },
}));

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

beforeEach(() => {
  vi.clearAllMocks();
});

describe('usePriceConfigForm — defaults', () => {
  it('seeds a court with no prices as a blank full-day price with no rows', () => {
    const { result } = renderForm({
      schedules: [schedule({ day: 'monday', open_time: '09:00', close_time: '22:00' })],
    });

    expect(result.current.form.getValues('monday')).toEqual({ price: Number.NaN, bands: [] });
    // A day with no schedule row still gets an entry — the schedule only
    // matters again at submit time, when the full-day price's hours are
    // filled in.
    expect(result.current.form.getValues('sunday')).toEqual({ price: Number.NaN, bands: [] });
  });

  it("reads a single stored band as the day's full-day price", () => {
    const court = makeCourt([makePrice({ day_type: 'friday', time_from: '08:00', time_to: '23:00', price: 20000 })]);
    const { result } = renderForm({ court, schedules: [schedule({ day: 'friday' })] });

    expect(result.current.form.getValues('friday')).toEqual({ price: 200, bands: [] });
  });

  it('reconstructs several stored bands as a full-day price plus rows', () => {
    const court = makeCourt([
      makePrice({ day_type: 'friday', time_from: '19:00', time_to: '23:00', price: 20000 }),
      makePrice({ day_type: 'friday', time_from: '08:00', time_to: '19:00', price: 12000 }),
    ]);
    const { result } = renderForm({ court, schedules: [schedule({ day: 'friday' })] });

    expect(result.current.form.getValues('friday')).toEqual({
      price: 120,
      bands: [{ time_from: '19:00', time_to: '23:00', price: 200 }],
    });
  });
});

describe('usePriceConfigForm — a day charging one rate', () => {
  it("converts pesos to cents and sends the day's window as its one band", async () => {
    vi.mocked(courtsApi.updatePrices).mockResolvedValueOnce({ prices: [] });
    const { result } = renderForm({
      schedules: [schedule({ day: 'monday', open_time: '08:00', close_time: '23:00' })],
    });

    act(() => {
      result.current.form.setValue('monday.price', 120);
    });
    await submit(result);

    expect(courtsApi.updatePrices).toHaveBeenCalledWith('c1', 'ct1', {
      prices: [{ price: 12000, day_type: 'monday', time_from: '08:00', time_to: '23:00' }],
    });
  });
});

describe('usePriceConfigForm — a day with a differentiated row', () => {
  it('sends the row as its own band and fills the rest of the window at the full-day rate', async () => {
    vi.mocked(courtsApi.updatePrices).mockResolvedValueOnce({ prices: [] });
    const { result } = renderForm({
      schedules: [schedule({ day: 'friday', open_time: '08:00', close_time: '23:00' })],
    });

    act(() => {
      result.current.form.setValue('friday', {
        price: 100,
        bands: [{ time_from: '20:00', time_to: '23:00', price: 200 }],
      });
    });
    await submit(result);

    expect(courtsApi.updatePrices).toHaveBeenCalledWith('c1', 'ct1', {
      prices: [
        { price: 10000, day_type: 'friday', time_from: '08:00', time_to: '20:00' },
        { price: 20000, day_type: 'friday', time_from: '20:00', time_to: '23:00' },
      ],
    });
  });

  it('drops a day entirely once every row is deleted and the full-day price is left blank', async () => {
    vi.mocked(courtsApi.updatePrices).mockResolvedValueOnce({ prices: [] });
    const { result } = renderForm({
      schedules: [
        schedule({ day: 'monday', open_time: '08:00', close_time: '23:00' }),
        schedule({ day: 'friday', open_time: '08:00', close_time: '23:00' }),
      ],
    });

    act(() => {
      result.current.form.setValue('monday.price', 100);
      result.current.form.setValue('friday', { price: Number.NaN, bands: [] });
    });
    await submit(result);

    expect(courtsApi.updatePrices).toHaveBeenCalledWith('c1', 'ct1', {
      prices: [{ price: 10000, day_type: 'monday', time_from: '08:00', time_to: '23:00' }],
    });
  });
});
