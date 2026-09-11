import { Link } from 'react-router-dom';
import { Logo } from '@/shared/components/common/Logo';

export function SidebarBrand() {
  return (
    <Link to="/" className="flex items-center gap-2.5">
      <Logo />
    </Link>
  );
}
