import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useEffect } from 'react';
import { ImageUpload } from './ImageUpload';
import type { Complex } from '@/shared/types/api.types';

const mockUpload = vi.hoisted(() => vi.fn());
const mockToastError = vi.hoisted(() => vi.fn());

vi.mock('sonner', () => ({ toast: { error: mockToastError, success: vi.fn() } }));

/**
 * happy-dom does not decode images. This stand-in "decodes" a file by its
 * name: `broken.*` fails, `small.*` is 300x300, anything else is 3000x2000.
 */
let decodeAs: 'ok' | 'small' | 'broken' = 'ok';
class FakeImage {
  naturalWidth = 0;
  naturalHeight = 0;
  onload: (() => void) | null = null;
  onerror: (() => void) | null = null;
  set src(_url: string) {
    queueMicrotask(() => {
      if (decodeAs === 'broken') {
        this.onerror?.();
        return;
      }
      const side = decodeAs === 'small' ? 300 : 3000;
      this.naturalWidth = side;
      this.naturalHeight = decodeAs === 'small' ? 300 : 2000;
      this.onload?.();
    });
  }
}

const CROP_AREA = { x: 10, y: 20, width: 160, height: 90 };

// happy-dom has no layout, so the real cropper never measures an area: this
// stand-in reports one, as the real one does once it has laid the image out.
vi.mock('react-easy-crop', () => ({
  default: function MockCropper({ onCropComplete }: { onCropComplete: (a: unknown, p: unknown) => void }) {
    useEffect(() => {
      onCropComplete({}, CROP_AREA);
    }, [onCropComplete]);
    return <div data-testid="cropper" />;
  },
}));

vi.mock('../hooks/useUploadImage', () => ({
  validateImageFile: (f: File) => (f.type === 'image/gif' ? 'tipo invalido' : null),
  useUploadImage: () => ({ mutate: mockUpload, isPending: false }),
  useDeleteImage: () => ({ mutate: vi.fn(), isPending: false }),
}));

const mockComplex: Complex = {
  id: 'c1',
  owner_id: 'u1',
  name: 'Padel Club',
  slug: 'padel-club',
  amenities: [],
  payments_enabled: false,
  address: 'Av. Libertador 1234',
  city: 'Buenos Aires',
  province: 'CABA',
  country_code: 'AR',
  currency: 'ARS',
  phone: '1155550000',
  email: null,
  logo_url: null,
  cover_url: null,
  deposit_percentage: 30,
  cancellation_hours: 24,
  latitude: null,
  longitude: null,
  is_active: true,
  created_at: '2026-01-10T10:00:00Z',
  updated_at: '2026-01-10T10:00:00Z',
  version: 1,
};

describe('crop dialog', () => {
  const pick = (label: string, file: File) => {
    const input = screen.getByRole('button', { name: label }).parentElement?.querySelector('input[type="file"]');
    fireEvent.change(input as HTMLInputElement, { target: { files: [file] } });
  };
  const png = new File(['x'], 'a.png', { type: 'image/png' });

  beforeEach(() => {
    decodeAs = 'ok';
    mockUpload.mockClear();
    mockToastError.mockClear();
    vi.stubGlobal('URL', Object.assign(URL, { createObjectURL: () => 'blob:x', revokeObjectURL: vi.fn() }));
    vi.stubGlobal('Image', FakeImage);
  });

  it('opens on file select instead of uploading', async () => {
    render(<ImageUpload complex={mockComplex} />);
    pick('Portada', png);

    expect(await screen.findByText('Recortá la imagen')).toBeInTheDocument();
    expect(mockUpload).not.toHaveBeenCalled();
  });

  it('does not upload on Cancelar', async () => {
    render(<ImageUpload complex={mockComplex} />);
    pick('Logo', png);

    await userEvent.click(await screen.findByRole('button', { name: 'Cancelar' }));

    expect(mockUpload).not.toHaveBeenCalled();
    expect(screen.queryByText('Recortá la imagen')).not.toBeInTheDocument();
  });

  it('uploads the file with the chosen area on Guardar', async () => {
    render(<ImageUpload complex={mockComplex} />);
    pick('Portada', png);

    await userEvent.click(await screen.findByRole('button', { name: 'Guardar' }));

    expect(mockUpload).toHaveBeenCalledWith({ file: png, type: 'cover', crop: CROP_AREA });
  });

  it('rejects an invalid file without opening the dialog', () => {
    render(<ImageUpload complex={mockComplex} />);
    pick('Logo', new File(['x'], 'a.gif', { type: 'image/gif' }));

    expect(screen.queryByText('Recortá la imagen')).not.toBeInTheDocument();
    expect(mockUpload).not.toHaveBeenCalled();
  });

  it('shows an error and no dialog when the file is not a readable image', async () => {
    decodeAs = 'broken';
    render(<ImageUpload complex={mockComplex} />);
    pick('Portada', png);

    await vi.waitFor(() => {
      expect(mockToastError).toHaveBeenCalledWith('No pudimos abrir la imagen. Probá con otro archivo.');
    });
    expect(screen.queryByText('Recortá la imagen')).not.toBeInTheDocument();
  });

  it('warns that an image smaller than the upload size may look blurry', async () => {
    decodeAs = 'small';
    render(<ImageUpload complex={mockComplex} />);
    pick('Portada', png);

    expect(await screen.findByText(/puede verse borrosa/)).toBeInTheDocument();
  });

  it('does not warn for an image at least as large as the upload size', async () => {
    render(<ImageUpload complex={mockComplex} />);
    pick('Portada', png);

    await screen.findByText('Recortá la imagen');
    expect(screen.queryByText(/puede verse borrosa/)).not.toBeInTheDocument();
  });
});
