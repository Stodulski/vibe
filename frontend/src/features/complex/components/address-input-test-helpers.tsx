import { render, fireEvent, act } from '@testing-library/react';
import { vi, beforeEach } from 'vitest';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/msw/server';
import { AddressInput } from './AddressInput';

// This file is not itself a `*.test.tsx` file, so it falls outside
// `tsconfig.test.json`'s `include` (which is what supplies the ambient
// `vitest/globals` types elsewhere) — `describe`/`it`/`beforeEach` are not
// ambiently typed here even though they work fine at runtime. `vi` was
// already imported explicitly; `beforeEach` needs the same treatment.

export const PREDICTIONS = [
  {
    place_id: 'place_1',
    description: 'Av. Libertador 1234, Palermo, CABA, Argentina',
    structured_formatting: {
      main_text: 'Av. Libertador 1234',
      secondary_text: 'Palermo, CABA, Argentina',
    },
  },
  {
    place_id: 'place_2',
    description: 'Av. Libertador 5678, Vicente Lopez, Buenos Aires, Argentina',
    structured_formatting: {
      main_text: 'Av. Libertador 5678',
      secondary_text: 'Vicente Lopez, Buenos Aires, Argentina',
    },
  },
];

export const PLACE_DETAILS = {
  address: 'Avenida del Libertador 1234',
  city: 'Buenos Aires',
  province: 'Ciudad Autónoma de Buenos Aires',
  formatted_address: 'Av. del Libertador 1234, C1425 CABA, Argentina',
  latitude: '-34.574000',
  longitude: '-58.420000',
};

/**
 * `mockGet`/`mockAutocompleteJson`/`mockDetailsJson` stay `vi.fn()`s so every
 * dependent `AddressInput.*.test.tsx` file keeps asserting on them exactly as
 * before (`toHaveBeenCalledWith`, `.mock.calls`, `mockImplementationOnce`,
 * `mockRejectedValue`, a promise that never resolves for the abort test) —
 * but the network call underneath is now real: `AddressInput` renders the
 * real `ky` client, MSW (`src/test/msw`) intercepts the two Google Places
 * proxy endpoints below, and each resolver calls the matching spy so the
 * same assertions still hold. `mockGet` is invoked with the same
 * `(endpoint, { searchParams })` shape the old `vi.mock('@/shared/lib/ky')`
 * stub used to receive.
 */
export const mockAutocompleteJson = vi.fn();
export const mockDetailsJson = vi.fn();
interface MockGetOptions {
  searchParams?: Record<string, string>;
}
export const mockGet = vi.fn((_endpoint: string, ..._rest: [MockGetOptions?]) => {
  /* recording only — see the MSW handlers below for the actual response */
});

function searchParamsOf(request: Request): Record<string, string> {
  return Object.fromEntries(new URL(request.url).searchParams);
}

// `src/test/setup.ts`'s `afterEach(() => server.resetHandlers())` peels any
// `server.use(...)` override back off after every test, so these two are
// re-registered in a `beforeEach` (not a one-time call at module load) —
// otherwise only the first test in a file importing this helper would see
// them.
beforeEach(() => {
  server.use(
    http.get('*/places/autocomplete', async ({ request }) => {
      mockGet('places/autocomplete', { searchParams: searchParamsOf(request) });
      try {
        const data = (await mockAutocompleteJson()) as Record<string, unknown>;
        return HttpResponse.json(data);
      } catch {
        return HttpResponse.error();
      }
    }),
    http.get('*/places/details', async ({ request }) => {
      mockGet('places/details', { searchParams: searchParamsOf(request) });
      try {
        const data = (await mockDetailsJson()) as Record<string, unknown>;
        return HttpResponse.json(data);
      } catch {
        return HttpResponse.error();
      }
    }),
  );
});

export function mockAutocompleteSuccess(data: unknown = PREDICTIONS) {
  mockAutocompleteJson.mockResolvedValue({ predictions: data });
}

export function mockDetailsSuccess(data = PLACE_DETAILS) {
  mockDetailsJson.mockResolvedValue(data);
}

export async function typeAndFlush(input: HTMLElement, value: string) {
  fireEvent.change(input, { target: { value } });
  await act(() => vi.advanceTimersByTimeAsync(550));
}

export function setup(props: Partial<React.ComponentProps<typeof AddressInput>> = {}) {
  const onChange = vi.fn();
  const onSelect = vi.fn();
  const onClear = vi.fn();
  const utils = render(
    <AddressInput
      value=""
      confirmed={false}
      onChange={onChange}
      onSelect={onSelect}
      onClear={onClear}
      placeholder="Ej: Av. Libertador 1234, Palermo, CABA"
      {...props}
    />,
  );
  return { onChange, onSelect, onClear, ...utils };
}
