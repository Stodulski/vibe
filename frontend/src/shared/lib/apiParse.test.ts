// @vitest-environment node
import { z } from 'zod';
import { parseResponse, parseWith, ApiResponseError } from './apiParse';

const mockCaptureException = vi.fn<(error: unknown) => void>();

vi.mock('@sentry/react', () => ({
  captureException: (error: unknown) => {
    mockCaptureException(error);
  },
}));

const schema = z
  .object({
    id: z.string(),
    name: z.string(),
  })
  .loose();

describe('parseResponse', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('returns the parsed value on a matching shape', () => {
    const data = { id: '1', name: 'Cancha 1' };
    expect(parseResponse(schema, data, 'test.context')).toEqual(data);
  });

  it('allows extra keys the schema does not declare', () => {
    const data = { id: '1', name: 'Cancha 1', sport: 'padel' };
    const result = parseResponse(schema, data, 'test.context');
    expect(result).toEqual(data);
  });

  it('throws ApiResponseError carrying the context on a mismatched shape', () => {
    const data = { id: '1' };
    expect(() => parseResponse(schema, data, 'bookingsApi.list')).toThrow(ApiResponseError);
    try {
      parseResponse(schema, data, 'bookingsApi.list');
    } catch (error) {
      expect(error).toBeInstanceOf(ApiResponseError);
      const apiError = error as ApiResponseError;
      expect(apiError.context).toBe('bookingsApi.list');
      expect(apiError.issues.fieldErrors.name).toBeDefined();
    }
  });

  it('reports the failure to Sentry', () => {
    const data = { id: '1' };
    expect(() => parseResponse(schema, data, 'test.context')).toThrow();
    expect(mockCaptureException).toHaveBeenCalledTimes(1);
    expect(mockCaptureException).toHaveBeenCalledWith(expect.any(ApiResponseError));
  });

  it('does not report on success', () => {
    parseResponse(schema, { id: '1', name: 'x' }, 'test.context');
    expect(mockCaptureException).not.toHaveBeenCalled();
  });
});

describe('parseWith', () => {
  it('returns a function usable in a .then() chain', async () => {
    const data = { id: '1', name: 'Cancha 1' };
    const result = await Promise.resolve(data).then(parseWith(schema, 'test.context'));
    expect(result).toEqual(data);
  });

  it('rejects the promise chain when the shape does not match', async () => {
    await expect(Promise.resolve({ id: '1' }).then(parseWith(schema, 'test.context'))).rejects.toThrow(
      ApiResponseError,
    );
  });
});
