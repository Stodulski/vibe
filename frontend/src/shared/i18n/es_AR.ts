import { common } from './es_AR/common';
import { auth } from './es_AR/auth';
import { bookings } from './es_AR/bookings';
import { courts } from './es_AR/courts';
import { clients } from './es_AR/clients';
import { complex } from './es_AR/complex';
import { dashboard } from './es_AR/dashboard';
import { serviceFee } from './es_AR/serviceFee';
import { mp } from './es_AR/mp';
import { publicBooking } from './es_AR/publicBooking';
import { placeholders } from './es_AR/placeholders';
import { validation } from './es_AR/validation';
import { navigation } from './es_AR/navigation';
import { profile } from './es_AR/profile';
import { layout } from './es_AR/layout';
import { admin } from './es_AR/admin';
import { reports } from './es_AR/reports';
import { cash } from './es_AR/cash';

export const ES_AR = {
  common,
  auth,
  bookings,
  courts,
  clients,
  complex,
  dashboard,
  serviceFee,
  mp,
  publicBooking,
  placeholders,
  validation,
  navigation,
  profile,
  layout,
  admin,
  reports,
  cash,
} as const;
