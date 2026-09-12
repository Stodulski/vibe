import { render, screen, within, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { CourtWithPrices } from '@/shared/types/api.types';

vi.mock('@/shared/hooks/usePageTitle', () => ({ usePageTitle: vi.fn() }));
vi.mock('@/shared/tour/useTourReady', () => ({ useTourReady: vi.fn() }));
interface MockPageHeaderProps {
  title: string;
  action?: { onClick: () => void; label: string };
}

vi.mock('@/shared/components/common/PageHeader', () => ({
  PageHeader: ({ title, action }: MockPageHeaderProps) => (
    <div>
      <h1>{title}</h1>
      {action && <button onClick={action.onClick}>{action.label}</button>}
    </div>
  ),
}));
vi.mock('@/shared/components/common/EmptyState', () => ({
  EmptyState: ({ title, actionLabel, onAction }: { title: string; actionLabel?: string; onAction?: () => void }) => (
    <div data-testid="empty">
      {title}
      {actionLabel && onAction && <button onClick={onAction}>{actionLabel}</button>}
    </div>
  ),
}));
vi.mock('@/shared/components/common/Skeletons', () => ({
  SkeletonCourtCard: () => <div data-testid="skeleton-court">Loading...</div>,
}));
vi.mock('@/features/courts/components/CourtGrid', () => ({
  CourtGrid: () => <div data-testid="court-grid">CourtGrid</div>,
}));
let lastCourtFormProps: { onCreated?: (court: CourtWithPrices) => void } | undefined;
vi.mock('@/features/courts/components/CourtForm', () => ({
  CourtForm: (props: { onCreated?: (court: CourtWithPrices) => void }) => {
    lastCourtFormProps = props;
    return null;
  },
}));
let lastPriceConfigProps: { court: CourtWithPrices } | undefined;
vi.mock('@/features/courts/components/PriceConfig', () => ({
  PriceConfig: (props: { court: CourtWithPrices }) => {
    lastPriceConfigProps = props;
    return null;
  },
}));
vi.mock('@/features/courts/hooks/useCourts', () => ({
  useCourts: vi.fn().mockReturnValue({ data: null, isLoading: true }),
}));
vi.mock('@/features/complex/hooks/useSelectedComplex', () => ({
  useSelectedComplex: vi.fn().mockReturnValue({ selectedComplexId: 'c1' }),
}));

import { useCourts } from '@/features/courts';
import { queryResult } from '@/test/query';
import { makeCourt } from '@/test/factories';

describe('CourtsPage', () => {
  beforeEach(() => {
    lastCourtFormProps = undefined;
    lastPriceConfigProps = undefined;
  });

  it('renders page title', async () => {
    const CourtsPage = (await import('./CourtsPage')).default;
    render(<CourtsPage />);
    expect(screen.getByText('Canchas')).toBeInTheDocument();
  });

  it('shows loading skeleton while fetching', async () => {
    const CourtsPage = (await import('./CourtsPage')).default;
    render(<CourtsPage />);
    expect(screen.getAllByTestId('skeleton-court').length).toBeGreaterThan(0);
  });

  it('shows empty state when no courts', async () => {
    vi.mocked(useCourts).mockReturnValue(queryResult([]));
    const CourtsPage = (await import('./CourtsPage')).default;
    render(<CourtsPage />);
    expect(screen.getByTestId('empty')).toBeInTheDocument();
  });

  it('shows court grid when courts exist', async () => {
    vi.mocked(useCourts).mockReturnValue(queryResult([{ ...makeCourt({ id: '1', name: 'Court 1' }), prices: [] }]));
    const CourtsPage = (await import('./CourtsPage')).default;
    render(<CourtsPage />);
    expect(screen.getByTestId('court-grid')).toBeInTheDocument();
  });

  it('shows the error state with a retry action instead of the empty state when the query fails', async () => {
    vi.mocked(useCourts).mockReturnValue(queryResult([], { isError: true, isSuccess: false, status: 'error' }));
    const CourtsPage = (await import('./CourtsPage')).default;
    render(<CourtsPage />);
    const emptyState = screen.getByTestId('empty');
    expect(emptyState).toHaveTextContent('No pudimos cargar los datos.');
    // The header keeps its persistent "Crear cancha" button; only the
    // empty-state's own create CTA (which would open the form over a page
    // that actually has no courts) must not appear on an error.
    expect(within(emptyState).queryByText('Crear cancha')).not.toBeInTheDocument();
    expect(screen.queryByTestId('court-grid')).not.toBeInTheDocument();
  });

  it('calls refetch when the retry button is clicked', async () => {
    const refetch = vi.fn();
    vi.mocked(useCourts).mockReturnValue(
      queryResult([], { isError: true, isSuccess: false, status: 'error', refetch }),
    );
    const CourtsPage = (await import('./CourtsPage')).default;
    render(<CourtsPage />);
    await userEvent.click(screen.getByRole('button', { name: 'Reintentar' }));
    expect(refetch).toHaveBeenCalledTimes(1);
  });
});

// 03-owner-pages-dashboard.md M8: the page used to keep the raw court object
// `onCreated` handed it in state. It must instead track only the id and look
// the court up from the courts query data, so the price dialog always
// reflects the same record the rest of the page renders from.
describe('CourtsPage price config for a newly created court', () => {
  beforeEach(() => {
    lastCourtFormProps = undefined;
    lastPriceConfigProps = undefined;
  });

  it('shows PriceConfig using the courts query data, not the callback argument', async () => {
    const fromQuery = { ...makeCourt({ id: 'new-court', name: 'Cancha (query)' }), prices: [] };
    vi.mocked(useCourts).mockReturnValue(queryResult([fromQuery]));
    const CourtsPage = (await import('./CourtsPage')).default;
    render(<CourtsPage />);

    const fromCallback = {
      ...makeCourt({ id: 'new-court', name: 'Cancha (callback)' }),
      prices: [],
    };
    lastCourtFormProps?.onCreated?.(fromCallback);

    await waitFor(() => {
      expect(lastPriceConfigProps?.court).toEqual(fromQuery);
    });
  });

  it('does not show PriceConfig for a created court that is not (yet) in the courts query data', async () => {
    vi.mocked(useCourts).mockReturnValue(queryResult([]));
    const CourtsPage = (await import('./CourtsPage')).default;
    render(<CourtsPage />);

    const created = { ...makeCourt({ id: 'not-in-list' }), prices: [] };
    lastCourtFormProps?.onCreated?.(created);

    expect(lastPriceConfigProps).toBeUndefined();
  });
});
