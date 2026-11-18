import { describe, it, expect, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { useSelectedComplex } from '@/features/complex';
import { makeComplex } from '@/test/factories';

vi.mock('@/features/complex/hooks/useSelectedComplex', () => ({
  useSelectedComplex: vi.fn(),
}));

import SocialKitPage from './SocialKitPage';

const KIT_URL = 'https://vibe.com.ar/kit/c/los-alamos';

function withComplex(complex: ReturnType<typeof makeComplex> | null) {
  vi.mocked(useSelectedComplex).mockReturnValue({ complex } as unknown as ReturnType<typeof useSelectedComplex>);
}

function renderKit() {
  withComplex(makeComplex({ slug: 'los-alamos' }));
  render(<SocialKitPage />);
}

describe('SocialKitPage', () => {
  it('shows the title and the intro', () => {
    renderKit();
    expect(screen.getByRole('heading', { level: 1, name: 'Kit para redes' })).toBeInTheDocument();
    expect(
      screen.getByText(
        'Descargá estas imágenes y subilas a tus historias y posts. Llevan el link y el QR de tu complejo.',
      ),
    ).toBeInTheDocument();
  });

  it('shows one row per piece with its title, what it is and its post as the thumbnail', () => {
    renderKit();
    const reservar = screen.getByRole('article', { name: 'Link de reservas' });
    expect(
      within(reservar).getByText('El QR y el link para que tus clientes reserven desde el celular.'),
    ).toBeInTheDocument();
    expect(within(reservar).getByRole('img', { name: 'Post: Link de reservas' })).toHaveAttribute(
      'src',
      `${KIT_URL}/reservar-post.png`,
    );
    const como = screen.getByRole('article', { name: 'Cómo reservar' });
    expect(within(como).getByText('Los tres pasos para reservar en tu complejo, con el QR.')).toBeInTheDocument();
    expect(within(como).getByRole('img', { name: 'Post: Cómo reservar' })).toHaveAttribute(
      'src',
      `${KIT_URL}/como-reservar-post.png`,
    );
  });
});

describe('SocialKitPage loading and downloads', () => {
  it('lazy-loads the thumbnails', () => {
    renderKit();
    for (const img of screen.getAllByRole('img')) {
      expect(img).toHaveAttribute('loading', 'lazy');
    }
  });

  it('downloads each piece as a story or a post, from the attachment variant', () => {
    renderKit();
    const reservar = screen.getByRole('article', { name: 'Link de reservas' });
    expect(within(reservar).getByRole('link', { name: 'Descargar historia: Link de reservas' })).toHaveAttribute(
      'href',
      `${KIT_URL}/reservar-historia.png?descargar=1`,
    );
    expect(within(reservar).getByRole('link', { name: 'Descargar post: Link de reservas' })).toHaveAttribute(
      'href',
      `${KIT_URL}/reservar-post.png?descargar=1`,
    );
    const como = screen.getByRole('article', { name: 'Cómo reservar' });
    expect(within(como).getByRole('link', { name: 'Descargar historia: Cómo reservar' })).toHaveAttribute(
      'href',
      `${KIT_URL}/como-reservar-historia.png?descargar=1`,
    );
    expect(within(como).getByRole('link', { name: 'Descargar post: Cómo reservar' })).toHaveAttribute(
      'href',
      `${KIT_URL}/como-reservar-post.png?descargar=1`,
    );
  });

  it('renders nothing while there is no selected complex', () => {
    withComplex(null);
    const { container } = render(<SocialKitPage />);
    expect(container).toBeEmptyDOMElement();
  });
});
