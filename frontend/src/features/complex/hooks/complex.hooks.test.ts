import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement } from 'react';

vi.mock('../api/complex.api', () => ({
  complexApi: {
    list: vi.fn().mockResolvedValue({ complexes: [{ id: 'c1', name: 'Club' }] }),
    getById: vi.fn().mockResolvedValue({ complex: { id: 'c1' } }),
    create: vi.fn().mockResolvedValue({ complex: { id: 'c2' } }),
    update: vi.fn().mockResolvedValue({ complex: { id: 'c1' } }),
    delete: vi.fn().mockResolvedValue({ message: 'ok' }),
    updateSchedules: vi.fn().mockResolvedValue({ schedules: [] }),
    getPublicComplex: vi.fn().mockResolvedValue({ complex: {}, courts: [], schedules: [] }),
  },
}));

vi.mock('../api/upload.api', () => ({
  uploadApi: {
    presign: vi.fn().mockResolvedValue({ upload_url: 'url', public_url: 'purl', key: 'k' }),
    uploadToR2: vi.fn().mockResolvedValue({ ok: true }),
    deleteImage: vi.fn().mockResolvedValue({}),
  },
}));

vi.mock('../utils/compressImage', () => ({
  compressImage: vi.fn().mockResolvedValue(new Blob()),
  LOGO_OPTIONS: {},
  COVER_OPTIONS: {},
}));

vi.mock('@/shared/lib/queryKeys', () => ({
  queryKeys: {
    complexes: {
      all: ['complexes'],
      detail: (id: string) => ['complexes', id],
      bySlug: (slug: string) => ['complexes', 'slug', slug],
      schedules: (id: string) => ['complexes', id, 'schedules'],
    },
  },
}));

vi.mock('@/shared/stores', () => {
  const state = { selectedComplexId: 'c1', setSelectedComplexId: vi.fn() };
  return {
    // Selector-aware, like the real zustand store: hooks under test call
    // `useStore((s) => s.x)` rather than reading the whole state.
    useStore: Object.assign((selector?: (s: typeof state) => unknown) => (selector ? selector(state) : state), {
      getState: () => state,
    }),
  };
});

vi.mock('react-router-dom', () => ({
  useNavigate: () => vi.fn(),
}));

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

function createWrapper() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

describe('useComplexes', () => {
  it('fetches and selects complexes', async () => {
    const { useComplexes } = await import('./useComplexes');
    const { result } = renderHook(() => useComplexes(), { wrapper: createWrapper() });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(result.current.data).toEqual([{ id: 'c1', name: 'Club' }]);
  });
});

describe('useComplex', () => {
  it('fetches a single complex', async () => {
    const { useComplex } = await import('./useComplex');
    const { result } = renderHook(() => useComplex('c1'), { wrapper: createWrapper() });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
  });

  it('is disabled when id is null', async () => {
    const { useComplex } = await import('./useComplex');
    const { result } = renderHook(() => useComplex(null), { wrapper: createWrapper() });
    expect(result.current.fetchStatus).toBe('idle');
  });
});

describe('useCreateComplex', () => {
  it('returns a mutation', async () => {
    const { useCreateComplex } = await import('./useCreateComplex');
    const { result } = renderHook(() => useCreateComplex(), { wrapper: createWrapper() });
    expect(typeof result.current.mutate).toBe('function');
  }, 10_000);
});

describe('useUpdateComplex', () => {
  it('returns a mutation', async () => {
    const { useUpdateComplex } = await import('./useUpdateComplex');
    const { result } = renderHook(() => useUpdateComplex('c1'), { wrapper: createWrapper() });
    expect(typeof result.current.mutate).toBe('function');
  });
});

describe('useDeleteComplex', () => {
  it('returns a mutation', async () => {
    const { useDeleteComplex } = await import('./useDeleteComplex');
    const { result } = renderHook(() => useDeleteComplex(), { wrapper: createWrapper() });
    expect(typeof result.current.mutate).toBe('function');
  });
});

describe('useSchedules', () => {
  it('fetches schedules', async () => {
    const { useSchedules } = await import('./useSchedules');
    const { result } = renderHook(() => useSchedules('c1', 'test-club'), {
      wrapper: createWrapper(),
    });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
  });

  it('is disabled when slug is undefined', async () => {
    const { useSchedules } = await import('./useSchedules');
    const { result } = renderHook(() => useSchedules('c1', undefined), {
      wrapper: createWrapper(),
    });
    expect(result.current.fetchStatus).toBe('idle');
  });
});

describe('useUpdateSchedules', () => {
  it('returns a mutation', async () => {
    const { useUpdateSchedules } = await import('./useUpdateSchedules');
    const { result } = renderHook(() => useUpdateSchedules('c1'), { wrapper: createWrapper() });
    expect(typeof result.current.mutate).toBe('function');
  });
});

describe('useUploadImage', () => {
  it('returns a mutation', async () => {
    const { useUploadImage } = await import('./useUploadImage');
    const { result } = renderHook(() => useUploadImage('c1'), { wrapper: createWrapper() });
    expect(typeof result.current.mutate).toBe('function');
  });
});

describe('useDeleteImage', () => {
  it('returns a mutation', async () => {
    const { useDeleteImage } = await import('./useUploadImage');
    const { result } = renderHook(() => useDeleteImage('c1'), { wrapper: createWrapper() });
    expect(typeof result.current.mutate).toBe('function');
  });
});
