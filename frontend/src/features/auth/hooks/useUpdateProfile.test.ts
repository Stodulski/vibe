import { describe, it, expect, vi, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { toast } from 'sonner';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement } from 'react';
import { ES_AR } from '@/shared/i18n/es_AR';
import { createQueryWrapper } from '@/test/test-utils';
import { makeUser } from '@/test/factories';

const t = ES_AR;

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

const { mockUpdateMe } = vi.hoisted(() => ({ mockUpdateMe: vi.fn() }));
vi.mock('../api/auth.api', () => ({
  authApi: { updateMe: mockUpdateMe },
}));

const { useUpdateProfile } = await import('./useUpdateProfile');
const { useAuth } = await import('./useAuth');
const { setSessionUser } = await import('./session');

const submitted = {
  first_name: 'Ana',
  last_name: 'Perez',
  email: 'new@example.com',
  phone: '+541155550000',
};

describe('useUpdateProfile', () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  // `email_change: "requested"` is the backend's own word for "a new
  // confirmation link went out" — the hook no longer infers this by
  // comparing the submitted address against the one in the response.
  it('shows the confirmation-sent toast when email_change is "requested"', async () => {
    const user = makeUser({ email: 'ana@example.com' });
    mockUpdateMe.mockResolvedValue({ user, pending_email: 'new@example.com', email_change: 'requested' });

    const { result } = renderHook(() => useUpdateProfile(), { wrapper: createQueryWrapper() });
    result.current.mutate(submitted);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(toast.success).toHaveBeenCalledWith(t.auth.emailChangeConfirmationSent);
    expect(toast.error).not.toHaveBeenCalled();
  });

  // A failed Put must never be read as a silent success: it gets its own
  // error toast, distinct from both the plain-success and the
  // confirmation-sent copy, and the rest of the update (already saved by the
  // backend) must still land in the session cache. `useAuth` is rendered
  // alongside as the cache's observer — without one, `gcTime: 0` (the test
  // query client's default) drops the entry the instant `setSessionUser`
  // writes it, before this test can read it back.
  it('shows an error toast, but still treats the rest of the update as saved, when email_change is "failed"', async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
    const initialUser = makeUser({ id: 'u-1', email: 'ana@example.com', first_name: 'OldName' });
    setSessionUser(queryClient, initialUser);

    const user = makeUser({ id: 'u-1', email: 'ana@example.com', first_name: 'Ana' });
    mockUpdateMe.mockResolvedValue({ user, pending_email: null, email_change: 'failed' });

    const { result } = renderHook(() => ({ auth: useAuth(), update: useUpdateProfile() }), {
      wrapper: ({ children }) => createElement(QueryClientProvider, { client: queryClient }, children),
    });
    result.current.update.mutate(submitted);

    await waitFor(() => {
      expect(result.current.update.isSuccess).toBe(true);
    });
    expect(toast.error).toHaveBeenCalledWith(t.auth.emailChangeRequestFailed);
    expect(toast.success).not.toHaveBeenCalled();

    // The rest of the update is still reflected in the session cache — a
    // failed email-change request must not roll back the fields that did save.
    await waitFor(() => {
      expect(result.current.auth.user?.first_name).toBe('Ana');
    });
    expect(result.current.auth.pendingEmail).toBeNull();
  });
});

// Sibling describe, not nested in the one above: max-lines-per-function
// counts a describe callback's whole body.
describe('useUpdateProfile, remaining outcomes', () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  // No email change was requested at all: the ordinary success toast, same
  // as before this field existed.
  it('shows the plain success toast when email_change is "none"', async () => {
    const user = makeUser({ first_name: 'Renamed' });
    mockUpdateMe.mockResolvedValue({ user, pending_email: null, email_change: 'none' });

    const { result } = renderHook(() => useUpdateProfile(), { wrapper: createQueryWrapper() });
    result.current.mutate({ ...submitted, email: user.email });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(toast.success).toHaveBeenCalledWith(t.auth.profileUpdated);
    expect(toast.error).not.toHaveBeenCalled();
  });

  // Resubmitting the current address with only its case changed is not a
  // request — the backend reports "none", and the hook must not read that
  // as a confirmation having been sent.
  it('shows the plain success toast for a case-only email resubmission ("none")', async () => {
    const user = makeUser({ email: 'ana@example.com' });
    mockUpdateMe.mockResolvedValue({ user, pending_email: null, email_change: 'none' });

    const { result } = renderHook(() => useUpdateProfile(), { wrapper: createQueryWrapper() });
    result.current.mutate({ ...submitted, email: 'ANA@EXAMPLE.COM' });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(toast.success).toHaveBeenCalledWith(t.auth.profileUpdated);
    expect(toast.error).not.toHaveBeenCalled();
  });

  it('shows the generic update-error toast on a request failure', async () => {
    mockUpdateMe.mockRejectedValue(new Error('network error'));

    const { result } = renderHook(() => useUpdateProfile(), { wrapper: createQueryWrapper() });
    result.current.mutate(submitted);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });
    expect(toast.error).toHaveBeenCalledWith(t.auth.updateError);
  });
});
