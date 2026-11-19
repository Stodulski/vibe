import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen } from '@/test/test-utils';
import { CourtForm } from './CourtForm';

const mockCreateMutate = vi.fn();
const mockUpdateMutate = vi.fn();

vi.mock('../hooks/useCreateCourt', () => ({
  useCreateCourt: () => ({
    mutate: mockCreateMutate,
    isPending: false,
  }),
}));

vi.mock('../hooks/useUpdateCourt', () => ({
  useUpdateCourt: () => ({
    mutate: mockUpdateMutate,
    isPending: false,
  }),
}));

describe('CourtForm', () => {
  const defaultProps = {
    open: true,
    onClose: vi.fn(),
    complexId: 'c1',
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('does not ask for a description', () => {
    renderWithProviders(<CourtForm {...defaultProps} />);
    expect(screen.queryByLabelText(/descripci/i)).not.toBeInTheDocument();
  });

  it('does not render when closed', () => {
    renderWithProviders(<CourtForm {...defaultProps} open={false} />);
    expect(screen.queryByText(/crear cancha|nueva cancha/i)).not.toBeInTheDocument();
  });
});
