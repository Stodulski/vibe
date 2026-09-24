/**
 * Tutoriales = un video, una pantalla, una URL.
 *
 * No son guías. Una guía contesta algo que se busca ANTES de saber que Vibe
 * existe, y cita fuentes externas. Un tutorial muestra una pantalla del panel
 * funcionando, y su fuente es el producto. Van separados para que ninguna de
 * las dos cosas diluya a la otra: mezcladas, las guías dejan de leerse como
 * contenido editorial y los tutoriales dejan de encontrarse.
 *
 * `respuesta` va primero y por el mismo motivo que en las guías: un motor
 * generativo cita unidades que puede extraer enteras. El video no se puede
 * extraer, el párrafo sí, así que el párrafo tiene que decir lo mismo que el
 * video sin depender de él. Una página cuyo contenido está solo en el video es
 * una página vacía para todo el que no lo reproduce, incluido Google.
 *
 * `slug` es también el nombre del archivo en el CDN. Una sola cosa para
 * recordar, y el día que se regraba un video no hay que buscar a qué URL
 * correspondía.
 */
import type { Bloque } from './guias.ts';
import { fechaDeContenido, guardarFechas } from './fecha-de-contenido.ts';

export const CDN = 'https://cdn.vibe.com.ar/tutorials';

/** Alto y ancho reales de la grabación; van en el markup para que no salte el layout. */
export const VIDEO_ANCHO = 1920;
export const VIDEO_ALTO = 1080;

export type TutorialFaq = { question: string; answer: string };

export type Tutorial = {
  /** También el nombre del .mp4 y del .webp en el CDN. */
  slug: string;
  /** H1 de la página. */
  title: string;
  seoTitle: string;
  metaDescription: string;
  /** Lo que muestra el video, en un párrafo, sin depender de que se reproduzca. */
  respuesta: string;
  /** Bajada del índice. */
  excerpt: string;
  /** Segundos exactos de la grabación, para el VideoObject y el video-sitemap. */
  duracion: number;
  /**
   * El mismo video publicado en YouTube. Va en `sameAs` del VideoObject, no en
   * `contentUrl`: la página reproduce el archivo propio, y esto le dice a Google
   * que la copia de YouTube es el mismo contenido y no otro que compite.
   */
  youtube: string;
  datePublished: string;
  /* NO se escribe: sale del hash del contenido, igual que en las guías. */
  dateModified: string;
  bloques: Bloque[];
  faq: TutorialFaq[];
  /** Otros tutoriales que siguen naturalmente a este. */
  relacionados: string[];
  /** La guía que desarrolla el "qué hacer" de lo que este tutorial muestra. */
  guia?: { slug: string; texto: string };
};

