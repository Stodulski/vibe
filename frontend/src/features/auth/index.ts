// Public API of the auth feature.
// Only export what other features or pages actually consume from outside
// this folder — see 06-auth-shared-tooling.md A2/A3.

export { RegisterForm } from './components/RegisterForm';
export { LoginForm } from './components/LoginForm';
export { PersonalInfoForm } from './components/PersonalInfoForm';
export { SecurityForm } from './components/SecurityForm';
export { GoogleCompleteForm } from './components/GoogleCompleteForm';

export { useLogout } from './hooks/useLogout';
export { useAuth } from './hooks/useAuth';
export { useCrossTabLogout } from './hooks/useCrossTabLogout';

export { authApi } from './api/auth.api';

export { forgotPasswordSchema } from './schemas/auth.schema';
export { resetPasswordSchema } from './schemas/auth.schema';
export { verifyEmailSentStateSchema } from './schemas/auth.schema';
export { googleCompleteStateSchema } from './schemas/auth.schema';
export type { ForgotPasswordDto, ResetPasswordDto } from './schemas/auth.schema';
