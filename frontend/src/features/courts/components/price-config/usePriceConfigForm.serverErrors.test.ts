import { describe, it, expect, vi, beforeEach } from 'vitest';
import { act, waitFor } from '@testing-library/react';
import { toast } from 'sonner';
import { makeConsumedHttpError } from '@/test/factories';
import { courtsApi } from '../../api/courts.api';
import { renderForm, schedule, submit } from './priceConfigHarness';

vi.mock('../../api/courts.api', () => ({
  courtsApi: { updatePrices: vi.fn() },
}));

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

async function refusal(field: string, message: string) {
  return makeConsumedHttpError(422, { title: 'Validation Failed', errors: [{ field, message }] });
}

beforeEach(() => {
  vi.clearAllMocks();
});

// A server 422 field error is keyed by array index ("prices[1].time_to"), not
// by day name and not by row — the form has to trace that index back to the
// control it came from and put the message there instead of leaving the
// dialog to fail silently, which is the other half of the QA-reported bug.
// The wire entry that index names may be a differentiated row's own band, or
// a gap `buildDayBands` filled at the full-day rate — which has no row of
// its own, only the day's price field.
describe('usePriceConfigForm — placing a server field error on a differentiated row', () => {
  it('maps a server field error back onto the row it came from', async () => {
    vi.mocked(courtsApi.updatePrices).mockRejectedValueOnce(
      await refusal('prices[1].time_to', 'overlaps another band'),
    );
    const { result, onClose } = renderForm({
      schedules: [schedule({ day: 'thursday', open_time: '08:00', close_time: '01:30' })],
    });

    act(() => {
      result.current.form.setValue('thursday', {
        price: 100,
        bands: [{ time_from: '22:00', time_to: '01:30', price: 200 }],
      });
    });
    await submit(result);

    // prices[0] is the full-day gap band (08:00–22:00); prices[1] is the row.
    await waitFor(() => {
      expect(result.current.form.formState.errors.thursday?.bands?.[0]?.time_to?.message).toBe('overlaps another band');
    });
    expect(onClose).not.toHaveBeenCalled();
    // useUpdatePrices stays quiet on a mappable field error so it is not shown
    // twice — once on the row and once as a toast.
    expect(toast.error).not.toHaveBeenCalled();
  });

  // The wire index counts across every day, so an error on the second day's
  // only band is "prices[1]" — which must not land on the first day's field.
  it('counts the wire index across days, not within one', async () => {
    vi.mocked(courtsApi.updatePrices).mockRejectedValueOnce(await refusal('prices[1].price', 'must be positive'));
    const { result } = renderForm({ schedules: [schedule({ day: 'monday' }), schedule({ day: 'tuesday' })] });

    act(() => {
      result.current.form.setValue('monday.price', 100);
      result.current.form.setValue('tuesday.price', 150);
    });
    await submit(result);

    await waitFor(() => {
      expect(result.current.form.formState.errors.tuesday?.price?.message).toBe('must be positive');
    });
    expect(result.current.form.formState.errors.monday).toBeUndefined();
  });
});

describe('usePriceConfigForm — a server error on the full-day gap has nowhere but the price field', () => {
  it('puts a whole-item server error for the gap band on the day’s price field', async () => {
    vi.mocked(courtsApi.updatePrices).mockRejectedValueOnce(await refusal('prices[0]', 'invalid band'));
    const { result } = renderForm({ schedules: [schedule({ day: 'monday' })] });

    act(() => {
      result.current.form.setValue('monday.price', 100);
    });
    await submit(result);

    await waitFor(() => {
      expect(result.current.form.formState.errors.monday?.price?.message).toBe('invalid band');
    });
  });

  // A time-field complaint about the gap band would be about hours the owner
  // never typed — it has nowhere to land, so it falls back to a toast rather
  // than being forced onto the price field.
  it('falls back to a toast for a time-field error on the full-day gap band', async () => {
    vi.mocked(courtsApi.updatePrices).mockRejectedValueOnce(await refusal('prices[0].time_from', 'invalid time'));
    const { result } = renderForm({ schedules: [schedule({ day: 'monday' })] });

    act(() => {
      result.current.form.setValue('monday.price', 100);
    });
    await submit(result);

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('invalid time');
    });
  });

  it('falls back to a toast for an error it cannot place on a control', async () => {
    vi.mocked(courtsApi.updatePrices).mockRejectedValueOnce(await refusal('prices', 'must contain at least one price'));
    const { result } = renderForm({ schedules: [schedule({ day: 'monday' })] });

    act(() => {
      result.current.form.setValue('monday.price', 100);
    });
    await submit(result);

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('must contain at least one price');
    });
  });
});
