import { renderHook, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/msw/server';
import { createQueryWrapper } from '@/test/test-utils';
import { makeClient } from '@/test/factories';
import { useClients } from './useClients';

function listHandler(name: string) {
  return http.get('*/complexes/:complexId/clients', () =>
    HttpResponse.json({
      clients: [makeClient({ id: `c-${name}`, first_name: name })],
      metadata: { has_more: false },
    }),
  );
}

describe('useClients', () => {
  // DATA-08: the search text is part of the queryKey, so a keystroke asks for
  // a cache entry that does not exist yet. Without `keepPreviousData` the hook
  // hands back `undefined` in that gap and the list someone is filtering blanks
  // out between one letter and the next.
  it('keeps the previous page on screen while a new search is in flight', async () => {
    server.use(listHandler('Ana'));
    const { result, rerender } = renderHook(({ search }) => useClients('cx-1', search), {
      wrapper: createQueryWrapper(),
      initialProps: { search: 'an' },
    });

    await waitFor(() => {
      expect(result.current.data?.pages[0]?.clients[0]?.first_name).toBe('Ana');
    });

    server.use(listHandler('Anabel'));
    rerender({ search: 'ana' });

    // The very next render — the new key has no data of its own yet.
    expect(result.current.data?.pages[0]?.clients[0]?.first_name).toBe('Ana');
    expect(result.current.isPlaceholderData).toBe(true);

    await waitFor(() => {
      expect(result.current.data?.pages[0]?.clients[0]?.first_name).toBe('Anabel');
    });
    expect(result.current.isPlaceholderData).toBe(false);
  });
});
