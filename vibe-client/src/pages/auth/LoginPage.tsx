import { LoginForm } from '@/features/auth';
import { AuthSplitLayout } from '@/shared/components/layout/AuthSplitLayout';
import { usePageTitle, useOGTags } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export default function LoginPage() {
  usePageTitle(t.auth.login);
  useOGTags({
    title: t.auth.loginMetaTitle,
    description: t.auth.loginMetaDescription,
  });

  return (
    <AuthSplitLayout>
      <LoginForm />
    </AuthSplitLayout>
  );
}
