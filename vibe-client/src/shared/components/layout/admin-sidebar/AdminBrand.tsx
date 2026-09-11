import { Link } from 'react-router-dom';
import { Logo } from '@/shared/components/common/Logo';

interface AdminBrandProps {
  /** Hides the "Admin" label when the masthead's brand cell is icon-only-width (sidebar collapsed). */
  collapsed?: boolean;
}

export function AdminBrand({ collapsed = false }: AdminBrandProps) {
  return (
    <Link to="/admin" className="flex items-center gap-2.5">
      <Logo />
      {!collapsed && <span className="whitespace-nowrap text-xs font-medium text-primary-400">Admin</span>}
    </Link>
  );
}
