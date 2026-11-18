import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
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

  it('opens on Historia and shows both pieces as stories, titled', () => {
    renderKit();
    expect(screen.getByRole('button', { name: 'Historia' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByRole('button', { name: 'Post' })).toHaveAttribute('aria-pressed', 'false');
    expect(screen.getByText('Para historias de Instagram y estados de WhatsApp. 1080 × 1920.')).toBeInTheDocument();
    expect(screen.getByRole('heading', { level: 2, name: 'Link de reservas' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { level: 2, name: 'Cómo reservar' })).toBeInTheDocument();
    expect(screen.getByRole('img', { name: 'Historia: Link de reservas' })).toHaveAttribute(
      'src',
      `${KIT_URL}/reservar-historia.png`,
    );
    expect(screen.getByRole('img', { name: 'Historia: Cómo reservar' })).toHaveAttribute(
      'src',
      `${KIT_URL}/como-reservar-historia.png`,
    );
    expect(screen.getAllByRole('img')).toHaveLength(2);
  });

  it('switches both pieces to posts when Post is chosen', async () => {
    const user = userEvent.setup();
    renderKit();
    await user.click(screen.getByRole('button', { name: 'Post' }));

    expect(screen.getByRole('button', { name: 'Post' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByText('Para el feed de Instagram y Facebook. 1080 × 1440.')).toBeInTheDocument();
    expect(screen.getByRole('img', { name: 'Post: Link de reservas' })).toHaveAttribute(
      'src',
      `${KIT_URL}/reservar-post.png`,
    );
    expect(screen.getByRole('img', { name: 'Post: Cómo reservar' })).toHaveAttribute(
      'src',
      `${KIT_URL}/como-reservar-post.png`,
    );
    expect(screen.getAllByRole('img')).toHaveLength(2);
  });
});

describe('SocialKitPage loading and downloads', () => {
  it('lazy-loads the previews', () => {
    renderKit();
    for (const img of screen.getAllByRole('img')) {
      expect(img).toHaveAttribute('loading', 'lazy');
    }
  });

  it('gives each piece a Descargar link to the attachment variant of the chosen format', async () => {
    const user = userEvent.setup();
    renderKit();
    const hrefs = () => screen.getAllByRole('link', { name: /Descargar/ }).map((link) => link.getAttribute('href'));
    expect(hrefs()).toEqual([
      `${KIT_URL}/reservar-historia.png?descargar=1`,
      `${KIT_URL}/como-reservar-historia.png?descargar=1`,
    ]);
    await user.click(screen.getByRole('button', { name: 'Post' }));
    expect(hrefs()).toEqual([
      `${KIT_URL}/reservar-post.png?descargar=1`,
      `${KIT_URL}/como-reservar-post.png?descargar=1`,
    ]);
  });

  it('renders nothing while there is no selected complex', () => {
    withComplex(null);
    const { container } = render(<SocialKitPage />);
    expect(container).toBeEmptyDOMElement();
  });
});
