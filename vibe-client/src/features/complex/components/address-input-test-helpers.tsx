import { render, fireEvent, act } from '@testing-library/react';
import { vi } from 'vitest';
import { AddressInput } from './AddressInput';

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

// Mock ky api client.
export const mockAutocompleteJson = vi.fn();
export const mockDetailsJson = vi.fn();
interface MockGetOptions {
  searchParams?: Record<string, string>;
}

export const mockGet = vi.fn((endpoint: string, ..._rest: [MockGetOptions?]) => {
  if (endpoint === 'places/autocomplete') return { json: mockAutocompleteJson };
  if (endpoint === 'places/details') return { json: mockDetailsJson };
  return { json: vi.fn() };
});
vi.mock('@/shared/lib/ky', () => ({
  default: { get: (...args: unknown[]) => mockGet(...(args as [string])) },
  withSignal: (signal?: AbortSignal) => (signal ? { signal } : {}),
}));

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