const contenido: Omit<Tutorial, 'dateModified'>[] = [
  {
    slug: 'grilla-de-reservas',
    title: 'La grilla de reservas: todo el día en una pantalla',
    seoTitle: 'Grilla de reservas para un complejo deportivo | Vibe',
    metaDescription:
      'Todas tus canchas del día en una pantalla, cada turno con su seña y lo que falta cobrar, y '
      + 'cómo bloquear un horario desde el panel de Vibe.',
    excerpt: 'Todas tus canchas del día en una pantalla, y cada turno con su cliente y su pago.',
    duracion: 50,
    youtube: 'S7PUkxm8lu0',
    datePublished: '2026-09-16',
    respuesta:
      'La grilla pone todas las canchas del complejo una al lado de la otra, con cada turno en su '
      + 'horario. Tocás uno y ves quién reservó, a qué hora, cuánto pagó de seña y cuánto falta '
      + 'cobrar. Con las flechas te movés día por día, y el botón Hoy te trae de vuelta. Si una '
      + 'cancha queda fuera de servicio, bloqueás el horario desde la misma pantalla y ese turno deja '
      + 'de ofrecerse en tu página de reservas.',
    bloques: [
      {
        tipo: 'parrafos',
        heading: 'Chau cuaderno de turnos',
        parrafos: [
          'Te llaman: "¿hay lugar el jueves a las nueve?". La respuesta está acá, sin hojear nada: qué '
            + 'está tomado y qué queda libre, en la cancha que te piden.',
          'Y el cobro también está a la vista. Un turno con la seña pagada y uno reservado por '
            + 'teléfono se distinguen en el detalle, así que no cruzás la agenda con los pagos para '
            + 'saber quién te debe.',
        ],
      },
      {
        tipo: 'lista',
        heading: 'Qué hacés desde la grilla',
        items: [
          'Ver el día completo, con todas las canchas en paralelo',
          'Abrir un turno y ver el cliente, el horario y el estado del pago',
          'Moverte a otro día con las flechas, o volver a hoy',
          'Bloquear un horario para que no se pueda reservar online',
          'Cargar a mano la reserva del que te llama por teléfono',
        ],
      },
    ],
    faq: [
      {
        question: '¿Qué pasa cuando bloqueo un horario?',
        answer:
          'Desaparece de tu página de reservas y nadie lo puede tomar. Sirve para mantenimiento, un '
          + 'torneo o una cancha inundada. El bloqueo se saca igual que se pone.',
      },
      {
        question: '¿Puedo cargar una reserva que me pidieron por teléfono?',
        answer:
          'Sí. Ocupa el turno igual que una online y queda en el historial del cliente. No pasa por '
          + 'MercadoPago: si te pagan algo, lo registrás en el momento en efectivo, transferencia, '
          + 'débito, crédito o QR.',
      },
      {
        question: '¿Y si dos personas quieren el mismo turno a la vez?',
        answer:
          'Entra una sola. Antes de confirmar, el sistema chequea que el horario siga libre: si las dos '
          + 'llegan casi juntas, la primera queda y la segunda se rechaza. Nunca hay una cancha '
          + 'reservada dos veces.',
      },
      {
        question: '¿Puedo cambiarle la cancha o el horario a una reserva?',
        answer:
          'No desde la misma reserva. Lo que cambiás es el estado (confirmada, cancelada, completada, '
          + 'ausente) y las notas. Para moverla, la cancelás y cargás una nueva.',
      },
      {
        question: 'Si entra una reserva con la grilla abierta, ¿tengo que recargar?',
        answer:
          'No. El panel se actualiza en tiempo real: la reserva nueva o la cancelación aparece sola, '
          + 'sin tocar nada.',
      },
    ],
    relacionados: ['precios-por-horario', 'base-de-clientes'],
  },
  {
    slug: 'precios-por-horario',
    title: 'Precios por horario: cobrá la noche más cara',
    seoTitle: 'Precios por horario y tarifa nocturna en tu cancha | Vibe',
    metaDescription:
      'Cargá un precio base por día y una franja nocturna más cara. Cómo cobra Vibe un turno que '
      + 'cruza de una tarifa a la otra, por media hora.',
    excerpt: 'Un precio base por día y, arriba, las franjas que cobrás distinto.',
    duracion: 50,
    youtube: 'Zn91DlIh2rs',
    datePublished: '2026-09-16',
    respuesta:
      'Cada cancha arranca con un precio base para todo el día. Sobre ese precio abrís cualquier día '
      + 'de la semana y le sumás una franja con otro valor: así se carga la tarifa nocturna. El cobro '
      + 'no redondea a la franja donde empieza el turno: cada media hora se cobra a su propia tarifa. '
      + 'Un turno de una hora que arranca a las 17:30, con la noche empezando a las 18:00, se cobra '
      + 'media hora a precio de día y media hora a precio de noche.',
    bloques: [
      {
        tipo: 'parrafos',
        heading: 'Por qué se cobra media hora por media hora',
        parrafos: [
          'Si cobrás la noche más cara, siempre vas a tener turnos que cruzan el límite. Si el sistema '
            + 'cobrara todo el turno a la tarifa de la hora de inicio, reservar a las 17:30 saldría más '
            + 'barato que a las 18:00 por la misma hora de cancha. Y ese agujero lo encuentra el primer '
            + 'cliente que hace la cuenta.',
          'Cobrar cada media hora a su tarifa cierra el agujero sin que tengas que pensarlo. Cargás el '
            + 'precio de día y el de noche, y cualquier duración con cualquier horario de inicio da el '
            + 'número justo.',
        ],
      },
      {
        tipo: 'lista',
        heading: 'Qué más editás desde la misma pantalla',
        items: [
          'El nombre de la cancha y el deporte que se juega',
          'Si es techada o descubierta',
          'La descripción que lee el cliente cuando elige dónde jugar',
          'Si la cancha está activa o deja de aparecer en las reservas',
        ],
      },
    ],
    faq: [
      {
        question: '¿Puedo tener un precio distinto para cada día de la semana?',
        answer:
          'Sí. El precio base se carga por día, así que el sábado puede valer distinto que el martes '
          + 'sin tocar el resto. Y sobre cada día sumás franjas con su propio valor.',
      },
      {
        question: '¿Cuántas franjas puedo cargar por día?',
        answer:
          'Las que necesites. Lo habitual es una, la nocturna, pero si cobrás distinto al mediodía '
          + 'cargás esa también. Lo único que no se puede es superponer dos franjas en el mismo '
          + 'horario.',
      },
      {
        question: '¿Qué pasa si dos personas editan el precio de la misma cancha a la vez?',
        answer:
          'El sistema lo detecta: si alguien ya guardó un cambio en esa cancha, tu edición se rechaza '
          + 'con un aviso de que hay una versión más nueva. La volvés a cargar sobre los datos '
          + 'actualizados, sin pisar nada.',
      },
      {
        question: '¿Puedo eliminar una cancha que tiene reservas futuras?',
        answer:
          'No. Mientras tenga una reserva confirmada o pendiente que todavía no pasó, no se puede '
          + 'borrar. Esperás a que esas reservas se completen o las cancelás, y después la eliminás.',
      },
      {
        question: '¿Una cancha techada puede costar distinto que una descubierta a la misma hora?',
        answer:
          'Sí. El precio va por cancha: a las nueve de la noche la techada puede salir más y la '
          + 'descubierta menos, y el que reserva ve el precio de cada una antes de elegir.',
      },
    ],
    relacionados: ['grilla-de-reservas', 'reportes-de-facturacion'],
    guia: {
      slug: 'cuanto-cuesta-un-sistema-de-reservas-para-canchas',
      texto: 'Cuánto cuesta un sistema de reservas para canchas',
    },
  },
  {
    slug: 'base-de-clientes',
    title: 'Tu base de clientes, armada sola',
    seoTitle: 'Base de clientes para tu complejo deportivo | Vibe',
    metaDescription:
      'Cada persona que reserva queda guardada con su teléfono, su historial y su asistencia. Sin '
      + 'fichas a mano ni planillas: la base se llena sola.',
    excerpt: 'Cada persona que reserva queda guardada, con su historial y su asistencia.',
    duracion: 54,
    youtube: 'c5QQGWhU3Es',
    datePublished: '2026-09-16',
    respuesta:
      'La base de clientes no se carga: se llena sola con cada reserva. De cada persona queda el '
      + 'teléfono, cuántas veces reservó, cuántas faltó y su porcentaje de asistencia. El buscador '
      + 'encuentra por nombre o por parte del teléfono, y la ficha de cada cliente muestra sus '
      + 'últimas reservas con la cancha, la fecha y si se presentó.',
    bloques: [
      {
        tipo: 'parrafos',
        heading: 'Para qué te sirve la asistencia',
        parrafos: [
          'Es el dato que decide a quién le pedís seña y a quién no. El que viene hace dos años y '
            + 'nunca faltó no necesita el mismo trato que el que reservó tres veces y vino una. Sin el '
            + 'dato, esa decisión se toma de memoria, y la memoria falla justo con el que menos '
            + 'conviene.',
          'También sirve al revés: saber a quién avisarle primero cuando se libera un buen horario, o '
            + 'cuando querés llenar un martes flojo.',
        ],
      },
    ],
    faq: [
      {
        question: '¿Cómo marco que un cliente no se presentó?',
        answer:
          'Desde el turno, en la grilla. Esa marca alimenta el porcentaje de asistencia, así que el '
          + 'dato vale tanto como la constancia con que la cargás.',
      },
      {
        question: '¿Le tengo que pedir el mail al cliente?',
        answer:
          'No es obligatorio: con el nombre y el teléfono alcanza para reservar. Si deja el mail, le '
          + 'llegan ahí la confirmación y el recordatorio del turno.',
      },
      {
        question: '¿Puedo editar a mano los datos de un cliente?',
        answer:
          'Solo las notas y el bloqueo. El nombre, el teléfono y el mail salen de sus reservas y no se '
          + 'escriben desde la ficha. Lo que sí sumás son notas tuyas, por ejemplo que prefiere cierta '
          + 'cancha o cierto horario.',
      },
      {
        question: '¿Puedo bloquear a un cliente para que no vuelva a reservar?',
        answer:
          'Sí. Cada cliente tiene un estado de bloqueado que activás desde su ficha, y una vez '
          + 'bloqueado no puede volver a reservar online.',
      },
      {
        question: '¿La asistencia cuenta las reservas telefónicas o solo las pagadas online?',
        answer:
          'Cuenta todas. El porcentaje sale de las reservas completadas y las ausencias reales del '
          + 'cliente, sea que haya pagado online o que la hayas cargado vos por teléfono.',
      },
    ],
    relacionados: ['grilla-de-reservas', 'panel-de-control'],
  },
  {
    slug: 'panel-de-control',
    title: 'El panel: tu complejo de un vistazo',
    seoTitle: 'Panel de control para un complejo deportivo | Vibe',
    metaDescription:
      'Lo que facturaste hoy, las reservas, la ocupación y el mapa de horas muertas: qué ves apenas '
      + 'entrás al panel de Vibe, con la caja incluida.',
    excerpt: 'Cuánto facturaste, cuántas reservas tenés y qué canchas están vacías, apenas entrás.',
    duracion: 54,
    youtube: 'EfSBAkqQHCk',
    datePublished: '2026-09-16',
    respuesta:
      'Entrás al panel y ves lo que facturaste en el día, cuántas reservas tenés y el porcentaje de '
      + 'ocupación de las canchas. Arriba está tu link público de reservas, listo para copiar. Más '
      + 'abajo, el gráfico de ingresos, que mirás por semana o por mes, los clientes que más reservan '
      + 'y el mapa de ocupación por horario, que es donde aparecen las horas que quedan vacías.',
    bloques: [
      {
        tipo: 'parrafos',
        heading: 'El mapa de ocupación es el que más vas a mirar',
        parrafos: [
          'Lo que facturaste hoy te dice cómo te fue. El mapa de ocupación te dice dónde está la plata '
            + 'que todavía no cobraste: qué horarios quedan vacíos semana tras semana, que no es lo '
            + 'mismo que un día flojo suelto.',
          'Un complejo lleno los viernes a la noche y vacío los martes a las tres de la tarde no tiene '
            + 'un problema de demanda: tiene un problema de franja. El mapa te lo muestra sin llevar la '
            + 'cuenta a mano.',
        ],
      },
    ],
    faq: [
      {
        question: '¿El link de reservas es siempre el mismo?',
        answer:
          'Sí, no cambia. Dejalo fijo en la bio de Instagram y en el estado de WhatsApp: una vez que '
          + 'está publicado en varios lados, no querés que se rompa.',
      },
      {
        question: '¿Puedo ver los ingresos de un mes anterior?',
        answer:
          'El gráfico del panel muestra la tendencia reciente. Para el detalle de un mes cerrado, con '
          + 'el desglose por método de pago y por cancha, está la pantalla de reportes.',
      },
      {
        question: '¿Cuántos clientes muestra el ranking de los que más reservan?',
        answer:
          'Los diez que más reservaron en los últimos 30 días. Si tenés menos de diez clientes activos '
          + 'en ese período, los lugares que sobran quedan vacíos en vez de llenarse con cualquiera.',
      },
      {
        question: '¿La vista semanal muestra los últimos 7 días corridos?',
        answer:
          'Sí. La semanal suma los últimos 7 días contando hoy, y la mensual los últimos 30: las dos '
          + 'son una ventana móvil, no el calendario del mes.',
      },
      {
        question: '¿La ocupación cuenta las canchas que desactivé?',
        answer:
          'No. Se calcula solo sobre las canchas activas, así que desactivar una en mantenimiento no '
          + 'te baja el porcentaje por horas que de entrada no estaban a la venta.',
      },
      {
        question: '¿El panel muestra la caja?',
        answer:
          'Sí. Si la caja está abierta, ves el efectivo esperado y desde cuándo está abierta, con un '
          + 'acceso directo; si está cerrada, te ofrece abrirla. Y si algún producto quedó con stock '
          + 'bajo, aparece un aviso con acceso a productos.',
      },
    ],
    relacionados: ['reportes-de-facturacion', 'base-de-clientes'],
    guia: {
      slug: 'como-llenar-los-horarios-vacios-de-un-complejo',
      texto: 'Cómo llenar los horarios vacíos de un complejo',
    },
  },
  {
    slug: 'reportes-de-facturacion',
    title: 'El reporte del mes, listo para el contador',
    seoTitle: 'Reporte de facturación mensual de tu complejo | Vibe',
    metaDescription:
      'El mes cerrado solo: comparado con el anterior, abierto por método de pago y por cancha, y '
      + 'en Excel para el contador, con la caja incluida.',
    excerpt: 'El cierre del mes armado solo, comparado con el anterior y listo para el contador.',
    duracion: 44,
    youtube: 'n4h7--y2sd4',
    datePublished: '2026-09-16',
    respuesta:
      'El reporte se arma mes a mes y se compara siempre contra el mes anterior. Te muestra cuánto '
      + 'entró por cada método de pago —MercadoPago y los que registrás en el mostrador—, con los '
      + 'reembolsos ya descontados, así que el número que ves es el neto. También se abre cancha por '
      + 'cancha, para ver cuáles facturan y cuáles casi no se usan. Todo eso se baja en Excel.',
    bloques: [
      {
        tipo: 'parrafos',
        heading: 'El desglose por cancha te dice qué arreglar',
        parrafos: [
          'El total del mes no te dice qué hacer. El desglose por cancha, sí: una cancha que factura la '
            + 'mitad que las demás está marcando un precio mal puesto, un horario mal cargado o algo '
            + 'para arreglar en la cancha misma.',
          'Cruzado con el mapa de ocupación del panel, separa los dos casos que se confunden siempre: '
            + 'la cancha que se usa poco y la que se usa mucho pero barata.',
        ],
      },
    ],
    faq: [
      {
        question: '¿Qué formato tiene la exportación?',
        answer:
          'Un Excel con dos hojas: los pagos del período y un resumen por método y por cancha. Si ese '
          + 'mes usaste la caja, el resumen suma una sección con las ventas por método y los '
          + 'movimientos por categoría. Se abre igual en Google Sheets.',
      },
      {
        question: '¿Los reembolsos ya están descontados?',
        answer:
          'Sí. El neto que muestra el reporte ya tiene los reembolsos restados: no hacés la cuenta '
          + 'aparte.',
      },
      {
        question: '¿Veo cuánto facturó cada cancha por separado?',
        answer:
          'Sí. Además del total, el reporte abre un desglose por cancha con lo cobrado, lo reembolsado '
          + 'y el neto de cada una, para ver cuáles rinden y cuáles no.',
      },
      {
        question: '¿Hay un límite de pagos por exportación?',
        answer:
          'Sí, hay un tope de filas. Si un mes tiene más pagos que ese tope, la exportación se '
          + 'rechaza: achicás el rango de fechas o la pedís en partes.',
      },
      {
        question: '¿Puedo exportar un mes que ya pasó?',
        answer:
          'Sí. La exportación pide el mes y el año como cualquier filtro, así que sirve igual para el '
          + 'mes en curso que para uno cerrado hace tiempo.',
      },
    ],
    relacionados: ['panel-de-control', 'precios-por-horario'],
    guia: {
      slug: 'cuanto-cobra-mercadopago-por-una-sena',
      texto: 'Cuánto cobra MercadoPago por cobrar una seña',
    },
  },
  {
    slug: 'configuracion-del-complejo',
    title: 'Configurá tu complejo: horarios y seña',
    seoTitle: 'Cómo configurar horarios y seña de tu complejo | Vibe',
    metaDescription:
      'Los datos del complejo, hasta qué hora se reserva cada día, cuánta seña pedís y el estado de '
      + 'MercadoPago: todo se define en una sola pantalla.',
    excerpt: 'Los datos del complejo, los horarios de atención y cuánta seña pedís.',
    duracion: 38,
    youtube: '05OH3m9ZJxM',
    datePublished: '2026-09-16',
    respuesta:
      'En Configuración cargás el nombre, la dirección y el teléfono del complejo, y el porcentaje '
      + 'de seña que se pide al reservar online. Los horarios de atención se definen día por día: '
      + 'fuera de esa franja, tu página no ofrece turnos. En la pestaña de cobros ves si MercadoPago '
      + 'está conectado y cuánto cobra según el plazo de acreditación que elegiste.',
    bloques: [
      {
        tipo: 'parrafos',
        heading: 'La seña es la decisión que más pesa',
        parrafos: [
          'El porcentaje de seña es lo único de esta pantalla que elegís vos y no copiás de un dato '
            + 'que ya existe. Una seña baja hace fácil reservar, y también faltar. Una seña alta filtra '
            + 'al que no va a ir, y también a alguno que sí iba.',
          'Revisalo contra el porcentaje de ausencias que te muestra la base de clientes. No lo dejes '
            + 'donde quedó el primer día.',
        ],
      },
    ],
    faq: [
      {
        question: '¿Qué pasa si no conecto MercadoPago?',
        answer:
          'El complejo funciona igual y podés cargar reservas, pero la seña no se cobra online. '
          + 'Conectarlo es lo que hace que el cliente pague al reservar y que la plata entre directo a '
          + 'tu cuenta.',
      },
      {
        question: '¿Puedo tener otro horario los fines de semana?',
        answer:
          'Sí, los horarios se definen día por día. El sábado puede abrir y cerrar a otra hora que el '
          + 'lunes, y tu página respeta cada uno.',
      },
      {
        question: '¿Qué rango tiene la ventana de cancelación?',
        answer:
          'De 1 a 168 horas, o sea hasta una semana antes del turno. No se puede dejar en cero: sería '
          + 'reembolsar siempre, sin ninguna ventana real.',
      },
      {
        question: '¿Puedo pedir el 100% de seña?',
        answer:
          'Sí, cualquier valor entre 0% y 100%. Si querés cobrar la cancha completa por adelantado, '
          + 'pedís el 100% y listo.',
      },
      {
        question: 'Si cambio el porcentaje de seña, ¿cambian las reservas que ya están cargadas?',
        answer:
          'No. Cada reserva guarda la seña que correspondía el día que se cargó, así que el cambio '
          + 'solo toca las reservas nuevas.',
      },
    ],
    relacionados: ['precios-por-horario'],
    guia: {
      slug: 'cuanto-cobra-mercadopago-por-una-sena',
      texto: 'Cuánto cobra MercadoPago por cobrar una seña',
    },
  },
  {
    slug: 'abrir-y-cerrar-la-caja',
    title: 'Abrí la caja del turno, cerrala con la cuenta hecha',
    seoTitle: 'Cómo abrir y cerrar la caja de tu complejo | Vibe',
    metaDescription:
      'Abrís la caja con el efectivo del cajón. Al cerrar, cargás lo que contaste y ves si sobra o falta, '
      + 'sin sacar la cuenta a mano.',
    excerpt: 'Abrís con el efectivo del cajón; al cerrar, Vibe ya hizo la cuenta.',
    duracion: 54,
    youtube: '',
    datePublished: '2026-09-24',
    respuesta:
      'Abrís la caja cargando el efectivo con el que arrancás el turno. Mientras está abierta, ves el '
      + 'efectivo esperado en cualquier momento: lo que debería haber en el cajón según lo que entró y '
      + 'salió. Al cerrar, contás el efectivo real y lo cargás; Vibe lo compara contra lo esperado y te '
      + 'muestra la diferencia, sobrante o faltante. Una caja cerrada no se puede reabrir ni editar: es el '
      + 'corte del turno.',
    bloques: [
      {
        tipo: 'parrafos',
        heading: 'El cuaderno que a la noche no cerraba',
        parrafos: [
          'Contar el cajón a mano al final del día y no saber si lo que sobra es un error tuyo o de la '
            + 'persona que atendió antes es una pelea que se repite todas las noches.',
          'Acá la cuenta la hace Vibe: cada cobro en efectivo, cada venta y cada ingreso o egreso que '
            + 'cargás durante el turno ya está sumado al efectivo esperado. Vos solo contás el cajón y '
            + 'comparás.',
        ],
      },
      {
        tipo: 'lista',
        heading: 'Qué pasa entre que abrís y cerrás',
        items: [
          'Cargás el efectivo inicial una sola vez, al abrir',
          'Cada cobro en efectivo de una reserva se suma solo al esperado',
          'Cada venta del mostrador también entra sola, con su método',
          'Al cerrar, contás el cajón y Vibe te muestra sobrante o faltante',
          'La caja cerrada queda en el historial, con lo esperado y lo contado',
        ],
      },
    ],
    faq: [
      {
        question: '¿Puedo tener dos cajas abiertas a la vez?',
        answer:
          'No. Un complejo tiene una sola caja abierta por vez. Cerrás la de un turno antes de abrir la '
          + 'del siguiente.',
      },
      {
        question: '¿Qué pasa si me equivoco al contar el efectivo?',
        answer:
          'Una vez que cerraste la caja con ese conteo, no se puede reabrir ni editar. La diferencia que '
          + 'haya quedado registrada.',
      },
      {
        question: '¿Los cobros con tarjeta o QR también entran al efectivo esperado?',
        answer:
          'No, solo el efectivo. Los demás métodos se muestran a modo informativo en el resumen del '
          + 'turno, pero no forman parte de lo que contás en el cajón.',
      },
      {
        question: '¿Necesito la caja abierta para cobrar una reserva?',
        answer:
          'No para cualquier cobro, pero sí para que ese cobro en efectivo entre al esperado, y para '
          + 'vender productos o reponer stock, que sí necesitan un turno abierto.',
      },
      {
        question: '¿Puedo ver las cajas de turnos anteriores?',
        answer:
          'Sí. El historial guarda cada cierre, con la fecha, el efectivo esperado y el contado de ese '
          + 'turno.',
      },
    ],
    relacionados: ['ingresos-egresos-y-anulaciones', 'panel-de-control'],
  },
  {
    slug: 'ingresos-egresos-y-anulaciones',
    title: 'Ingresos, egresos y cómo anular un movimiento',
    seoTitle: 'Ingresos y egresos de caja en tu complejo | Vibe',
    metaDescription:
      'Cargá lo que entra o sale de la caja por categoría, desde clases hasta el alquiler del local, y '
      + 'anulá un movimiento sin perder el historial.',
    excerpt: 'Todo lo que entra o sale de la caja, por categoría, con su anulación si hace falta.',
    duracion: 54,
    youtube: '',
    datePublished: '2026-09-24',
    respuesta:
      'Con la caja abierta, cargás un ingreso o un egreso eligiendo su categoría —clases, torneos, '
      + 'alquiler del local, sueldos, insumos, entre otras—, el método y el monto. Cada uno queda en el '
      + 'historial del turno. Si cargaste algo mal, no lo borrás: lo anulás, y Vibe registra un movimiento '
      + 'contrario por el mismo monto, así el historial completo queda a la vista.',
    bloques: [
      {
        tipo: 'parrafos',
        heading: 'Categorías, no un cajón de "otros"',
        parrafos: [
          'Anotar todo como "varios" en un cuaderno sirve el mismo día y no sirve nunca más: a fin de '
            + 'mes no hay forma de saber cuánto entró por clases o cuánto salió en sueldos.',
          'Cada movimiento se carga con su categoría propia, así que el desglose por categoría de la caja '
            + '—y del reporte del mes— sale solo, sin tener que releer cada nota.',
        ],
      },
      {
        tipo: 'lista',
        heading: 'Categorías disponibles',
        items: [
          'Ingresos: clases, torneos, eventos, cuotas y abonos, publicidad y sponsors, aporte o cambio, y otros ingresos',
          'Egresos: insumos, sueldos, servicios, mantenimiento, limpieza, retiro, alquiler del local, impuestos, honorarios, marketing, comisiones bancarias, y otros egresos',
        ],
      },
    ],
    faq: [
      {
        question: '¿Qué pasa cuando anulo un movimiento?',
        answer:
          'Se registra un movimiento contrario por el mismo monto, no se borra nada. El original y su '
          + 'anulación quedan los dos en el historial.',
      },
      {
        question: '¿Puedo anular una venta o un cobro de reserva desde acá?',
        answer:
          'No de la misma forma: una venta se anula desde Vender, y devuelve el stock además del ingreso. '
          + 'Este botón es para los movimientos que cargaste a mano.',
      },
      {
        question: '¿Necesito la caja abierta para cargar un ingreso o un egreso?',
        answer:
          'Sí. Los movimientos manuales pertenecen a un turno de caja abierto, igual que las ventas.',
      },
      {
        question: '¿Puedo agregar una nota a un movimiento?',
        answer:
          'Sí, es opcional. Sirve para aclarar algo que la categoría sola no cuenta, por ejemplo a quién '
          + 'se le pagó un servicio.',
      },
      {
        question: '¿Los ingresos y egresos manuales entran al efectivo esperado de la caja?',
        answer:
          'Los que registrás en efectivo, sí. Los que cargás con otro método se muestran igual en el '
          + 'historial, pero no suman al efectivo del cajón.',
      },
    ],
    relacionados: ['abrir-y-cerrar-la-caja', 'reportes-de-facturacion'],
  },
  {
    slug: 'vender-desde-el-mostrador',
    title: 'Vender desde el mostrador, como una caja registradora',
    seoTitle: 'Vender productos desde el mostrador | Vibe',
    metaDescription:
      'Tocás un producto, se suma al carrito, elegís el método y cobrás. La venta descuenta el stock y '
      + 'entra a la caja del turno sola.',
    excerpt: 'Tocás un producto, elegís el método, cobrás. El stock se descuenta solo.',
    duracion: 44,
    youtube: '',
    datePublished: '2026-09-24',
    respuesta:
      'La pantalla de vender muestra tu catálogo en tarjetas: tocás un producto y se suma una unidad al '
      + 'carrito. Sumás o restás desde el carrito mismo, elegís el método de cobro y cobrás. La venta '
      + 'descuenta el stock de cada producto y registra un ingreso en la caja del turno, en el mismo paso. '
      + 'Vender por encima del stock disponible está permitido, con un aviso.',
    bloques: [
      {
        tipo: 'parrafos',
        heading: 'El sábado sin gaseosas, resuelto antes de que pase',
        parrafos: [
          'Vender en el mostrador a mano —anotar en un papel, descontar el stock después, sumar la caja '
            + 'al final— es donde más se pierde: un producto que se vendió y nunca se cargó, o una caja '
            + 'que no cierra porque faltó anotar una venta.',
          'Acá la venta hace las tres cosas en el mismo toque: cobra, descuenta stock y entra a la caja. '
            + 'No hay un paso aparte que te puedas olvidar.',
        ],
      },
      {
        tipo: 'lista',
        heading: 'Qué hacés desde Vender',
        items: [
          'Tocar un producto para sumarlo al carrito',
          'Sumar o restar unidades, o quitar una línea entera',
          'Elegir el método: efectivo, transferencia, débito, crédito o QR',
          'Cobrar y ver la confirmación con el total',
          'Anular una venta ya cobrada, si hace falta',
        ],
      },
    ],
    faq: [
      {
        question: '¿Puedo vender un producto sin stock?',
        answer:
          'Sí, queda permitido igual, pero el carrito te avisa que no hay stock suficiente antes de '
          + 'cobrar.',
      },
      {
        question: '¿Qué pasa si anulo una venta?',
        answer:
          'El stock vendido se restaura y el ingreso de esa venta se anula. Queda marcada como anulada en '
          + 'el historial del turno, no desaparece.',
      },
      {
        question: '¿Necesito la caja abierta para vender?',
        answer:
          'Sí. Vender requiere un turno de caja abierto, porque cada venta registra su ingreso ahí.',
      },
      {
        question: '¿Puedo cobrar una venta con MercadoPago?',
        answer:
          'No desde acá: Vibe registra el cobro en efectivo, transferencia, débito, crédito o QR que vos '
          + 'ya recibiste en el mostrador, no procesa el pago con tarjeta.',
      },
      {
        question: '¿Puedo buscar un producto por categoría?',
        answer:
          'Sí, el catálogo se filtra por categoría o por nombre, para encontrar rápido lo que buscás.',
      },
    ],
    relacionados: ['productos-y-stock', 'abrir-y-cerrar-la-caja'],
  },
  {
    slug: 'productos-y-stock',
    title: 'Tu catálogo, con stock y sin sorpresas',
    seoTitle: 'Productos y control de stock en tu complejo | Vibe',
    metaDescription:
      'Cargá cada producto con su precio y, si querés, su stock. Un aviso solo cuando queda poco, y la '
      + 'reposición entra a la caja como cualquier gasto.',
    excerpt: 'Cada producto con su precio y su stock, con aviso cuando queda poco.',
    duracion: 42,
    youtube: '',
    datePublished: '2026-09-24',
    respuesta:
      'Cada producto se carga con su nombre, categoría y precio. El control de stock es opcional: un '
      + 'servicio como el alquiler de una paleta no lo necesita, y queda marcado sin control de stock. '
      + 'A los que sí lo controlás les definís un aviso de stock bajo: cuando baja de ese número, aparece '
      + 'marcado en el catálogo y en el panel de control. Reponer stock carga la cantidad y el costo '
      + 'total, y ese gasto entra a la caja del turno abierto.',
    bloques: [
      {
        tipo: 'parrafos',
        heading: 'El stock que se entera solo, no cuando ya faltó',
        parrafos: [
          'Enterarte que no quedan pelotas cuando un cliente ya las está pidiendo es tarde. El aviso de '
            + 'stock bajo lo dice antes, mientras todavía hay margen para reponer.',
          'Y como la reposición se carga con su costo, ese gasto queda en la caja del turno igual que '
            + 'cualquier otro egreso: no hace falta anotarlo dos veces.',
        ],
      },
      {
        tipo: 'lista',
        heading: 'Qué hacés desde Productos',
        items: [
          'Crear un producto con su precio, categoría y control de stock opcional',
          'Definir a partir de qué cantidad avisa stock bajo',
          'Reponer stock cargando cantidad y costo total',
          'Ajustar el stock a mano tras un conteo físico o una rotura',
          'Desactivar un producto que dejaste de vender, sin borrar su historial',
        ],
      },
    ],
    faq: [
      {
        question: '¿Todos los productos necesitan control de stock?',
        answer:
          'No. Un servicio como el alquiler de una paleta puede quedar sin control de stock, y se vende '
          + 'igual desde Vender.',
      },
      {
        question: '¿Necesito la caja abierta para reponer stock?',
        answer:
          'Sí, porque el costo de la reposición sale de la caja y se registra como un egreso del turno '
          + 'abierto.',
      },
      {
        question: '¿Puedo sacar el control de stock a un producto que ya lo tenía?',
        answer:
          'Solo si su stock está en cero. Si todavía tiene stock cargado, primero lo ajustás a cero y '
          + 'después lo desactivás.',
      },
      {
        question: '¿Qué diferencia hay entre reponer y ajustar?',
        answer:
          'Reponer es una compra: suma stock y su costo entra a la caja. Ajustar corrige el número sin '
          + 'plata de por medio, por ejemplo tras un conteo físico o una rotura.',
      },
      {
        question: '¿Puedo desactivar un producto sin borrar sus ventas anteriores?',
        answer:
          'Sí. Desactivarlo lo saca de Vender, pero su historial de stock y sus ventas ya registradas '
          + 'quedan igual.',
      },
    ],
    relacionados: ['vender-desde-el-mostrador', 'panel-de-control'],
  },
  {
    slug: 'cobrar-una-reserva-en-el-mostrador',
    title: 'Cobrar una reserva en el mostrador',
    seoTitle: 'Cómo cobrar una reserva en el mostrador | Vibe',
    metaDescription:
      'Un turno confirmado sin cobrar todavía se cobra igual desde el panel: elegís el método real y '
      + 'queda registrado, sin pasar por MercadoPago.',
    excerpt: 'Un turno sin cobrar se cobra igual desde el panel, con el método que uses en el momento.',
    duracion: 34,
    youtube: '',
    datePublished: '2026-09-24',
    respuesta:
      'No todos los clientes pagan la seña online: un turno cargado por teléfono, o uno al que todavía le '
      + 'falta el resto después de la seña, se cobra desde el mismo detalle de la reserva. Tocás Confirmar '
      + 'pago, elegís el método real —efectivo, transferencia, débito, crédito o QR— y confirmás. El turno '
      + 'pasa a cobrado sin que MercadoPago intervenga en ningún momento.',
    bloques: [
      {
        tipo: 'parrafos',
        heading: 'Lo que no pasó por MercadoPago, igual queda registrado',
        parrafos: [
          'La reserva que te pidieron por teléfono, o el resto de una seña que te pagan en cancha, no '
            + 'tienen por qué quedar afuera de tus números solo porque no pasaron por un link de pago.',
          'Confirmando el pago desde el mismo turno, ese cobro entra a tu facturación y a tu reporte del '
            + 'mes exactamente igual que uno pagado online.',
        ],
      },
      {
        tipo: 'lista',
        heading: 'Qué ves al confirmar el pago',
        items: [
          'El monto que falta cobrar de ese turno',
          'Si querés cobrar el total o solo la seña',
          'El método real: efectivo, transferencia, débito, crédito o QR',
          'El turno pasando de sin cobrar a pagado, al toque',
        ],
      },
    ],
    faq: [
      {
        question: '¿Este cobro pasa por MercadoPago?',
        answer:
          'No. Confirmar pago registra que vos cobraste en el mostrador; MercadoPago solo interviene en '
          + 'el cobro online de la seña.',
      },
      {
        question: '¿Puedo cobrar solo la seña y dejar el resto pendiente?',
        answer:
          'Sí. Elegís si confirmás el pago completo o solo la seña, y el turno queda con el saldo '
          + 'restante a la vista hasta que lo cobres.',
      },
      {
        question: '¿Este cobro entra a la caja del turno?',
        answer:
          'Si lo cobraste en efectivo, sí, suma al efectivo esperado. Con otro método queda igual '
          + 'registrado en el resumen de cobros de reservas.',
      },
      {
        question: '¿Puedo elegir cualquier método al confirmar el pago?',
        answer:
          'Sí: efectivo, transferencia, débito, crédito o QR, el que el cliente haya usado en el '
          + 'momento.',
      },
      {
        question: '¿Qué pasa si el cliente ya había pagado la seña online?',
        answer:
          'Confirmar pago te pide entonces solo el resto: la seña ya cobrada por MercadoPago no se '
          + 'vuelve a pedir.',
      },
    ],
    relacionados: ['grilla-de-reservas', 'ingresos-egresos-y-anulaciones'],
  },
];

/* La fecha sale del contenido, igual que en las guías. Las claves se prefijan
   para que un tutorial y una guía con el mismo slug nunca se pisen en el
   registro compartido. */
export const tutoriales: Tutorial[] = contenido.map(t => {
  const { datePublished, ...loQueSeMide } = t;
  return { ...t, dateModified: fechaDeContenido(`tutorial:${t.slug}`, loQueSeMide) };
});
guardarFechas();

export const getTutorial = (slug: string) => tutoriales.find(t => t.slug === slug);

export const getRelacionados = (t: Tutorial) =>
  t.relacionados.map(getTutorial).filter((x): x is Tutorial => Boolean(x));

/** Fecha del tutorial más nuevo, para el índice. */
export const ULTIMA_ACTUALIZACION = tutoriales
  .map(t => t.dateModified)
  .sort()
  .at(-1) ?? '2026-09-16';
