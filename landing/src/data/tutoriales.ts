/**
 * Tutoriales = una pantalla del panel, explicada en texto, una URL.
 *
 * No son guías. Una guía contesta algo que se busca ANTES de saber que Vibe
 * existe, y cita fuentes externas. Un tutorial explica una pantalla del panel
 * funcionando, y su fuente es el producto. Van separados para que ninguna de
 * las dos cosas diluya a la otra: mezcladas, las guías dejan de leerse como
 * contenido editorial y los tutoriales dejan de encontrarse.
 *
 * `respuesta` va primero y por el mismo motivo que en las guías: un motor
 * generativo cita unidades que puede extraer enteras. Es el resumen de la
 * pantalla que se explica más abajo, sin depender de ningún otro elemento de
 * la página.
 */
import type { Bloque } from './guias.ts';
import { fechaDeContenido, guardarFechas } from './fecha-de-contenido.ts';

export type TutorialFaq = { question: string; answer: string };

export type Tutorial = {
  slug: string;
  /** H1 de la página. */
  title: string;
  seoTitle: string;
  metaDescription: string;
  /** Lo que muestra esa pantalla, en un párrafo. */
  respuesta: string;
  /** Bajada del índice. */
  excerpt: string;
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
    datePublished: '2026-09-16',
    respuesta:
      'La grilla pone todas las canchas del complejo una al lado de la otra, con cada turno en su '
      + 'horario. Tocás uno y ves quién reservó, a qué hora, el precio del turno y cuánto pagó de '
      + 'seña. Con las flechas te movés día por día, y si estás en otro día, Ir a hoy te trae de '
      + 'vuelta. Si una cancha queda fuera de servicio, bloqueás el horario desde la misma pantalla y '
      + 'ese turno deja de ofrecerse en tu página de reservas.',
    bloques: [
      {
        tipo: 'pasos',
        heading: 'Paso a paso, en la grilla de reservas',
        intro:
          'Entrá a Reservas desde el menú de la izquierda. El número marcado en cada captura es el del '
          + 'paso. Si tocás la captura, ves la pantalla completa.',
        pasos: [
          {
            titulo: 'Elegí el día',
            texto:
              'Con las flechas pasás al día anterior o al siguiente, y en la tira de abajo tocás cualquier '
              + 'día de la semana. Si te moviste a otro día, Ir a hoy te trae de vuelta.',
            imagen: {
              src: '/tutoriales/pasos/grilla-de-reservas/01-elegir-el-dia',
              alt: 'Flechas y fecha arriba de la grilla de reservas, marcadas con el número 1, sobre la tira de días de la semana.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/grilla-de-reservas/zoom-01-elegir-el-dia', width: 471, height: 175 },
            },
          },
          {
            titulo: 'Tocá un turno',
            texto:
              'Cada cancha es una columna y cada turno, un bloque con su horario y el nombre del cliente. '
              + 'Tocalo para abrir el detalle.',
            imagen: {
              src: '/tutoriales/pasos/grilla-de-reservas/02-tocar-un-turno',
              alt: 'Turno de 18:30 a 20:00 en la columna de Pádel 1, marcado con el número 2.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/grilla-de-reservas/zoom-02-tocar-un-turno', width: 265, height: 153 },
            },
          },
          {
            titulo: 'Mirá el estado del pago',
            texto:
              'En el detalle ves si la seña está pagada, el precio del turno y cuánto pagó de seña y por '
              + 'qué medio. Desde ahí también confirmás el pago, marcás que no se presentó o cancelás la '
              + 'reserva.',
            imagen: {
              src: '/tutoriales/pasos/grilla-de-reservas/03-estado-y-pago',
              alt: 'Detalle de una reserva con el estado del pago, el precio y la seña marcados con el número 3.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/grilla-de-reservas/zoom-03-estado-y-pago', width: 260, height: 208 },
            },
          },
          {
            titulo: 'Tocá Bloquear horario',
            texto:
              'Está arriba a la derecha, al lado de Nueva reserva. Sirve para mantenimiento, un evento '
              + 'privado o una cancha que no se puede usar.',
            imagen: {
              src: '/tutoriales/pasos/grilla-de-reservas/04-bloquear-horario',
              alt: 'Botón Bloquear horario, arriba a la derecha de la grilla, marcado con el número 4.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/grilla-de-reservas/zoom-04-bloquear-horario', width: 260, height: 125 },
            },
          },
          {
            titulo: 'Elegí qué bloquear',
            texto:
              'Elegí la fecha, la cancha y desde qué hora hasta qué hora. Si querés, sumá un motivo.',
            imagen: {
              src: '/tutoriales/pasos/grilla-de-reservas/05-datos-del-bloqueo',
              alt: 'Formulario Bloquear horario con fecha, cancha, hora de inicio, hora de fin y motivo, marcado con el número 5.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/grilla-de-reservas/zoom-05-datos-del-bloqueo', width: 469, height: 381 },
            },
          },
          {
            titulo: 'Confirmá el bloqueo',
            texto:
              'Tocá Bloquear horario. Ese horario deja de ofrecerse en tu página de reservas. Para '
              + 'sacarlo, tocás el horario bloqueado en la grilla y lo desbloqueás.',
            imagen: {
              src: '/tutoriales/pasos/grilla-de-reservas/06-confirmar-bloqueo',
              alt: 'Botón Bloquear horario al pie del formulario, marcado con el número 6.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/grilla-de-reservas/zoom-06-confirmar-bloqueo', width: 260, height: 107 },
            },
          },
          {
            titulo: 'Cargá una reserva por teléfono',
            texto:
              'Tocá Nueva reserva y elegí la fecha, la cancha y el horario, después el cliente y el pago. '
              + 'También podés tocar un horario libre de la grilla.',
            imagen: {
              src: '/tutoriales/pasos/grilla-de-reservas/07-nueva-reserva',
              alt: 'Botón Nueva reserva, arriba a la derecha de la grilla, marcado con el número 7.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/grilla-de-reservas/zoom-07-nueva-reserva', width: 260, height: 125 },
            },
          },
        ],
      },
      {
        tipo: 'parrafos',
        heading: 'Toda la agenda, en una sola pantalla',
        parrafos: [
          'Te llaman: "¿hay lugar el jueves a las nueve?". La respuesta está acá, sin hojear nada: qué '
            + 'está tomado y qué queda libre, en la cancha que te piden.',
          'Y el cobro también está a la vista. Un turno con la seña pagada y uno reservado por '
            + 'teléfono se distinguen en el detalle, así que no cruzás la agenda con los pagos para '
            + 'saber quién te debe.',
        ],
      },
    ],
    faq: [
      {
        question: '¿Qué pasa cuando bloqueo un horario?',
        answer:
          'Desaparece de tu página de reservas y nadie lo puede tomar. Sirve para mantenimiento, un '
          + 'torneo o una cancha inundada. Para sacarlo, tocás el horario bloqueado en la grilla y lo '
          + 'desbloqueás.',
      },
      {
        question: '¿Puedo cargar una reserva que me pidieron por teléfono?',
        answer:
          'Sí. Ocupa el turno igual que una online y queda en el historial del cliente. No pasa por '
          + 'Mercado Pago: si te pagan algo, lo registrás en el momento en efectivo, transferencia, '
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
          'No desde la misma reserva. En el detalle confirmás el pago, marcás que no se presentó o la '
          + 'cancelás. Para moverla, la cancelás y cargás una nueva.',
      },
      {
        question: 'Si entra una reserva con la grilla abierta, ¿tengo que recargar?',
        answer:
          'No. El panel se actualiza en tiempo real: la reserva nueva o la cancelación aparece sola, '
          + 'sin tocar nada.',
      },
    ],
    relacionados: ['precios-por-horario', 'asistencia-de-cada-turno', 'base-de-clientes', 'turnos-fijos'],
  },
  {
    slug: 'precios-por-horario',
    title: 'Precios por horario: cobrá la noche más cara',
    seoTitle: 'Precios por horario y tarifa nocturna en tu cancha | Vibe',
    metaDescription:
      'Cargá un precio base por día y una franja nocturna más cara. Cómo cobra Vibe un turno que '
      + 'cruza de una tarifa a la otra, por media hora.',
    excerpt: 'Un precio base por día y, arriba, las franjas que cobrás distinto.',
    datePublished: '2026-09-16',
    respuesta:
      'Cada cancha arranca con un precio base para todo el día. Sobre ese precio abrís cualquier día '
      + 'de la semana y le sumás una franja con otro valor: así se carga la tarifa nocturna. El cobro '
      + 'no redondea a la franja donde empieza el turno: cada media hora se cobra a su propia tarifa. '
      + 'Un turno de una hora que arranca a las 17:30, con la noche empezando a las 18:00, se cobra '
      + 'media hora a precio de día y media hora a precio de noche.',
    bloques: [
      {
        tipo: 'pasos',
        heading: 'Paso a paso, en Canchas',
        intro:
          'Entrá a Canchas desde el menú de la izquierda. El número marcado en cada captura es el del paso.'
          + ' Si tocás la captura, ves la pantalla completa.',
        pasos: [
          {
            titulo: 'Abrí las acciones de la cancha',
            texto:
              'En la fila de la cancha, tocá los tres puntos de la columna Acciones.',
            imagen: {
              src: '/tutoriales/pasos/precios-por-horario/01-mas-acciones',
              alt: 'Botón de tres puntos en la columna Acciones de la cancha Pádel 1, marcado con el número 1.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/precios-por-horario/zoom-01-mas-acciones', width: 260, height: 141 },
            },
          },
          {
            titulo: 'Tocá Precios',
            texto:
              'En el menú que se abre, elegí Precios. Editar es para el nombre, el deporte y el tipo de cancha.',
            imagen: {
              src: '/tutoriales/pasos/precios-por-horario/02-precios',
              alt: 'Menú de la cancha con Editar, Precios y Eliminar, con Precios marcado con el número 2.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/precios-por-horario/zoom-02-precios', width: 260, height: 131 },
            },
          },
          {
            titulo: 'Cargá el precio de cada día',
            texto:
              'Cada día tiene su precio por hora, que vale para todo el día. Escribilo en el campo de ese día.',
            imagen: {
              src: '/tutoriales/pasos/precios-por-horario/03-precio-del-dia',
              alt: 'Campo del precio por hora del lunes, marcado con el número 3.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/precios-por-horario/zoom-03-precio-del-dia', width: 260, height: 169 },
            },
          },
          {
            titulo: 'Copialo a toda la semana',
            texto:
              'El botón de al lado aplica ese precio a todos los días. Las franjas que ya cargaste quedan como '
              + 'estaban.',
            imagen: {
              src: '/tutoriales/pasos/precios-por-horario/04-aplicar-a-todos',
              alt: 'Botón Aplicar a todos, al lado del precio del lunes, marcado con el número 4.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/precios-por-horario/zoom-04-aplicar-a-todos', width: 260, height: 165 },
            },
          },
          {
            titulo: 'Abrí el día',
            texto:
              'Tocá la flecha a la derecha del día que cobrás distinto en algún horario, por ejemplo el '
              + 'viernes.',
            imagen: {
              src: '/tutoriales/pasos/precios-por-horario/05-abrir-el-dia',
              alt: 'Flecha para abrir las franjas del viernes, marcada con el número 5.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/precios-por-horario/zoom-05-abrir-el-dia', width: 285, height: 115 },
            },
          },
          {
            titulo: 'Tocá Nuevo precio',
            texto:
              'Se agrega una franja para ese día, debajo del precio de todo el día.',
            imagen: {
              src: '/tutoriales/pasos/precios-por-horario/06-nuevo-precio',
              alt: 'Botón Nuevo precio dentro del viernes, marcado con el número 6.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/precios-por-horario/zoom-06-nuevo-precio', width: 260, height: 101 },
            },
          },
          {
            titulo: 'Elegí el horario de la franja',
            texto:
              'Elegí desde qué hora y hasta qué hora vale. Para la tarifa nocturna, desde las 18:00 hasta el '
              + 'cierre.',
            imagen: {
              src: '/tutoriales/pasos/precios-por-horario/07-horario-de-la-franja',
              alt: 'Horario de la franja del viernes, desde las 18:00 hasta las 23:30, marcado con el número 7.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/precios-por-horario/zoom-07-horario-de-la-franja', width: 309, height: 111 },
            },
          },
          {
            titulo: 'Cargá el precio de la franja',
            texto:
              'Escribí el precio por hora de esa franja. Un turno que la cruza se cobra media hora por media '
              + 'hora.',
            imagen: {
              src: '/tutoriales/pasos/precios-por-horario/08-precio-de-la-franja',
              alt: 'Campo del precio de la franja del viernes, marcado con el número 8.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/precios-por-horario/zoom-08-precio-de-la-franja', width: 260, height: 111 },
            },
          },
          {
            titulo: 'Guardá los precios',
            texto:
              'Tocá Guardar para que queden los precios de todos los días y sus franjas.',
            imagen: {
              src: '/tutoriales/pasos/precios-por-horario/09-guardar',
              alt: 'Botón Guardar al pie de los precios, marcado con el número 9.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/precios-por-horario/zoom-09-guardar', width: 260, height: 107 },
            },
          },
        ],
      },
      {
        tipo: 'parrafos',
        heading: 'Por qué se cobra media hora por media hora',
        parrafos: [
          'Si cobrás la noche más cara, siempre vas a tener turnos que cruzan el límite. Si el sistema '
            + 'cobrara todo el turno a la tarifa de la hora de inicio, reservar a las 17:30 saldría más '
            + 'barato que a las 18:00 por la misma hora de cancha. Y ese agujero lo encuentra el primer '
            + 'cliente que hace la cuenta.',
          'Cobrar cada media hora a su tarifa cierra el agujero sin que tengás que pensarlo. Cargás el '
            + 'precio de día y el de noche, y cualquier duración con cualquier horario de inicio da el '
            + 'número justo.',
        ],
      },
      {
        tipo: 'lista',
        heading: 'Qué más editás desde Canchas',
        items: [
          'El nombre de la cancha y el deporte que se juega, desde Editar',
          'Si es techada, descubierta o semicubierta',
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
          'Las que necesités. Lo habitual es una, la nocturna, pero si cobrás distinto al mediodía '
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
    relacionados: ['grilla-de-reservas', 'reportes-de-facturacion', 'agregar-y-editar-canchas'],
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
    datePublished: '2026-09-16',
    respuesta:
      'La base de clientes no se carga: se llena sola con cada reserva. De cada persona queda el '
      + 'teléfono, cuántas veces reservó, cuántas faltó y su porcentaje de asistencia. El buscador '
      + 'encuentra por nombre o por parte del teléfono, y la ficha de cada cliente muestra sus últimas '
      + 'reservas con la fecha, la cancha y cómo terminó cada una.',
    bloques: [
      {
        tipo: 'pasos',
        heading: 'Paso a paso, en Clientes',
        intro:
          'Entrá a Clientes desde el menú de la izquierda. El número marcado en cada captura es el del '
          + 'paso. Si tocás la captura, ves la pantalla completa.',
        pasos: [
          {
            titulo: 'Buscá un cliente',
            texto:
              'Escribí el nombre, el apellido o parte del teléfono y la lista muestra solo los que coinciden.',
            imagen: {
              src: '/tutoriales/pasos/base-de-clientes/01-buscar',
              alt: 'Buscador de clientes con un nombre escrito, marcado con el número 1.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/base-de-clientes/zoom-01-buscar', width: 451, height: 159 },
            },
          },
          {
            titulo: 'Mirá sus reservas y su asistencia',
            texto:
              'Cada fila muestra cuántas reservas tiene, cuántas veces no se presentó y su porcentaje de '
              + 'asistencia, en verde, amarillo o rojo según cómo viene.',
            imagen: {
              src: '/tutoriales/pasos/base-de-clientes/02-reservas-y-asistencia',
              alt: 'Columnas Reservas, Ausencias y Asistencia de un cliente, marcadas con el número 2.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/base-de-clientes/zoom-02-reservas-y-asistencia', width: 484, height: 154 },
            },
          },
          {
            titulo: 'Abrí su ficha',
            texto:
              'Tocá el cliente para abrir su ficha. Arriba están sus reservas, sus ausencias y su asistencia.',
            imagen: {
              src: '/tutoriales/pasos/base-de-clientes/03-ficha',
              alt: 'Ficha de un cliente con sus reservas, ausencias y asistencia, marcadas con el número 3.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/base-de-clientes/zoom-03-ficha', width: 260, height: 221 },
            },
          },
          {
            titulo: 'Anotá lo que quieras recordar',
            texto:
              'En Notas escribís lo que te sirva para atenderlo mejor, por ejemplo qué cancha prefiere. Se '
              + 'guarda solo mientras escribís.',
            imagen: {
              src: '/tutoriales/pasos/base-de-clientes/04-notas',
              alt: 'Campo Notas de la ficha del cliente con una nota escrita, marcado con el número 4.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/base-de-clientes/zoom-04-notas', width: 518, height: 175 },
            },
          },
          {
            titulo: 'Revisá sus últimas reservas',
            texto:
              'Reservas recientes lista sus últimas reservas con la fecha, la cancha y cómo terminó cada una. '
              + 'Tocá una para abrir su detalle.',
            imagen: {
              src: '/tutoriales/pasos/base-de-clientes/05-reservas-recientes',
              alt: 'Lista de reservas recientes del cliente con el estado de cada una, marcada con el número 5.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/base-de-clientes/zoom-05-reservas-recientes', width: 518, height: 229 },
            },
          },
          {
            titulo: 'Escribile, llamalo o bloquealo',
            texto:
              'En el menú de los tres puntos, arriba de la ficha, tenés WhatsApp, Llamar y Bloquear. Un cliente'
              + ' bloqueado no puede volver a reservar online en tu complejo.',
            imagen: {
              src: '/tutoriales/pasos/base-de-clientes/06-acciones',
              alt: 'Menú de la ficha con WhatsApp, Llamar y Bloquear, marcado con el número 6.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/base-de-clientes/zoom-06-acciones', width: 260, height: 195 },
            },
          },
        ],
      },
      {
        tipo: 'parrafos',
        heading: 'Para qué te sirve la asistencia',
        parrafos: [
          'Es el dato que decide a quién le pedís seña y a quién no. El que viene hace dos años y nunca '
            + 'faltó no necesita el mismo trato que el que reservó tres veces y vino una. Sin el dato, esa '
            + 'decisión se toma de memoria, y la memoria falla justo con el que menos conviene.',
          'También sirve al revés: saber a quién avisarle primero cuando se libera un buen horario, o '
            + 'cuando querés llenar un martes flojo.',
        ],
      },
    ],
    faq: [
      {
        question: '¿Cómo marco que un cliente no se presentó?',
        answer:
          'Desde el turno, en la grilla: abrís el detalle y tocás No se presentó. Esa marca suma una '
          + 'ausencia en su ficha y baja su asistencia, así que el dato vale tanto como la constancia con que'
          + ' la cargás.',
      },
      {
        question: '¿Le tengo que pedir el mail al cliente?',
        answer:
          'No es obligatorio: con el nombre y el teléfono alcanza para reservar. Si deja el mail, le llegan'
          + ' ahí la confirmación y el recordatorio del turno.',
      },
      {
        question: '¿Puedo editar a mano los datos de un cliente?',
        answer:
          'Solo las notas y el bloqueo. El nombre, el teléfono y el mail salen de sus reservas y no se '
          + 'escriben desde la ficha. Lo que sí sumás son notas tuyas, por ejemplo que prefiere cierta cancha'
          + ' o cierto horario.',
      },
      {
        question: '¿La asistencia cuenta las reservas telefónicas o solo las pagadas online?',
        answer:
          'Cuenta todas. El porcentaje sale de las reservas del cliente que no se cancelaron y de las veces'
          + ' que marcaste No se presentó, sea que haya pagado online o que la hayás cargado vos por '
          + 'teléfono.',
      },
    ],
    relacionados: ['grilla-de-reservas', 'asistencia-de-cada-turno', 'panel-de-control', 'bloquear-un-cliente'],
  },
  {
    slug: 'panel-de-control',
    title: 'El panel: tu complejo de un vistazo',
    seoTitle: 'Panel de control para un complejo deportivo | Vibe',
    metaDescription:
      'Lo que facturaste hoy, las reservas, la ocupación y el mapa de horas muertas: qué ves apenas '
      + 'entrás al panel de Vibe, con la caja incluida.',
    excerpt: 'Cuánto facturaste, cuántas reservas tenés y qué canchas están vacías, apenas entrás.',
    datePublished: '2026-09-16',
    respuesta:
      'Entrás al panel y ves lo que facturaste en el día, cuántas reservas tenés y el porcentaje de '
      + 'ocupación de las canchas. Arriba está tu link público de reservas, listo para copiar. Más '
      + 'abajo, el gráfico de ingresos, que mirás por semana o por mes, los clientes que más reservan '
      + 'y el mapa de ocupación por horario, que es donde aparecen las horas que quedan vacías.',
    bloques: [
      {
        tipo: 'pasos',
        heading: 'Paso a paso, en el panel',
        intro:
          'Es la pantalla que ves al entrar a Vibe, y volvés cuando quieras desde Dashboard en el menú de '
          + 'la izquierda. El número marcado en cada captura es el del paso. Si tocás la captura, ves la '
          + 'pantalla completa.',
        pasos: [
          {
            titulo: 'Copiá tu link de reservas',
            texto:
              'Arriba de todo está el link de tu página de reservas. Tocá Copiar enlace y pegalo en tu '
              + 'Instagram o en tu WhatsApp.',
            imagen: {
              src: '/tutoriales/pasos/panel-de-control/01-link-de-reservas',
              alt: 'Link de la página de reservas con el botón Copiar enlace, marcados con el número 1.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/panel-de-control/zoom-01-link-de-reservas', width: 423, height: 103 },
            },
          },
          {
            titulo: 'Mirá lo que facturaste hoy',
            texto:
              'Ingresos de hoy suma lo que entró en el día, separado en turnos, bar y otros.',
            imagen: {
              src: '/tutoriales/pasos/panel-de-control/02-ingresos-de-hoy',
              alt: 'Ingresos de hoy con el total del día y su detalle, marcados con el número 2.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/panel-de-control/zoom-02-ingresos-de-hoy', width: 369, height: 143 },
            },
          },
          {
            titulo: 'Mirá las reservas y la ocupación',
            texto:
              'Debajo ves cuántas reservas tenés hoy, cómo vienen contra ayer y qué parte de las canchas está '
              + 'ocupada.',
            imagen: {
              src: '/tutoriales/pasos/panel-de-control/03-reservas-y-ocupacion',
              alt: 'Reservas de hoy y ocupación de las canchas, marcadas con el número 3.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/panel-de-control/zoom-03-reservas-y-ocupacion', width: 260, height: 201 },
            },
          },
          {
            titulo: 'Revisá las próximas reservas',
            texto:
              'A la derecha están los turnos que siguen hoy, en orden. Tocá uno para ver el detalle, o Ver '
              + 'todas para ir a la grilla.',
            imagen: {
              src: '/tutoriales/pasos/panel-de-control/04-proximas-reservas',
              alt: 'Lista Próximas reservas del panel, marcada con el número 4.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/panel-de-control/zoom-04-proximas-reservas', width: 385, height: 185 },
            },
          },
          {
            titulo: 'Elegí semana o mes',
            texto:
              'En Tendencias, el gráfico de ingresos muestra los últimos 7 días con Semana y los últimos 30 con'
              + ' Mes.',
            imagen: {
              src: '/tutoriales/pasos/panel-de-control/05-semana-o-mes',
              alt: 'Botones Semana y Mes del gráfico de ingresos, marcados con el número 5.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/panel-de-control/zoom-05-semana-o-mes', width: 260, height: 137 },
            },
          },
          {
            titulo: 'Mirá tus clientes fieles',
            texto:
              'En Clientes están los diez que más reservaron en los últimos 30 días, con cuántas reservas hizo '
              + 'cada uno.',
            imagen: {
              src: '/tutoriales/pasos/panel-de-control/06-clientes-que-mas-reservan',
              alt: 'Lista de los clientes que más reservaron en los últimos 30 días, marcada con el número 6.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/panel-de-control/zoom-06-clientes-que-mas-reservan', width: 481, height: 223 },
            },
          },
          {
            titulo: 'Buscá las horas vacías',
            texto:
              'El mapa de ocupación por horario cruza cada hora con cada día de la semana: cuanto más intenso '
              + 'el color, más ocupada está esa hora. Arriba ves el promedio y el horario pico.',
            imagen: {
              src: '/tutoriales/pasos/panel-de-control/07-ocupacion-por-horario',
              alt: 'Título del mapa de ocupación por horario con el promedio y el pico, marcado con el número 7.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/panel-de-control/zoom-07-ocupacion-por-horario', width: 378, height: 235 },
            },
          },
        ],
      },
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
          'No. La caja tiene su propia pantalla, Caja, en el menú de la izquierda. El panel sí te avisa '
          + 'cuando algún producto quedó con stock bajo, con un acceso a productos.',
      },
    ],
    relacionados: ['reportes-de-facturacion', 'base-de-clientes', 'perfil-del-complejo-y-link-publico'],
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
      'El mes cerrado solo: comparado con el anterior, abierto por método de pago y por cancha, y en '
      + 'Excel para el contador, con la caja incluida.',
    excerpt: 'El cierre del mes armado solo, comparado con el anterior y listo para el contador.',
    datePublished: '2026-09-16',
    respuesta:
      'El reporte se arma mes a mes y se compara siempre contra el mes anterior. Te muestra cuánto '
      + 'entró por cada método de pago (Mercado Pago y los que registrás en el mostrador), con los '
      + 'reembolsos ya descontados, así que el número que ves es el neto. También se abre cancha por '
      + 'cancha, para ver cuáles facturan y cuáles casi no se usan. Todo eso se baja en Excel.',
    bloques: [
      {
        tipo: 'pasos',
        heading: 'Paso a paso, en Reportes',
        intro:
          'Entrá a Reportes desde el menú de la izquierda. El número marcado en cada captura es el del '
          + 'paso. Si tocás la captura, ves la pantalla completa.',
        pasos: [
          {
            titulo: 'Elegí el mes y el año',
            texto:
              'Arriba de todo elegís el mes y el año que querés ver. Sirve igual para el mes en curso que para '
              + 'uno que ya cerró.',
            imagen: {
              src: '/tutoriales/pasos/reportes-de-facturacion/01-mes-y-anio',
              alt: 'Selectores de Mes y Año arriba del reporte, marcados con el número 1.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/reportes-de-facturacion/zoom-01-mes-y-anio', width: 369, height: 149 },
            },
          },
          {
            titulo: 'Mirá el neto del mes',
            texto:
              'Es lo que entró en el mes con los reembolsos ya restados. Al lado ves si subió o bajó contra el '
              + 'mes anterior, y abajo, el neto de ese mes.',
            imagen: {
              src: '/tutoriales/pasos/reportes-de-facturacion/02-neto-del-mes',
              alt: 'Neto del mes con su comparación contra el mes anterior, marcado con el número 2.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/reportes-de-facturacion/zoom-02-neto-del-mes', width: 283, height: 153 },
            },
          },
          {
            titulo: 'Revisá cada método de pago',
            texto:
              'La tabla abre el mes por método: cuántos pagos hubo, el total, lo reembolsado y el neto de cada '
              + 'uno, con la suma de todos en la última fila.',
            imagen: {
              src: '/tutoriales/pasos/reportes-de-facturacion/03-por-metodo',
              alt: 'Tabla del reporte por método de pago con cantidad, total, reembolsado y neto, marcada con el número 3.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/reportes-de-facturacion/zoom-03-por-metodo', width: 498, height: 358 },
            },
          },
          {
            titulo: 'Mirá cuánto dejó cada cancha',
            texto:
              'En Por cancha ves el neto de cada una y cuántos pagos tuvo en el mes.',
            imagen: {
              src: '/tutoriales/pasos/reportes-de-facturacion/04-por-cancha',
              alt: 'Sección Por cancha con el neto y la cantidad de pagos de cada cancha, marcada con el número 4.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/reportes-de-facturacion/zoom-04-por-cancha', width: 439, height: 358 },
            },
          },
          {
            titulo: 'Descargá el Excel',
            texto:
              'Descargar Excel te baja el mes elegido en un archivo con dos hojas: Pagos, con cada pago del '
              + 'período, y Resumen, por método y por cancha. Es el que le pasás al contador.',
            imagen: {
              src: '/tutoriales/pasos/reportes-de-facturacion/05-descargar-excel',
              alt: 'Botón Descargar Excel al lado de los selectores de mes y año, marcado con el número 5.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/reportes-de-facturacion/zoom-05-descargar-excel', width: 260, height: 129 },
            },
          },
        ],
      },
      {
        tipo: 'parrafos',
        heading: 'El desglose por cancha te dice qué arreglar',
        parrafos: [
          'El total del mes no te dice qué hacer. El desglose por cancha, sí: una cancha que factura la '
            + 'mitad que las demás está marcando un precio mal puesto, un horario mal cargado o algo para '
            + 'arreglar en la cancha misma.',
          'Cruzado con el mapa de ocupación del panel, separa los dos casos que se confunden siempre: la '
            + 'cancha que se usa poco y la que se usa mucho pero barata.',
        ],
      },
    ],
    faq: [
      {
        question: '¿Qué formato tiene la exportación?',
        answer:
          'Un Excel con dos hojas: los pagos del período y un resumen por método y por cancha. Si ese mes '
          + 'usaste la caja, el resumen suma una sección con las ventas por método y los movimientos por '
          + 'categoría. Se abre igual en Google Sheets.',
      },
      {
        question: '¿Los reembolsos ya están descontados?',
        answer:
          'Sí. El neto que muestra el reporte ya tiene los reembolsos restados: no hacés la cuenta aparte.',
      },
      {
        question: '¿Veo cuánto facturó cada cancha por separado?',
        answer:
          'Sí. En pantalla ves el neto y la cantidad de pagos de cada cancha. En el Excel, el resumen por '
          + 'cancha suma además lo cobrado y lo reembolsado de cada una.',
      },
      {
        question: '¿Hay un límite de pagos por exportación?',
        answer:
          'Sí, hay un tope de filas. Si el mes elegido tiene más pagos que ese tope, el panel te avisa que '
          + 'el período tiene demasiados pagos para exportar.',
      },
      {
        question: '¿Puedo exportar un mes que ya pasó?',
        answer:
          'Sí. Elegís el mes y el año en los mismos selectores, así que sirve igual para el mes en curso '
          + 'que para uno cerrado hace tiempo.',
      },
    ],
    relacionados: ['panel-de-control', 'precios-por-horario'],
    guia: {
      slug: 'cuanto-cobra-mercadopago-por-una-sena',
      texto: 'Cuánto cobra Mercado Pago por una seña',
    },
  },
  {
    slug: 'configuracion-del-complejo',
    title: 'Configurá tu complejo: horarios y seña',
    seoTitle: 'Cómo configurar horarios y seña de tu complejo | Vibe',
    metaDescription:
      'Los datos del complejo, hasta qué hora se reserva cada día, cuánta seña pedís y el estado de '
      + 'Mercado Pago: todo se define en una sola pantalla.',
    excerpt: 'Los datos del complejo, los horarios de atención y cuánta seña pedís.',
    datePublished: '2026-09-16',
    respuesta:
      'En Configuración cargás el nombre, la dirección y el teléfono del complejo, el porcentaje '
      + 'de seña que se pide al reservar online y hasta cuántas horas antes se puede cancelar sin '
      + 'cargo. Los horarios de atención se definen día por día: fuera de esa franja, tu página no '
      + 'ofrece turnos. En la pestaña Cobros ves si Mercado Pago está conectado y cuánto cobra según '
      + 'el plazo de acreditación que elegiste.',
    bloques: [
      {
        tipo: 'pasos',
        heading: 'Paso a paso, en Configuración',
        intro:
          'El número marcado en cada captura es el del paso. Si tocás la captura, ves la pantalla completa.',
        pasos: [
          {
            titulo: 'Entrá a Configuración',
            texto:
              'En el menú de la izquierda, tocá Configuración. Se abre en la pestaña General.',
            imagen: {
              src: '/tutoriales/pasos/configuracion-del-complejo/01-configuracion',
              alt: 'Opción Configuración del menú de la izquierda, marcada con el número 1.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/configuracion-del-complejo/zoom-01-configuracion', width: 260, height: 103 },
            },
          },
          {
            titulo: 'Revisá los datos del complejo',
            texto:
              'En General cargás el nombre, la descripción, la dirección y el teléfono. La URL pública es la '
              + 'última parte del link de tu página de reservas.',
            imagen: {
              src: '/tutoriales/pasos/configuracion-del-complejo/02-url-publica',
              alt: 'Campo URL pública de la pestaña General, marcado con el número 2.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/configuracion-del-complejo/zoom-02-url-publica', width: 391, height: 133 },
            },
          },
          {
            titulo: 'Elegí el porcentaje de seña',
            texto:
              'Es la parte del turno que se cobra al reservar online.',
            imagen: {
              src: '/tutoriales/pasos/configuracion-del-complejo/03-porcentaje-de-sena',
              alt: 'Campo Porcentaje de seña, marcado con el número 3.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/configuracion-del-complejo/zoom-03-porcentaje-de-sena', width: 281, height: 133 },
            },
          },
          {
            titulo: 'Definí las horas de cancelación',
            texto:
              'Son las horas antes del turno hasta las que se puede cancelar sin cargo.',
            imagen: {
              src: '/tutoriales/pasos/configuracion-del-complejo/04-horas-de-cancelacion',
              alt: 'Campo Horas de cancelación, marcado con el número 4.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/configuracion-del-complejo/zoom-04-horas-de-cancelacion', width: 292, height: 133 },
            },
          },
          {
            titulo: 'Guardá los cambios',
            texto:
              'Al final de General, tocá Guardar cambios.',
            imagen: {
              src: '/tutoriales/pasos/configuracion-del-complejo/05-guardar-cambios',
              alt: 'Botón Guardar cambios al final de la pestaña General, marcado con el número 5.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/configuracion-del-complejo/zoom-05-guardar-cambios', width: 291, height: 111 },
            },
          },
          {
            titulo: 'Abrí Horarios',
            texto:
              'En la columna de pestañas, tocá Horarios.',
            imagen: {
              src: '/tutoriales/pasos/configuracion-del-complejo/06-horarios',
              alt: 'Pestaña Horarios de Configuración, marcada con el número 6.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/configuracion-del-complejo/zoom-06-horarios', width: 279, height: 119 },
            },
          },
          {
            titulo: 'Elegí la apertura y el cierre de cada día',
            texto:
              'Para cada día elegís a qué hora abre y a qué hora cierra. Si un día no abrís, marcá Cerrado. '
              + 'Después tocá Guardar.',
            imagen: {
              src: '/tutoriales/pasos/configuracion-del-complejo/07-horario-del-dia',
              alt: 'Horario del sábado, de las 08:00 a las 23:30, marcado con el número 7.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/configuracion-del-complejo/zoom-07-horario-del-dia', width: 437, height: 111 },
            },
          },
          {
            titulo: 'Mirá el estado de Mercado Pago',
            texto:
              'En la pestaña Cobros ves si tu cuenta de Mercado Pago está conectada. Conectada, la seña se '
              + 'cobra al reservar y entra directo a tu cuenta.',
            imagen: {
              src: '/tutoriales/pasos/configuracion-del-complejo/08-cobros',
              alt: 'Estado Mercado Pago conectado en la pestaña Cobros, marcado con el número 8.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/configuracion-del-complejo/zoom-08-cobros', width: 260, height: 143 },
            },
          },
          {
            titulo: 'Revisá el plazo de acreditación',
            texto:
              'Debajo ves lo que cobra Mercado Pago en cada plazo de acreditación. Para cambiar el plazo, tocá '
              + 'Cómo cambiar el plazo de acreditación.',
            imagen: {
              src: '/tutoriales/pasos/configuracion-del-complejo/09-plazo-de-acreditacion',
              alt: 'Enlace Cómo cambiar el plazo de acreditación, debajo de los costos de Mercado Pago, marcado con el número 9.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/configuracion-del-complejo/zoom-09-plazo-de-acreditacion', width: 380, height: 86 },
            },
          },
        ],
      },
      {
        tipo: 'parrafos',
        heading: 'La seña es la decisión que más pesa',
        parrafos: [
          'El porcentaje de seña es lo único de esta pantalla que elegís vos y no copiás de un dato '
            + 'que ya existe. Una seña baja hace fácil reservar, y también faltar. Una seña alta filtra '
            + 'al que no va a ir, y también a alguno que sí iba.',
          'Revisalo contra las ausencias y el porcentaje de asistencia que te muestra la base de clientes. '
            + 'No lo dejés donde quedó el primer día.',
        ],
      },
    ],
    faq: [
      {
        question: '¿Qué pasa si no conecto Mercado Pago?',
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
        question: '¿Puedo cobrar la cancha completa como seña?',
        answer:
          'Sí. El porcentaje va de cero a cien: si querés cobrar la cancha completa por adelantado, '
          + 'ponés cien y listo.',
      },
      {
        question: 'Si cambio el porcentaje de seña, ¿cambian las reservas que ya están cargadas?',
        answer:
          'No. Cada reserva guarda la seña que correspondía el día que se cargó, así que el cambio '
          + 'solo toca las reservas nuevas.',
      },
    ],
    relacionados: ['precios-por-horario', 'plazo-de-acreditacion-mercado-pago', 'cancelar-una-reserva-y-devolver', 'conectar-mercado-pago', 'perfil-del-complejo-y-link-publico'],
    guia: {
      slug: 'cuanto-cobra-mercadopago-por-una-sena',
      texto: 'Cuánto cobra Mercado Pago por una seña',
    },
  },
  {
    /* Steps checked on 2026-11-16 against the owner's own captures of the Mercado Pago
       panel and https://www.mercadopago.com.ar/ayuda/33399 ("Los costos varían de
       acuerdo a [...] El plazo que definas para recibir tu dinero"). No rate is
       written here: the captures show Mercado Pago's own figures, the copy never does. */
    slug: 'plazo-de-acreditacion-mercado-pago',
    title: 'Cambiá el plazo de acreditación de Mercado Pago',
    seoTitle: 'Cómo cambiar el plazo de acreditación de Mercado Pago | Vibe',
    metaDescription:
      'Paso a paso para elegir cuándo te queda disponible la plata de cada cobro con Mercado Pago, '
      + 'y qué cambia en lo que te cobra según el plazo.',
    excerpt: 'Cuándo te queda disponible la plata de cada seña, y cómo cambiarlo en tu cuenta de Mercado Pago.',
    datePublished: '2026-11-16',
    respuesta:
      'El plazo de acreditación es cuánto tarda en quedarte disponible la plata de cada cobro con '
      + 'Mercado Pago. Se cambia en tu cuenta de Mercado Pago, no en Vibe: en Costos y cuotas entrás a '
      + 'Checkout, después a Por cobro y a Configurar costos por cobro, elegís el plazo y tocás Guardar. '
      + 'Mercado Pago cobra según el plazo que elijas: cuanto antes querés la plata, más alto es su '
      + 'costo por cobro. Las señas de tus reservas se cobran con Checkout, así que este es el plazo '
      + 'que les aplica.',
    bloques: [
      {
        tipo: 'pasos',
        heading: 'Paso a paso, en tu cuenta de Mercado Pago',
        intro:
          'Entrá a Mercado Pago desde la computadora, con la cuenta que conectaste a Vibe. El número '
          + 'marcado en cada captura es el del paso. Si tocás la captura, ves la pantalla completa.',
        pasos: [
          {
            titulo: 'Entrá a Costos y cuotas',
            texto: 'En el menú de la izquierda, dentro de Negocio, tocá Costos y cuotas.',
            imagen: {
              src: '/tutoriales/pasos/plazo-de-acreditacion-mercado-pago/01-costos-y-cuotas',
              alt: 'Menú de la izquierda de Mercado Pago, sección Negocio, con Costos y cuotas marcado con el número 1.',
              width: 1898,
              height: 862,
              recorte: { src: '/tutoriales/pasos/plazo-de-acreditacion-mercado-pago/zoom-01-costos-y-cuotas', width: 244, height: 292 },
            },
          },
          {
            titulo: 'Elegí la pestaña Checkout',
            texto: 'Arriba de la pantalla, entre las formas de cobrar, tocá Checkout.',
            imagen: {
              src: '/tutoriales/pasos/plazo-de-acreditacion-mercado-pago/02-checkout-por-cobro',
              alt: 'Pestañas de Costos y cuotas en Mercado Pago, con Checkout marcada con el número 2.',
              width: 1907,
              height: 860,
              recorte: { src: '/tutoriales/pasos/plazo-de-acreditacion-mercado-pago/zoom-02-checkout', width: 400, height: 80 },
            },
          },
          {
            titulo: 'Quedate en Por cobro',
            texto: 'Por cobro es la opción que ya viene elegida. Debajo ves lo que te cobra Mercado Pago con el plazo que tenés hoy.',
            imagen: {
              src: '/tutoriales/pasos/plazo-de-acreditacion-mercado-pago/02-checkout-por-cobro',
              alt: 'Opción Por cobro de Checkout, marcada con el número 3, al lado de Por ofrecer cuotas.',
              width: 1907,
              height: 860,
              recorte: { src: '/tutoriales/pasos/plazo-de-acreditacion-mercado-pago/zoom-03-por-cobro', width: 400, height: 119 },
            },
          },
          {
            titulo: 'Tocá Configurar costos por cobro',
            texto: 'Está al final de la lista de costos de Checkout.',
            imagen: {
              src: '/tutoriales/pasos/plazo-de-acreditacion-mercado-pago/02-checkout-por-cobro',
              alt: 'Botón Configurar costos por cobro, marcado con el número 4, al final de la lista de costos.',
              width: 1907,
              height: 860,
              recorte: { src: '/tutoriales/pasos/plazo-de-acreditacion-mercado-pago/zoom-04-configurar-costos', width: 420, height: 98 },
            },
          },
          {
            titulo: 'Elegí el plazo',
            texto: 'En Dinero disponible en, elegí cuándo querés tener la plata de cada cobro: al instante o unos días después. Cada opción muestra su costo.',
            imagen: {
              src: '/tutoriales/pasos/plazo-de-acreditacion-mercado-pago/03-elegir-plazo',
              alt: 'Opciones de Dinero disponible en, al instante, 10 y 35 días, marcadas con el número 5.',
              width: 1908,
              height: 859,
              recorte: { src: '/tutoriales/pasos/plazo-de-acreditacion-mercado-pago/zoom-05-plazo', width: 519, height: 315 },
            },
          },
          {
            titulo: 'Guardá el cambio',
            texto: 'Tocá Guardar para que quede el plazo que elegiste.',
            imagen: {
              src: '/tutoriales/pasos/plazo-de-acreditacion-mercado-pago/03-elegir-plazo',
              alt: 'Botón Guardar, marcado con el número 6, al lado de Cancelar.',
              width: 1908,
              height: 859,
              recorte: { src: '/tutoriales/pasos/plazo-de-acreditacion-mercado-pago/zoom-06-guardar', width: 334, height: 102 },
            },
          },
        ],
      },
      {
        tipo: 'parrafos',
        heading: 'Antes o más barato',
        parrafos: [
          'Mercado Pago cobra según el plazo que elijas. Cuanto antes querés tener la plata, más alto '
            + 'es su costo por cobro. Si podés esperar unos días, te queda más de cada seña.',
          'El costo también depende de la provincia de tu domicilio y no incluye IVA ni retenciones. '
            + 'La tabla vigente está en la '
            + '<a href="https://www.mercadopago.com.ar/ayuda/33399" rel="noopener">ayuda de Mercado Pago</a>, '
            + 'y en Vibe la ves en Configuración, en la pestaña Cobros.',
        ],
      },
    ],
    faq: [
      {
        question: '¿El plazo se cambia desde Vibe?',
        answer:
          'No. El plazo es una opción de tu cuenta de Mercado Pago, así que se cambia allá. En Vibe, en '
          + 'Configuración y en la pestaña Cobros, ves lo que cobra Mercado Pago en cada plazo y un '
          + 'enlace a este tutorial.',
      },
      {
        question: '¿Qué plazo me conviene?',
        answer:
          'Depende de cuándo necesitás la plata. Si la usás en el día a día del complejo, al instante '
          + 'te la deja disponible enseguida. Si podés esperar, un plazo más largo te deja más de cada seña.',
      },
      {
        question: '¿Cuánto cobra Mercado Pago en cada plazo?',
        answer:
          'Depende del plazo y de la provincia de tu domicilio. La tabla vigente, con la fuente de '
          + 'Mercado Pago, está en la guía sobre cuánto cobra Mercado Pago por una seña.',
      },
    ],
    relacionados: ['configuracion-del-complejo', 'reportes-de-facturacion', 'conectar-mercado-pago'],
    guia: {
      slug: 'cuanto-cobra-mercadopago-por-una-sena',
      texto: 'Cuánto cobra Mercado Pago por una seña',
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
    datePublished: '2026-09-24',
    respuesta:
      'Abrís la caja cargando el efectivo con el que arrancás el turno. Mientras está abierta, ves el '
      + 'efectivo esperado en cualquier momento: lo que debería haber en el cajón según lo que entró y '
      + 'salió. Al cerrar, contás el efectivo real y lo cargás; Vibe lo compara contra lo esperado y te '
      + 'muestra la diferencia, sobrante o faltante. Una caja cerrada no se puede reabrir ni editar: es el '
      + 'corte del turno.',
    bloques: [
      {
        tipo: 'pasos',
        heading: 'Paso a paso, en Caja',
        intro:
          'Entrás desde Caja en el menú de la izquierda, en la pestaña Turno. El número marcado en cada '
          + 'captura es el del paso. Si tocás la captura, ves la pantalla completa.',
        pasos: [
          {
            titulo: 'Tocá Abrir caja',
            texto:
              'Con la caja cerrada, la pestaña Turno te muestra el botón Abrir caja. Tocalo al empezar el '
              + 'turno.',
            imagen: {
              src: '/tutoriales/pasos/abrir-y-cerrar-la-caja/01-abrir-caja',
              alt: 'Botón Abrir caja de la pantalla Caja, marcado con el número 1.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/abrir-y-cerrar-la-caja/zoom-01-abrir-caja', width: 356, height: 195 },
            },
          },
          {
            titulo: 'Cargá el efectivo del cajón',
            texto:
              'En Monto inicial cargás el efectivo con el que arrancás el turno. La nota es opcional. Tocá '
              + 'Abrir caja para confirmar.',
            imagen: {
              src: '/tutoriales/pasos/abrir-y-cerrar-la-caja/02-monto-inicial',
              alt: 'Campo Monto inicial del formulario para abrir la caja, marcado con el número 2.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/abrir-y-cerrar-la-caja/zoom-02-monto-inicial', width: 481, height: 273 },
            },
          },
          {
            titulo: 'Mirá el efectivo esperado',
            texto:
              'Con la caja abierta, arriba ves el efectivo esperado: lo que debería haber en el cajón. Debajo '
              + 'está la cuenta, con el efectivo inicial y lo que entró y salió en efectivo.',
            imagen: {
              src: '/tutoriales/pasos/abrir-y-cerrar-la-caja/03-efectivo-esperado',
              alt: 'Efectivo esperado de la caja abierta, marcado con el número 3.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/abrir-y-cerrar-la-caja/zoom-03-efectivo-esperado', width: 260, height: 145 },
            },
          },
          {
            titulo: 'Tocá Cerrar caja',
            texto:
              'Al terminar el turno, tocá Cerrar caja, al lado de los botones Ingreso y Egreso.',
            imagen: {
              src: '/tutoriales/pasos/abrir-y-cerrar-la-caja/04-cerrar-caja',
              alt: 'Botón Cerrar caja, marcado con el número 4.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/abrir-y-cerrar-la-caja/zoom-04-cerrar-caja', width: 338, height: 137 },
            },
          },
          {
            titulo: 'Cargá lo que contaste',
            texto:
              'Contá el cajón y cargalo en Efectivo contado. Vibe te muestra al momento la diferencia contra el'
              + ' esperado: sobrante, faltante o sin diferencia. Tocá Cerrar caja para confirmar.',
            imagen: {
              src: '/tutoriales/pasos/abrir-y-cerrar-la-caja/05-efectivo-contado',
              alt: 'Efectivo esperado, efectivo contado y diferencia al cerrar la caja, marcados con el número 5.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/abrir-y-cerrar-la-caja/zoom-05-efectivo-contado', width: 469, height: 353 },
            },
          },
          {
            titulo: 'Revisá el historial',
            texto:
              'Cada caja cerrada queda en Historial de cajas, con su horario, el efectivo esperado, el contado '
              + 'y la diferencia. Tocá una para ver su detalle.',
            imagen: {
              src: '/tutoriales/pasos/abrir-y-cerrar-la-caja/06-historial-de-cajas',
              alt: 'Historial de cajas con un cierre y su faltante, marcado con el número 6.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/abrir-y-cerrar-la-caja/zoom-06-historial-de-cajas', width: 503, height: 227 },
            },
          },
        ],
      },
      {
        tipo: 'parrafos',
        heading: 'La cuenta que se arma sola durante el turno',
        parrafos: [
          'El efectivo esperado se arma solo mientras trabajás: cada cobro en efectivo, cada venta y '
            + 'cada ingreso o egreso que cargás durante el turno va sumando en tiempo real.',
          'Al cerrar, solo contás el cajón y comparás contra ese número. Si hay una diferencia, la ves '
            + 'al toque, con el detalle del turno completo a la vista.',
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
          'Una vez que cerraste la caja con ese conteo, no se puede reabrir ni editar: la diferencia queda '
          + 'registrada así. Por eso conviene contar dos veces antes de tocar Cerrar caja.',
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
          'No. Podés cobrar una reserva con la caja cerrada, pero un cobro en efectivo suma al esperado solo '
          + 'si entra con la caja abierta. Vender productos o reponer stock sí necesita la caja abierta.',
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
    datePublished: '2026-09-24',
    respuesta:
      'Con la caja abierta, cargás un ingreso o un egreso eligiendo su categoría (clases, torneos, '
      + 'alquiler del local, sueldos, insumos, entre otras), el método y el monto. Cada uno queda en el '
      + 'historial del turno. Si cargaste algo mal, no lo borrás: lo anulás, y Vibe registra un movimiento '
      + 'contrario por el mismo monto, así el historial completo queda a la vista.',
    bloques: [
      {
        tipo: 'pasos',
        heading: 'Paso a paso, en Caja',
        intro:
          'Se hace desde Caja, en la pestaña Turno, con la caja abierta. El número marcado en cada captura '
          + 'es el del paso. Si tocás la captura, ves la pantalla completa.',
        pasos: [
          {
            titulo: 'Elegí Ingreso o Egreso',
            texto:
              'Debajo del efectivo esperado están los botones Ingreso y Egreso. Una clase que cobraste es un '
              + 'ingreso; un arreglo que pagaste, un egreso.',
            imagen: {
              src: '/tutoriales/pasos/ingresos-egresos-y-anulaciones/01-ingreso-o-egreso',
              alt: 'Botones Ingreso y Egreso de la caja, marcados con el número 1.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/ingresos-egresos-y-anulaciones/zoom-01-ingreso-o-egreso', width: 307, height: 147 },
            },
          },
          {
            titulo: 'Elegí la categoría',
            texto:
              'En Categoría elegís de qué se trata: clases, torneos o cuotas en un ingreso; sueldos, insumos o '
              + 'mantenimiento en un egreso, entre otras.',
            imagen: {
              src: '/tutoriales/pasos/ingresos-egresos-y-anulaciones/02-categoria',
              alt: 'Campo Categoría de un egreso con Mantenimiento elegido, marcado con el número 2.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/ingresos-egresos-y-anulaciones/zoom-02-categoria', width: 481, height: 243 },
            },
          },
          {
            titulo: 'Cargá el monto y una nota',
            texto:
              'Método viene en efectivo: cambialo si lo pagaste de otra forma. Después cargá el monto. La nota '
              + 'es opcional y aclara lo que la categoría no dice.',
            imagen: {
              src: '/tutoriales/pasos/ingresos-egresos-y-anulaciones/03-monto-y-nota',
              alt: 'Campos Monto y Nota de un egreso, marcados con el número 3.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/ingresos-egresos-y-anulaciones/zoom-03-monto-y-nota', width: 481, height: 225 },
            },
          },
          {
            titulo: 'Registralo',
            texto:
              'Tocá Egreso, o Ingreso si estás cargando un ingreso, para guardarlo.',
            imagen: {
              src: '/tutoriales/pasos/ingresos-egresos-y-anulaciones/04-registrar',
              alt: 'Botón Egreso del formulario, marcado con el número 4.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/ingresos-egresos-y-anulaciones/zoom-04-registrar', width: 355, height: 123 },
            },
          },
          {
            titulo: 'Encontralo en Movimientos',
            texto:
              'El movimiento aparece en la lista Movimientos del turno, con su categoría, el método, la hora y '
              + 'la nota. Si es en efectivo, ya suma al efectivo esperado.',
            imagen: {
              src: '/tutoriales/pasos/ingresos-egresos-y-anulaciones/05-movimiento-en-la-lista',
              alt: 'Un ingreso por clases en la lista de movimientos, marcado con el número 5.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/ingresos-egresos-y-anulaciones/zoom-05-movimiento-en-la-lista', width: 310, height: 188 },
            },
          },
          {
            titulo: 'Tocá Anular si te equivocaste',
            texto:
              'Un movimiento cargado mal no se borra: debajo de su monto está Anular.',
            imagen: {
              src: '/tutoriales/pasos/ingresos-egresos-y-anulaciones/06-anular',
              alt: 'Monto de un egreso con el botón Anular, marcados con el número 6.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/ingresos-egresos-y-anulaciones/zoom-06-anular', width: 333, height: 123 },
            },
          },
          {
            titulo: 'Confirmá la anulación',
            texto:
              'Vibe registra un movimiento inverso por el mismo monto. Si querés, dejá el motivo en la nota y '
              + 'tocá Anular para confirmar.',
            imagen: {
              src: '/tutoriales/pasos/ingresos-egresos-y-anulaciones/07-confirmar-anulacion',
              alt: 'Aviso de la anulación y su nota, marcados con el número 7.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/ingresos-egresos-y-anulaciones/zoom-07-confirmar-anulacion', width: 509, height: 284 },
            },
          },
          {
            titulo: 'Mirá el movimiento anulado',
            texto:
              'El original queda en la lista, tachado y con la marca Anulado, junto a la hora de la anulación y'
              + ' su motivo. Ya no suma al efectivo esperado.',
            imagen: {
              src: '/tutoriales/pasos/ingresos-egresos-y-anulaciones/08-movimiento-anulado',
              alt: 'Egreso anulado en la lista de movimientos, marcado con el número 8.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/ingresos-egresos-y-anulaciones/zoom-08-movimiento-anulado', width: 298, height: 184 },
            },
          },
        ],
      },
      {
        tipo: 'parrafos',
        heading: 'Categorías, no un cajón de "otros"',
        parrafos: [
          'Cada movimiento se carga con su categoría propia (clases, sueldos, insumos y las demás), así '
            + 'que sabés al toque cuánto entró por cada una.',
          'El desglose por categoría de la caja y del reporte del mes sale solo, sin tener que releer '
            + 'cada nota.',
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
          'Las ventas no se anulan desde acá: se anulan en Vender, en Ventas de este turno, y así también '
          + 'vuelve el stock. Los cobros de reservas no aparecen en esta lista de movimientos.',
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
    datePublished: '2026-09-24',
    respuesta:
      'La pantalla de vender muestra tu catálogo en tarjetas: tocás un producto y se suma una unidad al '
      + 'carrito. Sumás o restás desde el carrito mismo, elegís el método de cobro y cobrás. La venta '
      + 'descuenta el stock de cada producto y registra un ingreso en la caja del turno, en el mismo paso. '
      + 'Vender por encima del stock disponible está permitido, con un aviso.',
    bloques: [
      {
        tipo: 'pasos',
        heading: 'Paso a paso, en Vender',
        intro:
          'Se hace desde Caja, en la pestaña Vender, con la caja abierta. El número marcado en cada captura'
          + ' es el del paso. Si tocás la captura, ves la pantalla completa.',
        pasos: [
          {
            titulo: 'Buscá el producto',
            texto:
              'Arriba del catálogo escribís el nombre del producto o tocás una categoría para ver solo esa.',
            imagen: {
              src: '/tutoriales/pasos/vender-desde-el-mostrador/01-buscar-o-filtrar',
              alt: 'Buscador y categorías del catálogo, marcados con el número 1.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/vender-desde-el-mostrador/zoom-01-buscar-o-filtrar', width: 451, height: 173 },
            },
          },
          {
            titulo: 'Tocá el producto',
            texto:
              'Cada toque suma una unidad al carrito, y la tarjeta muestra cuántas llevás.',
            imagen: {
              src: '/tutoriales/pasos/vender-desde-el-mostrador/02-tocar-un-producto',
              alt: 'Tarjeta del producto Agua 500 ml con una unidad en el carrito, marcada con el número 2.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/vender-desde-el-mostrador/zoom-02-tocar-un-producto', width: 294, height: 137 },
            },
          },
          {
            titulo: 'Revisá el carrito',
            texto:
              'A la derecha está el carrito. En cada línea sumás o restás unidades, y con la cruz la quitás '
              + 'entera.',
            imagen: {
              src: '/tutoriales/pasos/vender-desde-el-mostrador/03-carrito',
              alt: 'Líneas del carrito con sus cantidades, marcadas con el número 3.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/vender-desde-el-mostrador/zoom-03-carrito', width: 405, height: 303 },
            },
          },
          {
            titulo: 'Elegí el método',
            texto:
              'Debajo del total elegís cómo te pagaron. El botón de al lado agrega una nota a la venta.',
            imagen: {
              src: '/tutoriales/pasos/vender-desde-el-mostrador/04-metodo',
              alt: 'Total del carrito y selector de método, marcados con el número 4.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/vender-desde-el-mostrador/zoom-04-metodo', width: 429, height: 157 },
            },
          },
          {
            titulo: 'Cobrá',
            texto:
              'Tocá Cobrar, que ya muestra el total. La venta descuenta el stock y entra a la caja del turno en'
              + ' el mismo paso.',
            imagen: {
              src: '/tutoriales/pasos/vender-desde-el-mostrador/05-cobrar',
              alt: 'Botón Cobrar con el total, marcado con el número 5.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/vender-desde-el-mostrador/zoom-05-cobrar', width: 381, height: 137 },
            },
          },
          {
            titulo: 'Mirá la confirmación',
            texto:
              'Vibe te muestra el total cobrado, el método y lo que vendiste. Si algún producto quedó sin stock'
              + ' suficiente, también te lo avisa ahí.',
            imagen: {
              src: '/tutoriales/pasos/vender-desde-el-mostrador/06-venta-registrada',
              alt: 'Total cobrado y método de la venta registrada, marcados con el número 6.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/vender-desde-el-mostrador/zoom-06-venta-registrada', width: 505, height: 223 },
            },
          },
          {
            titulo: 'Anulá una venta si hace falta',
            texto:
              'Debajo del catálogo, en Ventas de este turno, cada venta tiene Anular. El stock vuelve y el '
              + 'ingreso de esa venta se anula.',
            imagen: {
              src: '/tutoriales/pasos/vender-desde-el-mostrador/07-anular-una-venta',
              alt: 'Una venta del turno con el botón Anular, marcada con el número 7.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/vender-desde-el-mostrador/zoom-07-anular-una-venta', width: 284, height: 146 },
            },
          },
        ],
      },
      {
        tipo: 'parrafos',
        heading: 'El sábado sin gaseosas, resuelto antes de que pase',
        parrafos: [
          'Cada venta hace las tres cosas en el mismo toque: cobra, descuenta el stock y entra a la caja '
            + 'del turno.',
          'Así el producto, el stock y la caja quedan sincronizados al momento, sin un paso aparte que '
            + 'dependa de acordarte más tarde.',
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
          + 'el historial del turno, no desaparece. Una venta que incluye el saldo de un turno no se puede '
          + 'anular.',
      },
      {
        question: '¿Necesito la caja abierta para vender?',
        answer:
          'Sí. Vender requiere un turno de caja abierto, porque cada venta registra su ingreso ahí.',
      },
      {
        question: '¿Puedo cobrar una venta con Mercado Pago?',
        answer:
          'No desde acá: Vibe registra el cobro en efectivo, transferencia, débito, crédito o QR que vos '
          + 'ya recibiste en el mostrador, no procesa el pago con tarjeta.',
      },
      {
        question: '¿Puedo cobrar el saldo de un turno desde Vender?',
        answer:
          'Sí. Arriba del catálogo están los turnos de hoy con saldo pendiente: tocás uno y se suma al '
          + 'carrito, junto con los productos que se lleven.',
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
    datePublished: '2026-09-24',
    respuesta:
      'Cada producto se carga con su nombre, categoría y precio. El control de stock es opcional: un '
      + 'servicio como el alquiler de una paleta no lo necesita, y queda marcado sin control de stock. '
      + 'A los que sí lo controlás les definís un aviso de stock bajo: cuando el stock llega a ese número, '
      + 'aparece marcado en la lista de productos y en el panel de control. Reponer stock carga la cantidad y el costo '
      + 'total, y ese gasto entra a la caja del turno abierto.',
    bloques: [
      {
        tipo: 'pasos',
        heading: 'Paso a paso, en Productos',
        intro:
          'Se hace desde Caja, en la pestaña Productos. El número marcado en cada captura es el del paso. '
          + 'Si tocás la captura, ves la pantalla completa.',
        pasos: [
          {
            titulo: 'Tocá Nuevo producto',
            texto:
              'Arriba a la derecha de Productos está el botón Nuevo producto.',
            imagen: {
              src: '/tutoriales/pasos/productos-y-stock/01-nuevo-producto',
              alt: 'Botón Nuevo producto, marcado con el número 1.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/productos-y-stock/zoom-01-nuevo-producto', width: 344, height: 135 },
            },
          },
          {
            titulo: 'Cargá nombre, categoría y precio',
            texto:
              'Escribí el nombre y la categoría, o tocá una de las sugeridas. Después cargá el precio de venta.',
            imagen: {
              src: '/tutoriales/pasos/productos-y-stock/02-nombre-categoria-precio',
              alt: 'Campos Nombre, Categoría y Precio de un producto nuevo, marcados con el número 2.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/productos-y-stock/zoom-02-nombre-categoria-precio', width: 481, height: 377 },
            },
          },
          {
            titulo: 'Elegí si controla stock',
            texto:
              'Con Controlar stock prendido, Vibe lleva la cuenta de las unidades, y en Avisar con stock bajo '
              + 'en ponés cuándo querés el aviso. Para un servicio, como el alquiler de una paleta, lo apagás. '
              + 'Tocá Guardar.',
            imagen: {
              src: '/tutoriales/pasos/productos-y-stock/03-controlar-stock',
              alt: 'Interruptor Controlar stock y aviso de stock bajo, marcados con el número 3.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/productos-y-stock/zoom-03-controlar-stock', width: 481, height: 189 },
            },
          },
          {
            titulo: 'Mirá el aviso de stock bajo',
            texto:
              'Cuando el stock llega al número del aviso, el producto aparece con la marca Stock bajo, en la '
              + 'lista y en su detalle. El panel de control también te avisa.',
            imagen: {
              src: '/tutoriales/pasos/productos-y-stock/04-stock-bajo',
              alt: 'Producto Grip con la marca Stock bajo, marcado con el número 4.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/productos-y-stock/zoom-04-stock-bajo', width: 260, height: 227 },
            },
          },
          {
            titulo: 'Elegí Reponer',
            texto:
              'En la fila del producto, tocá los tres puntos y elegí Reponer. Un producto nuevo arranca sin '
              + 'unidades: las cargás reponiendo.',
            imagen: {
              src: '/tutoriales/pasos/productos-y-stock/05-reponer',
              alt: 'Opción Reponer del menú de un producto, marcada con el número 5.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/productos-y-stock/zoom-05-reponer', width: 287, height: 161 },
            },
          },
          {
            titulo: 'Cargá la cantidad y el costo',
            texto:
              'Cargá cuántas unidades entraron y cuánto pagaste en total, elegí el método y confirmá. El costo '
              + 'entra como egreso a la caja del turno, por eso reponer necesita la caja abierta.',
            imagen: {
              src: '/tutoriales/pasos/productos-y-stock/06-cantidad-y-costo',
              alt: 'Campos Cantidad y Costo total de la reposición, marcados con el número 6.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/productos-y-stock/zoom-06-cantidad-y-costo', width: 481, height: 223 },
            },
          },
          {
            titulo: 'Ajustá después de contar',
            texto:
              'Si contaste y no coincide, elegí Ajustar en el mismo menú. Cargás la cantidad contada, ves la '
              + 'diferencia y elegís el motivo, como rotura o vencimiento. No mueve plata de la caja.',
            imagen: {
              src: '/tutoriales/pasos/productos-y-stock/07-ajustar',
              alt: 'Cantidad contada, diferencia y motivo del ajuste de stock, marcados con el número 7.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/productos-y-stock/zoom-07-ajustar', width: 481, height: 259 },
            },
          },
          {
            titulo: 'Desactivá lo que ya no vendés',
            texto:
              'Desactivar, en el mismo menú, saca el producto de Vender sin borrar su historial. Lo encontrás '
              + 'en Inactivos y lo reactivás cuando quieras.',
            imagen: {
              src: '/tutoriales/pasos/productos-y-stock/08-desactivar',
              alt: 'Opción Desactivar del menú de un producto, marcada con el número 8.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/productos-y-stock/zoom-08-desactivar', width: 287, height: 121 },
            },
          },
        ],
      },
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
          + 'después apagás Controlar stock.',
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
      'Un turno con saldo pendiente se cobra igual desde el panel: elegís el método real y queda '
      + 'registrado, sin pasar por Mercado Pago.',
    excerpt: 'Un turno sin cobrar se cobra igual desde el panel, con el método que usés en el momento.',
    datePublished: '2026-09-24',
    respuesta:
      'No todos los clientes pagan online: un turno cargado por teléfono, o uno al que todavía le falta'
      + ' el resto después de la seña, se cobra desde el mismo detalle de la reserva. Tocás Confirmar '
      + 'pago, elegís el método real (efectivo, transferencia, débito, crédito o QR / billetera) y '
      + 'confirmás. El cobro queda registrado sin que Mercado Pago intervenga.',
    bloques: [
      {
        tipo: 'pasos',
        heading: 'Paso a paso, desde la grilla de reservas',
        intro:
          'Entrá a Reservas desde el menú de la izquierda. El número marcado en cada captura es el del '
          + 'paso. Si tocás la captura, ves la pantalla completa.',
        pasos: [
          {
            titulo: 'Abrí el turno',
            texto:
              'En la grilla, tocá el turno que vas a cobrar para abrir su detalle.',
            imagen: {
              src: '/tutoriales/pasos/cobrar-una-reserva-en-el-mostrador/01-abrir-el-turno',
              alt: 'Turno de 18:30 a 20:00 en la columna de Pádel 1, marcado con el número 1.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/cobrar-una-reserva-en-el-mostrador/zoom-01-abrir-el-turno', width: 265, height: 153 },
            },
          },
          {
            titulo: 'Tocá Confirmar pago',
            texto:
              'Está abajo a la derecha del detalle y aparece mientras al turno le falta cobrar algo.',
            imagen: {
              src: '/tutoriales/pasos/cobrar-una-reserva-en-el-mostrador/02-confirmar-pago',
              alt: 'Botón Confirmar pago abajo del detalle de la reserva, marcado con el número 2.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/cobrar-una-reserva-en-el-mostrador/zoom-02-confirmar-pago', width: 264, height: 107 },
            },
          },
          {
            titulo: 'Mirá el saldo restante',
            texto:
              'Si el cliente ya pagó la seña, la ventana se llama Confirmar pago restante y te muestra solo lo '
              + 'que falta. La seña no se vuelve a cobrar.',
            imagen: {
              src: '/tutoriales/pasos/cobrar-una-reserva-en-el-mostrador/03-saldo-restante',
              alt: 'Saldo restante de la reserva en la ventana Confirmar pago restante, marcado con el número 3.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/cobrar-una-reserva-en-el-mostrador/zoom-03-saldo-restante', width: 469, height: 141 },
            },
          },
          {
            titulo: 'Si no pagó nada, elegí el tipo de cobro',
            texto:
              'En un turno sin seña paga, la ventana te deja elegir entre Pago completo y Seña. Si cobrás solo '
              + 'la seña, cargás su monto.',
            imagen: {
              src: '/tutoriales/pasos/cobrar-una-reserva-en-el-mostrador/04-tipo-de-cobro',
              alt: 'Selector Tipo de cobro con Pago completo elegido, marcado con el número 4.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/cobrar-una-reserva-en-el-mostrador/zoom-04-tipo-de-cobro', width: 481, height: 147 },
            },
          },
          {
            titulo: 'Elegí el método',
            texto:
              'Efectivo, transferencia, débito, crédito o QR / billetera: el que haya usado el cliente en el '
              + 'momento.',
            imagen: {
              src: '/tutoriales/pasos/cobrar-una-reserva-en-el-mostrador/05-metodo',
              alt: 'Lista de métodos de pago abierta en la ventana de cobro, marcada con el número 5.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/cobrar-una-reserva-en-el-mostrador/zoom-05-metodo', width: 490, height: 294 },
            },
          },
          {
            titulo: 'Confirmá el pago',
            texto:
              'Tocá Confirmar pago. Si fue en efectivo, el cobro suma al efectivo esperado de la caja.',
            imagen: {
              src: '/tutoriales/pasos/cobrar-una-reserva-en-el-mostrador/06-confirmar',
              alt: 'Botón Confirmar pago de la ventana de cobro, marcado con el número 6.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/cobrar-una-reserva-en-el-mostrador/zoom-06-confirmar', width: 304, height: 115 },
            },
          },
          {
            titulo: 'Revisá el estado del pago',
            texto:
              'Al abrir de nuevo el turno, el estado del pago dice Pago completo.',
            imagen: {
              src: '/tutoriales/pasos/cobrar-una-reserva-en-el-mostrador/07-pago-completo',
              alt: 'Estado del pago Pago completo en el detalle de la reserva, marcado con el número 7.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/cobrar-una-reserva-en-el-mostrador/zoom-07-pago-completo', width: 260, height: 119 },
            },
          },
        ],
      },
      {
        tipo: 'parrafos',
        heading: 'Lo que no pasó por Mercado Pago, igual queda registrado',
        parrafos: [
          'La reserva que te pidieron por teléfono, o el resto de una seña que te pagan en cancha, no '
            + 'tienen por qué quedar afuera de tus números solo porque no pasaron por un link de pago.',
          'Confirmando el pago desde el mismo turno, ese cobro entra a tu facturación y a tu reporte del '
            + 'mes exactamente igual que uno pagado online.',
        ],
      },
    ],
    faq: [
      {
        question: '¿Este cobro pasa por Mercado Pago?',
        answer:
          'No. Confirmar pago registra que vos cobraste en el mostrador. Mercado Pago solo interviene en lo'
          + ' que el cliente paga online al reservar.',
      },
      {
        question: '¿Puedo cobrar solo la seña y dejar el resto pendiente?',
        answer:
          'Sí, si el turno no tenía nada pagado: en Tipo de cobro elegís Seña y cargás el monto. El resto '
          + 'queda pendiente y lo cobrás después con el mismo botón.',
      },
      {
        question: '¿Este cobro entra a la caja del turno?',
        answer:
          'Si lo cobraste en efectivo, sí: suma al efectivo esperado. Con otro método no suma al efectivo, '
          + 'y la caja lo muestra aparte, en los cobros de reservas por otros medios.',
      },
      {
        question: '¿Qué pasa si el cliente ya había pagado la seña online?',
        answer:
          'Confirmar pago te pide entonces solo el resto: la seña ya cobrada por Mercado Pago no se vuelve '
          + 'a pedir.',
      },
      {
        question: '¿Puedo cobrar el saldo desde la caja, junto con lo que consumió?',
        answer:
          'Sí. En Vender, arriba del catálogo, aparecen los turnos de hoy con saldo pendiente. Tocás uno, '
          + 'se suma al carrito con los productos y cobrás todo junto.',
      },
    ],
    relacionados: ['grilla-de-reservas', 'abrir-y-cerrar-la-caja', 'vender-desde-el-mostrador'],
  },
  {
    slug: 'asistencia-de-cada-turno',
    title: 'La asistencia de cada turno: quién vino y quién faltó',
    seoTitle: 'Asistencia de cada turno en tu complejo | Vibe',
    metaDescription:
      'Cuando termina un turno marcás si el cliente se presentó o no, y esa marca arma sola las '
      + 'ausencias y la asistencia de cada cliente en Vibe.',
    excerpt: 'Al terminar cada turno marcás si el cliente vino, y su ficha suma las ausencias sola.',
    datePublished: '2026-11-16',
    respuesta:
      'Cuando pasa el horario de una reserva, su detalle dice Turno finalizado. Ahí marcás Se presentó '
      + 'o No se presentó. Cada No se presentó suma una ausencia a la ficha del cliente y baja su '
      + 'porcentaje de asistencia, que ves en la lista de Clientes y en su ficha, donde además figura '
      + 'cómo terminó cada una de sus últimas reservas.',
    bloques: [
      {
        tipo: 'pasos',
        heading: 'Paso a paso, de la grilla a la ficha del cliente',
        intro:
          'Empezás en Reservas y terminás en Clientes, los dos en el menú de la izquierda. El número '
          + 'marcado en cada captura es el del paso. Si tocás la captura, ves la pantalla completa.',
        pasos: [
          {
            titulo: 'Tocá un turno que ya terminó',
            texto:
              'En la grilla, buscá un turno cuyo horario ya pasó y tocalo para abrir su detalle.',
            imagen: {
              src: '/tutoriales/pasos/asistencia-de-cada-turno/01-turno-finalizado',
              alt: 'Turno de 17:00 a 18:30 en la columna de Pádel 4, ya terminado, marcado con el número 1.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/asistencia-de-cada-turno/zoom-01-turno-finalizado', width: 264, height: 152 },
            },
          },
          {
            titulo: 'Fijate el estado',
            texto:
              'En Estado de la reserva dice Turno finalizado: el horario ya pasó y todavía no marcaste si el '
              + 'cliente vino.',
            imagen: {
              src: '/tutoriales/pasos/asistencia-de-cada-turno/02-estado-de-la-reserva',
              alt: 'Estado de la reserva Turno finalizado en el detalle, marcado con el número 2.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/asistencia-de-cada-turno/zoom-02-estado-de-la-reserva', width: 260, height: 119 },
            },
          },
          {
            titulo: 'Marcá si vino',
            texto:
              'Abajo del detalle tocás Se presentó o No se presentó. Se presentó aparece cuando termina el '
              + 'horario. No se presentó también está antes, mientras la reserva sigue confirmada.',
            imagen: {
              src: '/tutoriales/pasos/asistencia-de-cada-turno/03-marcar-asistencia',
              alt: 'Botones No se presentó y Se presentó abajo del detalle de la reserva, marcados con el número 3.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/asistencia-de-cada-turno/zoom-03-marcar-asistencia', width: 333, height: 101 },
            },
          },
          {
            titulo: 'Mirá las ausencias en Clientes',
            texto:
              'En la lista de Clientes, Ausencias cuenta las veces que cada uno no se presentó, y Asistencia '
              + 'muestra qué parte de sus reservas cumplió, en verde, amarillo o rojo.',
            imagen: {
              src: '/tutoriales/pasos/asistencia-de-cada-turno/04-ausencias-y-asistencia',
              alt: 'Columnas Ausencias y Asistencia de un cliente en la lista de Clientes, marcadas con el número 4.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/asistencia-de-cada-turno/zoom-04-ausencias-y-asistencia', width: 365, height: 154 },
            },
          },
          {
            titulo: 'Abrí la ficha del cliente',
            texto:
              'Tocá el cliente: arriba de su ficha están sus ausencias y su asistencia.',
            imagen: {
              src: '/tutoriales/pasos/asistencia-de-cada-turno/05-ficha-del-cliente',
              alt: 'Ausencias y Asistencia en la ficha de un cliente, marcadas con el número 5.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/asistencia-de-cada-turno/zoom-05-ficha-del-cliente', width: 260, height: 171 },
            },
          },
          {
            titulo: 'Revisá turno por turno',
            texto:
              'En Reservas recientes ves cómo terminó cada una: Se presentó, No se presentó o Turno finalizado '
              + 'si quedó sin marcar.',
            imagen: {
              src: '/tutoriales/pasos/asistencia-de-cada-turno/06-turno-por-turno',
              alt: 'Reservas recientes del cliente con Se presentó y No se presentó, marcadas con el número 6.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/asistencia-de-cada-turno/zoom-06-turno-por-turno', width: 518, height: 229 },
            },
          },
        ],
      },
      {
        tipo: 'parrafos',
        heading: 'Qué cambia con cada marca',
        parrafos: [
          'No se presentó suma una ausencia en la ficha del cliente y baja su porcentaje de asistencia. Ese'
            + ' turno ya no se cobra desde el detalle, y lo que el cliente pagó de seña queda cobrado.',
          'Se presentó deja el turno registrado como cumplido, y así aparece en la ficha del cliente. Con '
            + 'el tiempo, la asistencia te muestra quién cumple siempre y a quién conviene pedirle seña.',
        ],
      },
    ],
    faq: [
      {
        question: '¿Tengo que marcar la asistencia de todos los turnos?',
        answer:
          'No es obligatorio. Un turno sin marcar queda como Turno finalizado y no cuenta como ausencia. Lo'
          + ' que baja la asistencia de un cliente es marcar No se presentó.',
      },
      {
        question: '¿Puedo cambiar la marca si me equivoqué?',
        answer:
          'Si marcaste Se presentó, desde el mismo detalle todavía podés cambiarlo a No se presentó. Un No '
          + 'se presentó, en cambio, ya no se cambia.',
      },
      {
        question: '¿Dónde veo cuántos turnos se pierden por ausencias?',
        answer:
          'En el Dashboard, la tarjeta Clientes muestra Ausencias: qué parte de los turnos terminados de '
          + 'los últimos 30 días se marcó como No se presentó.',
      },
      {
        question: '¿Puedo cobrar un turno marcado como No se presentó?',
        answer:
          'No. Confirmar pago deja de aparecer en ese turno, y lo que el cliente ya pagó de seña queda '
          + 'cobrado.',
      },
    ],
    relacionados: ['grilla-de-reservas', 'base-de-clientes', 'panel-de-control'],
    guia: {
      slug: 'como-llenar-los-horarios-vacios-de-un-complejo',
      texto: 'Cómo llenar los horarios vacíos de un complejo',
    },
  },
  {
    slug: 'cancelar-una-reserva-y-devolver',
    title: 'Cancelar una reserva y devolver la plata',
    seoTitle: 'Cancelar una reserva y devolver la seña | Vibe',
    metaDescription:
      'Cómo cancelar una reserva desde el panel de Vibe, qué pasa con la seña pagada con Mercado Pago y'
      + ' con lo cobrado a mano, y cuándo cancela el cliente.',
    excerpt: 'Cancelás un turno y lo pagado con Mercado Pago vuelve solo. Lo cobrado a mano lo devolvés vos.',
    datePublished: '2026-11-16',
    respuesta:
      'Abrís la reserva en la grilla, tocás Cancelar reserva y confirmás. Cuando cancelás vos, lo que '
      + 'el cliente pagó con Mercado Pago se le devuelve solo, al mismo medio con que pagó, y la reserva '
      + 'pasa a Reembolsado. Lo que cobraste en el mostrador aparece como Monto a devolver: se lo das en '
      + 'mano y lo marcás como realizado. El cliente también puede cancelar desde el link de su reserva, '
      + 'y recupera lo que pagó si cancela dentro de las horas de cancelación que configuraste.',
    bloques: [
      {
        tipo: 'pasos',
        heading: 'Paso a paso, de la grilla a la devolución',
        intro:
          'Empezás en Reservas, seguís en Clientes y terminás en Configuración, los tres en el menú de la '
          + 'izquierda. El número marcado en cada captura es el del paso. Si tocás la captura, ves la '
          + 'pantalla completa.',
        pasos: [
          {
            titulo: 'Fijate cómo pagó',
            texto:
              'Tocá el turno en la grilla. En el detalle, Estado del pago dice si el cliente pagó la seña o el '
              + 'total, y al lado del monto figura el medio: Mercado Pago o el que anotaste en el mostrador.',
            imagen: {
              src: '/tutoriales/pasos/cancelar-una-reserva-y-devolver/01-como-pago',
              alt: 'Estado del pago Seña pagada y la seña pagada con Mercado Pago en el detalle de la reserva, marcados con el número 1.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/cancelar-una-reserva-y-devolver/zoom-01-como-pago', width: 311, height: 220 },
            },
          },
          {
            titulo: 'Tocá Cancelar reserva',
            texto:
              'Está abajo del detalle. Si el turno tiene muchas acciones, la encontrás en el menú de los tres '
              + 'puntos, arriba a la derecha.',
            imagen: {
              src: '/tutoriales/pasos/cancelar-una-reserva-y-devolver/02-cancelar-reserva',
              alt: 'Botón Cancelar reserva abajo del detalle de la reserva, marcado con el número 2.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/cancelar-una-reserva-y-devolver/zoom-02-cancelar-reserva', width: 260, height: 107 },
            },
          },
          {
            titulo: 'Anotá el motivo',
            texto:
              'El motivo es opcional. Queda en las notas de la reserva, así después sabés por qué se canceló.',
            imagen: {
              src: '/tutoriales/pasos/cancelar-una-reserva-y-devolver/03-motivo',
              alt: 'Campo Motivo de cancelación con el texto Se suspende por lluvia, marcado con el número 3.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/cancelar-una-reserva-y-devolver/zoom-03-motivo', width: 509, height: 157 },
            },
          },
          {
            titulo: 'Confirmá la cancelación',
            texto:
              'Tocá Confirmar cancelación. El horario queda libre en la grilla y al cliente le llega un aviso '
              + 'con lo que pasa con su plata. La cancelación no se deshace.',
            imagen: {
              src: '/tutoriales/pasos/cancelar-una-reserva-y-devolver/04-confirmar-cancelacion',
              alt: 'Botón Confirmar cancelación del diálogo, marcado con el número 4.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/cancelar-una-reserva-y-devolver/zoom-04-confirmar-cancelacion', width: 350, height: 107 },
            },
          },
          {
            titulo: 'Lo de Mercado Pago vuelve solo',
            texto:
              'Si pagó con Mercado Pago, Vibe le devuelve ese pago al mismo medio. La reserva cancelada se abre'
              + ' desde la ficha del cliente, en Clientes: Estado del pago dice Reembolsado y abajo figura el '
              + 'monto devuelto.',
            imagen: {
              src: '/tutoriales/pasos/cancelar-una-reserva-y-devolver/05-reembolsado',
              alt: 'Estado del pago Reembolsado y el monto reembolsado de una seña pagada con Mercado Pago, marcados con el número 5.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/cancelar-una-reserva-y-devolver/zoom-05-reembolsado', width: 331, height: 270 },
            },
          },
          {
            titulo: 'Lo cobrado a mano lo devolvés vos',
            texto:
              'Si cobraste en el mostrador, en efectivo o con otro medio, el detalle dice Falta devolver y te '
              + 'muestra el Monto a devolver.',
            imagen: {
              src: '/tutoriales/pasos/cancelar-una-reserva-y-devolver/06-monto-a-devolver',
              alt: 'Monto a devolver de una seña cobrada en efectivo, con la indicación de devolvérsela al cliente, marcado con el número 6.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/cancelar-una-reserva-y-devolver/zoom-06-monto-a-devolver', width: 518, height: 135 },
            },
          },
          {
            titulo: 'Marcá la devolución',
            texto:
              'Cuando le diste la plata, tocá Marcar devolución realizada y confirmá el monto.',
            imagen: {
              src: '/tutoriales/pasos/cancelar-una-reserva-y-devolver/07-marcar-devolucion',
              alt: 'Botón Marcar devolución realizada abajo del detalle de la reserva cancelada, marcado con el número 7.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/cancelar-una-reserva-y-devolver/zoom-07-marcar-devolucion', width: 331, height: 107 },
            },
          },
          {
            titulo: 'Definí hasta cuándo cancela el cliente',
            texto:
              'En Configuración, Horas de cancelación dice con cuántas horas de anticipación el cliente puede '
              + 'cancelar y recuperar lo que pagó. Al terminar, tocá Guardar cambios.',
            imagen: {
              src: '/tutoriales/pasos/cancelar-una-reserva-y-devolver/08-horas-de-cancelacion',
              alt: 'Campo Horas de cancelación en la configuración del complejo, marcado con el número 8.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/cancelar-una-reserva-y-devolver/zoom-08-horas-de-cancelacion', width: 358, height: 153 },
            },
          },
        ],
      },
      {
        tipo: 'parrafos',
        heading: 'Cuando cancela el cliente',
        parrafos: [
          'Al reservar, el cliente recibe un link para cancelar. Si cancela con más anticipación que tus '
            + 'horas de cancelación, lo que pagó con Mercado Pago se le devuelve solo, y lo que pagó en el '
            + 'mostrador te queda como monto a devolver, igual que cuando cancelás vos.',
          'Si cancela más tarde, la página se lo avisa antes de confirmar: la reserva se cancela y lo que '
            + 'pagó no se devuelve. Desde el panel, en cambio, la devolución corre siempre, sin importar cuánto'
            + ' falte para el turno.',
        ],
      },
    ],
    faq: [
      {
        question: '¿Qué pasa con la seña si cancelo yo la reserva?',
        answer:
          'Se devuelve siempre, sin importar cuánto falte para el turno. Lo pagado con Mercado Pago vuelve '
          + 'solo al medio con que pagó el cliente, y lo cobrado en el mostrador te queda como monto a '
          + 'devolver.',
      },
      {
        question: '¿Cuánto tarda en llegarle la devolución de Mercado Pago?',
        answer:
          'Vibe la envía a Mercado Pago al cancelar, y se acredita en los próximos días hábiles. Si Mercado'
          + ' Pago tarda en tomarla, la reserva dice Reembolso en curso hasta que se completa.',
      },
      {
        question: '¿Qué pasa si el cliente cancela fuera de plazo?',
        answer:
          'La reserva se cancela y lo que pagó no se devuelve. La página de cancelación se lo avisa antes '
          + 'de que confirme.',
      },
      {
        question: '¿Dónde encuentro una reserva cancelada?',
        answer:
          'Deja de verse en la grilla y el horario queda libre. La abrís desde la ficha del cliente, en '
          + 'Clientes, dentro de Reservas recientes.',
      },
    ],
    relacionados: ['configuracion-del-complejo', 'cobrar-una-reserva-en-el-mostrador', 'turnos-fijos'],
    guia: {
      slug: 'como-llenar-los-horarios-vacios-de-un-complejo',
      texto: 'Cómo llenar los horarios vacíos de un complejo',
    },
  },
  {
    slug: 'turnos-fijos',
    title: 'Turnos fijos: el turno de todas las semanas, cargado una vez',
    seoTitle: 'Turnos fijos semanales para tu complejo | Vibe',
    metaDescription:
      'Cómo cargar un turno fijo semanal en Vibe, cancelar una sola fecha cuando el cliente no viene y '
      + 'terminar el turno fijo desde la fecha que elijas.',
    excerpt: 'Cargás el turno de todas las semanas una vez, y cancelás una fecha o todo el turno fijo desde la grilla.',
    datePublished: '2026-11-16',
    respuesta:
      'Un turno fijo se carga como una reserva nueva con Repetir todas las semanas activado. Elegís '
      + 'hasta cuándo se repite, y cada semana ocupa la cancha en la grilla con el nombre del cliente, '
      + 'sin que lo vuelvas a cargar. El día del turno, esa fecha pasa a ser una reserva y se cobra como '
      + 'cualquier otra. Si el cliente no viene una semana, cancelás solo esa fecha, y si deja de venir, '
      + 'cancelás el turno fijo desde la fecha que elijas.',
    bloques: [
      {
        tipo: 'pasos',
        heading: 'Paso a paso, en la grilla de reservas',
        intro:
          'Entrá a Reservas desde el menú de la izquierda. El número marcado en cada captura es el del '
          + 'paso. Si tocás la captura, ves la pantalla completa.',
        pasos: [
          {
            titulo: 'Activá Repetir todas las semanas',
            texto:
              'Tocá Nueva reserva y elegí la fecha del primer turno, la cancha, la duración y el horario. Abajo'
              + ' activás Repetir todas las semanas.',
            imagen: {
              src: '/tutoriales/pasos/turnos-fijos/01-repetir-todas-las-semanas',
              alt: 'Interruptor Repetir todas las semanas en el formulario de Nueva reserva, marcado con el número 1.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/turnos-fijos/zoom-01-repetir-todas-las-semanas', width: 513, height: 83 },
            },
          },
          {
            titulo: 'Elegí hasta cuándo',
            texto:
              'En Hasta cuándo elegís Sin fecha de fin, Hasta una fecha o Por N semanas, de 2 a 52.',
            imagen: {
              src: '/tutoriales/pasos/turnos-fijos/02-hasta-cuando',
              alt: 'Opciones Sin fecha de fin, Hasta y Por N semanas de Hasta cuándo, marcadas con el número 2.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/turnos-fijos/zoom-02-hasta-cuando', width: 335, height: 208 },
            },
          },
          {
            titulo: 'Revisá el resumen',
            texto:
              'Después de cargar los datos del cliente, el último paso muestra qué día se repite, el horario, '
              + 'la cancha y desde cuándo. Cada turno se cobra como una reserva normal.',
            imagen: {
              src: '/tutoriales/pasos/turnos-fijos/03-resumen',
              alt: 'Resumen del turno fijo con el día, el horario, la cancha, desde y hasta, marcado con el número 3.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/turnos-fijos/zoom-03-resumen', width: 509, height: 229 },
            },
          },
          {
            titulo: 'Creá el turno fijo',
            texto:
              'Tocá Crear turno fijo. Si la cancha ya está ocupada algún día, Vibe te muestra esos días y '
              + 'elegís, para cada uno, si no se reserva o si ese turno va en otra cancha libre.',
            imagen: {
              src: '/tutoriales/pasos/turnos-fijos/04-crear-turno-fijo',
              alt: 'Botón Crear turno fijo en el último paso de Nueva reserva, marcado con el número 4.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/turnos-fijos/zoom-04-crear-turno-fijo', width: 320, height: 111 },
            },
          },
          {
            titulo: 'Encontralo en la grilla',
            texto:
              'Cada semana el turno fijo ocupa su cancha en la grilla, con el ícono de repetir y el nombre del '
              + 'cliente. Ese horario ya no se ofrece en tu página de reservas.',
            imagen: {
              src: '/tutoriales/pasos/turnos-fijos/05-en-la-grilla',
              alt: 'Turno fijo de un cliente en la columna de Pádel 3 con el ícono de repetir, marcado con el número 5.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/turnos-fijos/zoom-05-en-la-grilla', width: 264, height: 147 },
            },
          },
          {
            titulo: 'Abrí una fecha',
            texto:
              'Tocá el turno: el detalle dice qué día y a qué hora se repite, para qué cliente y en qué cancha.'
              + ' El día del turno, esa fecha pasa a ser una reserva como cualquier otra.',
            imagen: {
              src: '/tutoriales/pasos/turnos-fijos/06-repite',
              alt: 'Repite Todos los lunes a las 20:00 en el detalle de un turno fijo, marcado con el número 6.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/turnos-fijos/zoom-06-repite', width: 321, height: 121 },
            },
          },
          {
            titulo: 'Cancelá solo esa fecha',
            texto:
              'Si el cliente avisa que una semana no viene, tocá Cancelar solo esta fecha.',
            imagen: {
              src: '/tutoriales/pasos/turnos-fijos/07-cancelar-solo-esta-fecha',
              alt: 'Botón Cancelar solo esta fecha abajo del detalle del turno fijo, marcado con el número 7.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/turnos-fijos/zoom-07-cancelar-solo-esta-fecha', width: 322, height: 107 },
            },
          },
          {
            titulo: 'Confirmá',
            texto:
              'Tocá Cancelar esta fecha. Ese día la cancha queda libre y el turno fijo sigue en las demás '
              + 'semanas.',
            imagen: {
              src: '/tutoriales/pasos/turnos-fijos/08-liberar-esta-fecha',
              alt: 'Botón Cancelar esta fecha del diálogo Liberar esta fecha, marcado con el número 8.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/turnos-fijos/zoom-08-liberar-esta-fecha', width: 502, height: 185 },
            },
          },
          {
            titulo: 'Terminá el turno fijo',
            texto:
              'Para darlo de baja, abrí la primera fecha que ya no va y tocá Cancelar turno fijo. Esa fecha y '
              + 'las siguientes quedan libres. Si alguna ya tiene su reserva sin pago, se cancela; si tiene un '
              + 'pago, se mantiene.',
            imagen: {
              src: '/tutoriales/pasos/turnos-fijos/09-cancelar-turno-fijo',
              alt: 'Botón Cancelar turno fijo del diálogo Cancelar el turno fijo, marcado con el número 9.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/turnos-fijos/zoom-09-cancelar-turno-fijo', width: 491, height: 195 },
            },
          },
        ],
      },
    ],
    faq: [
      {
        question: '¿Cómo se cobra un turno fijo?',
        answer:
          'Cada fecha se cobra como una reserva normal. El día del turno, esa fecha pasa a ser una reserva '
          + 'y la cobrás desde su detalle, como cualquier otra.',
      },
      {
        question: '¿Qué pasa si la cancha ya está ocupada alguna semana?',
        answer:
          'Al crearlo, Vibe te muestra los días ocupados y elegís para cada uno si no se reserva o si ese '
          + 'turno va en otra cancha libre. Si el turno fijo no tiene fecha de fin, revisa las fechas del '
          + 'próximo año.',
      },
      {
        question: '¿Puedo cancelar la fecha de hoy?',
        answer:
          'Sí. Cuando esa fecha ya es una reserva, Cancelar solo esta fecha cancela esa reserva como '
          + 'cualquier otra, con la devolución que corresponda. El turno fijo sigue en las demás semanas.',
      },
      {
        question: '¿Al cliente le llega una confirmación cada semana?',
        answer:
          'No. Las fechas de un turno fijo no le mandan una confirmación de reserva por semana.',
      },
    ],
    relacionados: ['grilla-de-reservas', 'cancelar-una-reserva-y-devolver', 'base-de-clientes', 'canchas-libres'],
    guia: {
      slug: 'como-llenar-los-horarios-vacios-de-un-complejo',
      texto: 'Cómo llenar los horarios vacíos de un complejo',
    },
  },
  {
    slug: 'canchas-libres',
    title: 'Canchas libres: publicá un horario y que los jugadores se anoten',
    seoTitle: 'Canchas libres con inscripción y pago online | Vibe',
    metaDescription:
      'Cómo publicar canchas libres en Vibe: elegís canchas, cupo y precio por persona, los jugadores '
      + 'se anotan y pagan desde tu página, y si cancelás se les devuelve.',
    excerpt: 'Publicás un horario con cupo y precio, los jugadores se anotan y pagan desde tu página, y ves quién viene.',
    datePublished: '2026-11-16',
    respuesta:
      'Canchas libres es un evento con horario, canchas, cupo y precio por persona. Lo creás en '
      + 'Eventos, y las canchas que elegís quedan bloqueadas para otras reservas durante ese horario. El '
      + 'evento aparece en tu página: cada jugador se anota con sus datos y paga su lugar con Mercado '
      + 'Pago. En el detalle del evento ves quién se anotó y quién pagó. Si cancelás el evento, las '
      + 'canchas quedan libres y a cada persona que pagó se le devuelve todo por Mercado Pago.',
    bloques: [
      {
        tipo: 'pasos',
        heading: 'Paso a paso, de Eventos a tu página',
        intro:
          'Entrá a Eventos desde el menú de la izquierda. El número marcado en cada captura es el del paso.'
          + ' Si tocás la captura, ves la pantalla completa.',
        pasos: [
          {
            titulo: 'Tocá Crear evento',
            texto:
              'En la pestaña Canchas libres, tocá Crear evento.',
            imagen: {
              src: '/tutoriales/pasos/canchas-libres/01-crear-evento',
              alt: 'Botón Crear evento en la pestaña Canchas libres de Eventos, marcado con el número 1.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/canchas-libres/zoom-01-crear-evento', width: 342, height: 125 },
            },
          },
          {
            titulo: 'Poné nombre, fecha y horario',
            texto:
              'Escribí el nombre del evento, por ejemplo Americano de los sábados, y elegí el deporte, la fecha'
              + ' y el horario, desde y hasta. Un evento dura entre 30 minutos y 12 horas.',
            imagen: {
              src: '/tutoriales/pasos/canchas-libres/02-nombre-fecha-horario',
              alt: 'Campos Nombre del evento, Deporte, Fecha, Desde y Hasta del formulario Crear evento, marcados con el número 2.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/canchas-libres/zoom-02-nombre-fecha-horario', width: 517, height: 361 },
            },
          },
          {
            titulo: 'Elegí las canchas',
            texto:
              'Viene marcado Todas las canchas. Si querés usar solo algunas, desmarcalo y elegí cuáles. Durante'
              + ' el evento, esas canchas no reciben reservas.',
            imagen: {
              src: '/tutoriales/pasos/canchas-libres/03-canchas',
              alt: 'Lista de canchas con Pádel 3 y Pádel 4 elegidas en Crear evento, marcada con el número 3.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/canchas-libres/zoom-03-canchas', width: 517, height: 207 },
            },
          },
          {
            titulo: 'Definí el cupo y el precio',
            texto:
              'En Cupo ponés cuántas personas se pueden anotar. En Precio por persona, lo que paga cada una por'
              + ' su lugar. El costo del servicio de pago se suma aparte.',
            imagen: {
              src: '/tutoriales/pasos/canchas-libres/04-cupo-y-precio',
              alt: 'Campos Cupo y Precio por persona de Crear evento, marcados con el número 4.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/canchas-libres/zoom-04-cupo-y-precio', width: 517, height: 155 },
            },
          },
          {
            titulo: 'Creá el evento',
            texto:
              'Tocá Crear evento. Si alguna cancha ya tiene una reserva, un bloqueo, un turno fijo u otro '
              + 'evento en ese horario, Vibe te dice en cuál, para que cambies el horario o la saques del evento.',
            imagen: {
              src: '/tutoriales/pasos/canchas-libres/05-crear',
              alt: 'Botón Crear evento al pie del formulario, marcado con el número 5.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/canchas-libres/zoom-05-crear', width: 364, height: 147 },
            },
          },
          {
            titulo: 'Mirá las canchas tomadas en la grilla',
            texto:
              'En Reservas, ese día cada cancha del evento muestra Canchas libres con el nombre del evento y el'
              + ' horario. Nadie puede reservarlas en ese horario.',
            imagen: {
              src: '/tutoriales/pasos/canchas-libres/06-en-la-grilla',
              alt: 'Bloques Canchas libres de un evento en las columnas de Pádel 3 y Pádel 4 de la grilla, marcados con el número 6.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/canchas-libres/zoom-06-en-la-grilla', width: 464, height: 177 },
            },
          },
          {
            titulo: 'Los jugadores se anotan desde tu página',
            texto:
              'El evento aparece en tu página, en Canchas libres, con la fecha, el horario y las canchas. Cada '
              + 'jugador toca Inscribirme, carga su nombre, apellido y teléfono, y paga su lugar con Mercado '
              + 'Pago.',
            imagen: {
              src: '/tutoriales/pasos/canchas-libres/07-pagina-publica',
              alt: 'Página pública de un evento con la fecha, el horario, el nombre y las canchas, marcada con el número 7.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/canchas-libres/zoom-07-pagina-publica', width: 419, height: 327 },
            },
          },
          {
            titulo: 'Mirá quién se anotó',
            texto:
              'Tocá el evento en Eventos. El detalle muestra el horario, las canchas, el cupo y la lista de '
              + 'participantes, con el teléfono de cada uno y si ya pagó.',
            imagen: {
              src: '/tutoriales/pasos/canchas-libres/08-participantes',
              alt: 'Participantes de un evento con el primer jugador y la marca Pagó, marcados con el número 8.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/canchas-libres/zoom-08-participantes', width: 518, height: 159 },
            },
          },
          {
            titulo: 'Cancelá el evento si hace falta',
            texto:
              'Abajo del detalle tocá Cancelar evento y confirmá con Sí, cancelar evento. Las canchas quedan '
              + 'libres y a cada persona que pagó se le devuelve todo por Mercado Pago, servicio incluido. Si '
              + 'dejó un email, le llega el aviso.',
            imagen: {
              src: '/tutoriales/pasos/canchas-libres/09-cancelar-evento',
              alt: 'Botón Sí, cancelar evento del diálogo Cancelar el evento, marcado con el número 9.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/canchas-libres/zoom-09-cancelar-evento', width: 496, height: 205 },
            },
          },
        ],
      },
    ],
    faq: [
      {
        question: '¿Se pueden reservar esas canchas durante el evento?',
        answer:
          'No. Mientras dura el evento, las canchas que elegiste quedan bloqueadas para otras reservas. Si '
          + 'cancelás el evento, vuelven a quedar libres.',
      },
      {
        question: '¿Un jugador puede cancelar su lugar?',
        answer:
          'Sí, desde la página de su inscripción, con Cancelar mi lugar. Si cancela dentro del plazo de '
          + 'cancelación de tu complejo, lo que pagó se le devuelve solo. Fuera de ese plazo, no se devuelve.',
      },
      {
        question: '¿Qué pasa cuando se llena el cupo?',
        answer:
          'El evento figura como Completo en tu página y nadie más se puede anotar.',
      },
    ],
    relacionados: ['torneos', 'turnos-fijos', 'grilla-de-reservas'],
  },
  {
    slug: 'torneos',
    title: 'Torneos: inscripción, zonas, cuadro y resultados',
    seoTitle: 'Torneos con inscripción online, zonas y cuadro | Vibe',
    metaDescription:
      'Cómo armar un torneo en Vibe: categorías con cupo y precio por pareja, inscripción y pago desde '
      + 'tu página, sorteo de zonas y cuadro, partidos y resultados.',
    excerpt: 'Armás las categorías, las parejas se inscriben y pagan desde tu página, y Vibe arma zonas y cuadro con los resultados que cargás.',
    datePublished: '2026-11-16',
    respuesta:
      'Un torneo se crea en Eventos, en la pestaña Torneos, con sus días y una o más categorías. Cada '
      + 'categoría tiene su cupo de parejas, su precio por pareja y su formato: zonas y cuadro o '
      + 'eliminación directa. Las parejas se inscriben desde tu página y paga una sola persona por la '
      + 'pareja con Mercado Pago. Cuando cerrás la inscripción, sorteás y Vibe arma las zonas y el cuadro'
      + ' con las parejas que pagaron. Programás cada partido en una cancha, que queda ocupada en la '
      + 'grilla, cargás los resultados, y los jugadores siguen las posiciones y el cuadro desde tu '
      + 'página.',
    bloques: [
      {
        tipo: 'pasos',
        heading: 'Paso a paso, de la inscripción a la final',
        intro:
          'Entrá a Eventos desde el menú de la izquierda. El número marcado en cada captura es el del paso.'
          + ' Si tocás la captura, ves la pantalla completa.',
        pasos: [
          {
            titulo: 'Tocá Crear torneo',
            texto:
              'Abrí la pestaña Torneos y tocá Crear torneo.',
            imagen: {
              src: '/tutoriales/pasos/torneos/01-crear-torneo',
              alt: 'Botón Crear torneo en la pestaña Torneos de Eventos, marcado con el número 1.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/torneos/zoom-01-crear-torneo', width: 339, height: 125 },
            },
          },
          {
            titulo: 'Cargá los datos del torneo',
            texto:
              'Poné el nombre, el deporte, el primer día y el último día. Un torneo puede durar hasta 60 días.',
            imagen: {
              src: '/tutoriales/pasos/torneos/02-datos-del-torneo',
              alt: 'Campos Nombre del torneo, Deporte, Primer día y Último día de Crear torneo, marcados con el número 2.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/torneos/zoom-02-datos-del-torneo', width: 517, height: 361 },
            },
          },
          {
            titulo: 'Agregá una categoría',
            texto:
              'Tocá Siguiente y después Agregar categoría. Cada categoría lleva su nombre, el formato, el cupo '
              + 'de parejas y el precio por pareja. Paga una sola persona por la pareja, y el costo del servicio '
              + 'de pago se suma aparte.',
            imagen: {
              src: '/tutoriales/pasos/torneos/03-categoria',
              alt: 'Campos Nombre, Formato, Cupo y Precio por pareja de una categoría, marcados con el número 3.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/torneos/zoom-03-categoria', width: 517, height: 245 },
            },
          },
          {
            titulo: 'Definí cómo se juega',
            texto:
              'Tocá Siguiente. Con zonas y cuadro elegís cuántas parejas van por zona y cuántas pasan al '
              + 'cuadro. También el formato de partido, los games por set, la duración de cada partido y si el '
              + 'tercer set es un súper tie-break. Tocá Guardar categoría y sumá las que necesites.',
            imagen: {
              src: '/tutoriales/pasos/torneos/04-como-se-juega',
              alt: 'Opciones de Cómo se juega: parejas por zona, pasan por zona, formato de partido, games, duración y súper tie-break, marcadas con el número 4.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/torneos/zoom-04-como-se-juega', width: 517, height: 415 },
            },
          },
          {
            titulo: 'Revisá y creá el torneo',
            texto:
              'Tocá Siguiente para ver el resumen del torneo y de cada categoría. Si está todo bien, tocá Crear'
              + ' torneo.',
            imagen: {
              src: '/tutoriales/pasos/torneos/05-revisar-y-crear',
              alt: 'Botón Crear torneo debajo del resumen de las categorías, marcado con el número 5.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/torneos/zoom-05-revisar-y-crear', width: 485, height: 235 },
            },
          },
          {
            titulo: 'Mirá las parejas inscriptas',
            texto:
              'Abrí el torneo desde la lista. En Parejas ves cada pareja con sus teléfonos y lo que pagó, y el '
              + 'cupo dice cuántas se anotaron.',
            imagen: {
              src: '/tutoriales/pasos/torneos/06-parejas',
              alt: 'Primera pareja inscripta con sus teléfonos y lo que pagó, marcada con el número 6.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/torneos/zoom-06-parejas', width: 306, height: 119 },
            },
          },
          {
            titulo: 'Sorteá las zonas y el cuadro',
            texto:
              'Cuando quieras cerrar la inscripción, tocá Sortear. Vibe arma las zonas y el cuadro con las '
              + 'parejas que ya pagaron. Con Ordenar cabezas de serie elegís el orden de las mejores; las que no '
              + 'ordenes van al azar. Podés sortear de nuevo mientras no haya resultados cargados.',
            imagen: {
              src: '/tutoriales/pasos/torneos/07-sortear',
              alt: 'Botones Ordenar cabezas de serie y Sortear de la tarjeta Sorteo, marcados con el número 7.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/torneos/zoom-07-sortear', width: 514, height: 111 },
            },
          },
          {
            titulo: 'Seguí las posiciones de cada zona',
            texto:
              'En Zonas, cada zona tiene su tabla con partidos jugados, ganados y perdidos, sets y games. Las '
              + 'parejas que pasan al cuadro dicen Clasifica.',
            imagen: {
              src: '/tutoriales/pasos/torneos/08-zonas',
              alt: 'Tabla de posiciones de la Zona A con dos parejas que clasifican, marcada con el número 8.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/torneos/zoom-08-zonas', width: 506, height: 332 },
            },
          },
          {
            titulo: 'Mirá el cuadro',
            texto:
              'En Cuadro ves cada ronda hasta la final. Las parejas que clasifican de las zonas y las que ganan'
              + ' cada partido avanzan solas.',
            imagen: {
              src: '/tutoriales/pasos/torneos/09-cuadro',
              alt: 'Cuadro con las semifinales y la final de una categoría, marcado con el número 9.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/torneos/zoom-09-cuadro', width: 514, height: 305 },
            },
          },
          {
            titulo: 'Programá cada partido',
            texto:
              'En Partidos tocá Programar y elegí el día, la hora, la cancha y la duración. Si esa cancha ya '
              + 'está ocupada en ese horario, Vibe te avisa.',
            imagen: {
              src: '/tutoriales/pasos/torneos/10-programar',
              alt: 'Campos Día, Hora, Cancha y Duración del diálogo Programar partido, marcados con el número 10.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/torneos/zoom-10-programar', width: 517, height: 297 },
            },
          },
          {
            titulo: 'Cargá el resultado',
            texto:
              'Tocá Cargar resultado. Podés marcar solo quién ganó, cargar el resultado por sets o marcar W.O. '
              + 'si una pareja no se presentó. El ganador pasa de ronda.',
            imagen: {
              src: '/tutoriales/pasos/torneos/11-cargar-resultado',
              alt: 'Opciones Quién ganó, Cargar resultado por sets y W.O. del diálogo Resultado del partido, marcadas con el número 11.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/torneos/zoom-11-cargar-resultado', width: 461, height: 271 },
            },
          },
          {
            titulo: 'Encontrá los partidos en la grilla',
            texto:
              'Cada partido programado ocupa su cancha en la grilla de Reservas, con la categoría y las dos '
              + 'parejas. Nadie más puede reservar esa cancha en ese horario.',
            imagen: {
              src: '/tutoriales/pasos/torneos/12-en-la-grilla',
              alt: 'Partido de torneo en la columna de Pádel 3 de la grilla, marcado con el número 12.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/torneos/zoom-12-en-la-grilla', width: 264, height: 149 },
            },
          },
          {
            titulo: 'Los jugadores lo siguen desde tu página',
            texto:
              'En tu página, el torneo muestra cada categoría con su formato. Las parejas se inscriben con '
              + 'Inscribir mi pareja y, después del sorteo, todos ven las zonas, los partidos, el cuadro y los '
              + 'resultados.',
            imagen: {
              src: '/tutoriales/pasos/torneos/13-pagina-publica',
              alt: 'Tabla de posiciones de la Zona A en la página pública del torneo, marcada con el número 13.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/torneos/zoom-13-pagina-publica', width: 446, height: 344 },
            },
          },
        ],
      },
    ],
    faq: [
      {
        question: '¿Cómo se cobra la cancha de cada partido?',
        answer:
          'Al crear la categoría podés poner un monto de cancha por partido, por pareja. Ese monto no se '
          + 'paga online: se cobra en el complejo el día del partido. También lo podés agregar después con '
          + 'Agregar monto de cancha.',
      },
      {
        question: '¿Qué pasa si cancelo el torneo o una categoría?',
        answer:
          'Se cancelan sus partidos y las canchas quedan libres. A cada pareja que pagó se le devuelve todo'
          + ' por Mercado Pago, servicio incluido, y le llega un aviso por email si dejó uno.',
      },
      {
        question: '¿Puedo corregir un resultado?',
        answer:
          'Sí, con Corregir resultado, mientras el partido siguiente de esas parejas no tenga resultado '
          + 'cargado.',
      },
    ],
    relacionados: ['canchas-libres', 'grilla-de-reservas', 'turnos-fijos'],
  },
  {
    slug: 'agregar-y-editar-canchas',
    title: 'Canchas: agregá, editá y desactivá las canchas de tu complejo',
    seoTitle: 'Cómo agregar y editar canchas en tu complejo | Vibe',
    metaDescription:
      'Cómo agregar una cancha en Vibe con su nombre, deporte, tipo y descripción, editarla, '
      + 'desactivarla por un tiempo o eliminarla, y dónde cargar sus precios.',
    excerpt: 'Cargás cada cancha con su deporte y su tipo, y después la editás, la desactivás o la eliminás.',
    datePublished: '2026-11-16',
    respuesta:
      'En Canchas tocás Agregar cancha y cargás el nombre, el deporte, el tipo de cancha y, si querés, '
      + 'la superficie, la iluminación y una descripción para los jugadores. Al crearla se abren sus '
      + 'precios, que cargás por día y por horario. Después la editás desde los tres puntos de su fila. '
      + 'Si una cancha queda fuera de uso por un tiempo, la desactivás: deja de aparecer en tu página y '
      + 'en la grilla, y sus reservas siguen vigentes. Si ya no la vas a usar, la eliminás.',
    bloques: [
      {
        tipo: 'pasos',
        heading: 'Paso a paso, en Canchas',
        intro:
          'Entrá a Canchas desde el menú de la izquierda. El número marcado en cada captura es el del paso.'
          + ' Si tocás la captura, ves la pantalla completa.',
        pasos: [
          {
            titulo: 'Tocá Agregar cancha',
            texto:
              'Arriba a la derecha de Canchas, tocá Agregar cancha.',
            imagen: {
              src: '/tutoriales/pasos/agregar-y-editar-canchas/01-agregar-cancha',
              alt: 'Botón Agregar cancha arriba a la derecha de Canchas, marcado con el número 1.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/agregar-y-editar-canchas/zoom-01-agregar-cancha', width: 323, height: 145 },
            },
          },
          {
            titulo: 'Poné el nombre, el deporte y el tipo',
            texto:
              'Escribí el nombre con el que la vas a reconocer en la grilla, elegí el deporte y si es techada, '
              + 'descubierta o semi cubierta.',
            imagen: {
              src: '/tutoriales/pasos/agregar-y-editar-canchas/02-nombre-deporte-y-tipo',
              alt: 'Campos Nombre, Deporte y Tipo de cancha del formulario Agregar cancha, marcados con el número 2.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/agregar-y-editar-canchas/zoom-02-nombre-deporte-y-tipo', width: 515, height: 317 },
            },
          },
          {
            titulo: 'Sumá los detalles',
            texto:
              'La superficie, la iluminación y la descripción son opcionales. La descripción la leen los '
              + 'jugadores en tu página, y tiene hasta 200 caracteres.',
            imagen: {
              src: '/tutoriales/pasos/agregar-y-editar-canchas/03-superficie-luz-y-descripcion',
              alt: 'Campos Superficie, Tiene iluminación y Descripción del formulario, marcados con el número 3.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/agregar-y-editar-canchas/zoom-03-superficie-luz-y-descripcion', width: 515, height: 259 },
            },
          },
          {
            titulo: 'Tocá Crear',
            texto:
              'Tocá Crear. La cancha aparece activa en la lista.',
            imagen: {
              src: '/tutoriales/pasos/agregar-y-editar-canchas/04-crear',
              alt: 'Botón Crear al pie del formulario Agregar cancha, marcado con el número 4.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/agregar-y-editar-canchas/zoom-04-crear', width: 265, height: 123 },
            },
          },
          {
            titulo: 'Cargá sus precios',
            texto:
              'Al crearla se abren sus precios. Cargás un precio por día y, si querés, franjas con otro precio.'
              + ' Hasta que tenga precios, no se ofrece en tu página de reservas.',
            imagen: {
              src: '/tutoriales/pasos/agregar-y-editar-canchas/05-precios-de-la-cancha',
              alt: 'Ventana Precios de la cancha nueva con la aclaración Los precios son por hora, marcada con el número 5.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/agregar-y-editar-canchas/zoom-05-precios-de-la-cancha', width: 388, height: 222 },
            },
          },
          {
            titulo: 'Editala cuando quieras',
            texto:
              'Tocá los tres puntos de su fila y elegí Editar. Se abre el mismo formulario: cambiás lo que '
              + 'necesites y tocás Guardar. En ese menú, Precios abre sus precios.',
            imagen: {
              src: '/tutoriales/pasos/agregar-y-editar-canchas/06-editar',
              alt: 'Opción Editar del menú de tres puntos de una cancha, marcada con el número 6.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/agregar-y-editar-canchas/zoom-06-editar', width: 260, height: 171 },
            },
          },
          {
            titulo: 'Desactivala con el interruptor',
            texto:
              'En la columna Estado, el interruptor dice si la cancha está activa. Si la cerrás por un tiempo, '
              + 'por ejemplo por un arreglo, tocalo.',
            imagen: {
              src: '/tutoriales/pasos/agregar-y-editar-canchas/07-estado',
              alt: 'Interruptor de la columna Estado de una cancha, marcado con el número 7.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/agregar-y-editar-canchas/zoom-07-estado', width: 353, height: 115 },
            },
          },
          {
            titulo: 'Confirmá',
            texto:
              'Tocá Desactivar. Deja de aparecer en tu página de reservas y en la grilla, y no se le cargan '
              + 'reservas nuevas. Las que ya tiene siguen vigentes. Para volver a usarla, tocás el interruptor '
              + 'otra vez.',
            imagen: {
              src: '/tutoriales/pasos/agregar-y-editar-canchas/08-desactivar',
              alt: 'Botón Desactivar del aviso que pregunta si desactivar la cancha, marcado con el número 8.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/agregar-y-editar-canchas/zoom-08-desactivar', width: 481, height: 243 },
            },
          },
          {
            titulo: 'Eliminala si ya no la usás',
            texto:
              'En los tres puntos, tocá Eliminar y confirmá. No se puede deshacer. Si la cancha tiene reservas,'
              + ' canchas libres o turnos fijos por delante, Vibe no la deja eliminar hasta que los canceles.',
            imagen: {
              src: '/tutoriales/pasos/agregar-y-editar-canchas/09-eliminar',
              alt: 'Botón Eliminar del aviso Eliminar cancha, marcado con el número 9.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/agregar-y-editar-canchas/zoom-09-eliminar', width: 462, height: 243 },
            },
          },
        ],
      },
      {
        tipo: 'parrafos',
        heading: 'Los precios de cada cancha',
        parrafos: [
          'Cómo cargar el precio de cada día y una franja más cara para la noche está en el tutorial de <a '
            + 'href="/tutoriales/precios-por-horario">precios por horario</a>.',
        ],
      },
    ],
    faq: [
      {
        question: '¿Qué diferencia hay entre desactivar y eliminar una cancha?',
        answer:
          'Desactivarla la saca de tu página y de la grilla por un tiempo, y la volvés a activar con el '
          + 'mismo interruptor. Sus reservas siguen vigentes. Eliminarla la saca para siempre, y solo se '
          + 'puede si no tiene reservas, canchas libres ni turnos fijos por delante.',
      },
      {
        question: '¿Puedo cerrar un horario de una cancha sin desactivarla?',
        answer:
          'Sí. Desde la grilla de Reservas, con Bloquear horario, cerrás un día y un horario puntual, y el '
          + 'resto de la cancha sigue disponible.',
      },
      {
        question: '¿Dónde ven los jugadores la descripción?',
        answer:
          'En tu página de reservas, junto al nombre de cada cancha, cuando eligen el horario.',
      },
    ],
    relacionados: ['precios-por-horario', 'grilla-de-reservas', 'configuracion-del-complejo'],
  },
  {
    slug: 'bloquear-un-cliente',
    title: 'Bloquear un cliente: cuidá tus horarios de las reservas online',
    seoTitle: 'Cómo bloquear a un cliente para que no reserve online | Vibe',
    metaDescription:
      'Cómo bloquear a un cliente desde Clientes en Vibe para que no reserve online en tu complejo, qué'
      + ' ve cuando intenta reservar y cómo desbloquearlo.',
    excerpt: 'Desde la base de clientes bloqueás a alguien para que no reserve online, y lo desbloqueás cuando quieras.',
    datePublished: '2026-11-16',
    respuesta:
      'Bloquear a un cliente cuida tus horarios: mientras esté bloqueado, no puede reservar online ni '
      + 'anotarse en canchas libres o torneos desde tu página. Lo hacés desde Clientes, en los tres '
      + 'puntos de su fila, con Bloquear. Si intenta pagar una reserva, la página le pide que se '
      + 'comunique con el complejo. Las reservas que ya tenía siguen igual. Para levantar el bloqueo, '
      + 'tocás Desbloquear en el mismo menú.',
    bloques: [
      {
        tipo: 'pasos',
        heading: 'Paso a paso, en Clientes',
        intro:
          'Entrá a Clientes desde el menú de la izquierda. El número marcado en cada captura es el del '
          + 'paso. Si tocás la captura, ves la pantalla completa.',
        pasos: [
          {
            titulo: 'Buscá al cliente',
            texto:
              'Escribí su nombre en el buscador de Clientes.',
            imagen: {
              src: '/tutoriales/pasos/bloquear-un-cliente/01-buscar',
              alt: 'Buscador de Clientes con un nombre escrito y el cliente encontrado abajo, marcado con el número 1.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/bloquear-un-cliente/zoom-01-buscar', width: 417, height: 183 },
            },
          },
          {
            titulo: 'Abrí su menú',
            texto:
              'En su fila, tocá los tres puntos de la columna Acciones.',
            imagen: {
              src: '/tutoriales/pasos/bloquear-un-cliente/02-menu-del-cliente',
              alt: 'Botón de tres puntos en la columna Acciones de un cliente, marcado con el número 2.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/bloquear-un-cliente/zoom-02-menu-del-cliente', width: 299, height: 139 },
            },
          },
          {
            titulo: 'Tocá Bloquear',
            texto:
              'Tocá Bloquear. En el mismo menú tenés WhatsApp y Llamar, si antes querés hablar con el cliente.',
            imagen: {
              src: '/tutoriales/pasos/bloquear-un-cliente/03-bloquear',
              alt: 'Opción Bloquear del menú del cliente, debajo de WhatsApp y Llamar, marcada con el número 3.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/bloquear-un-cliente/zoom-03-bloquear', width: 295, height: 179 },
            },
          },
          {
            titulo: 'Confirmá',
            texto:
              'Tocá Bloquear en el aviso. Desde ese momento no puede hacer reservas online en tu complejo.',
            imagen: {
              src: '/tutoriales/pasos/bloquear-un-cliente/04-confirmar-bloqueo',
              alt: 'Botón Bloquear del aviso que dice No podrá hacer reservas online en tu complejo, marcado con el número 4.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/bloquear-un-cliente/zoom-04-confirmar-bloqueo', width: 468, height: 223 },
            },
          },
          {
            titulo: 'Lo ves en su ficha',
            texto:
              'Su ficha dice Bloqueado debajo del nombre, y en la lista su fila se ve más tenue.',
            imagen: {
              src: '/tutoriales/pasos/bloquear-un-cliente/05-bloqueado',
              alt: 'Nombre del cliente con la etiqueta Bloqueado en su ficha, marcado con el número 5.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/bloquear-un-cliente/zoom-05-bloqueado', width: 349, height: 172 },
            },
          },
          {
            titulo: 'Qué ve el cliente',
            texto:
              'Si intenta reservar desde tu página, elige el turno y carga sus datos. Al tocar Pagar de manera '
              + 'segura le aparece Tu cuenta está bloqueada. Contactá al complejo. No se le cobra nada.',
            imagen: {
              src: '/tutoriales/pasos/bloquear-un-cliente/06-lo-que-ve',
              alt: 'Aviso Tu cuenta está bloqueada. Contactá al complejo, en la página de reservas, marcado con el número 6.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/bloquear-un-cliente/zoom-06-lo-que-ve', width: 457, height: 160 },
            },
          },
          {
            titulo: 'Desbloquealo cuando quieras',
            texto:
              'En los mismos tres puntos, la opción ahora dice Desbloquear.',
            imagen: {
              src: '/tutoriales/pasos/bloquear-un-cliente/07-desbloquear',
              alt: 'Opción Desbloquear del menú del cliente, marcada con el número 7.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/bloquear-un-cliente/zoom-07-desbloquear', width: 298, height: 179 },
            },
          },
          {
            titulo: 'Confirmá',
            texto:
              'Tocá Desbloquear. Desde ese momento puede volver a reservar online.',
            imagen: {
              src: '/tutoriales/pasos/bloquear-un-cliente/08-confirmar-desbloqueo',
              alt: 'Botón Desbloquear del aviso que dice Podrá volver a hacer reservas online, marcado con el número 8.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/bloquear-un-cliente/zoom-08-confirmar-desbloqueo', width: 494, height: 223 },
            },
          },
        ],
      },
    ],
    faq: [
      {
        question: '¿Qué pasa con las reservas que ya tenía?',
        answer:
          'Siguen como estaban. Bloquear no cancela nada, solo frena las reservas nuevas. Si querés liberar'
          + ' un turno, lo cancelás desde la grilla.',
      },
      {
        question: '¿Puedo cargarle una reserva yo desde el panel?',
        answer:
          'No mientras esté bloqueado. Si querés darle un turno, desbloquealo primero.',
      },
      {
        question: '¿Al cliente le llega un aviso cuando lo bloqueo?',
        answer:
          'No. Lo ve recién si intenta reservar online o anotarse en canchas libres o un torneo: la página '
          + 'le pide que se comunique con el complejo.',
      },
    ],
    relacionados: ['base-de-clientes', 'asistencia-de-cada-turno', 'grilla-de-reservas'],
  },
  {
    slug: 'perfil-del-complejo-y-link-publico',
    title: 'Perfil del complejo y link público: lo que ven los jugadores',
    seoTitle: 'Cómo armar el perfil de tu complejo y tu link | Vibe',
    metaDescription:
      'Cómo cargar la portada, el logo, los datos y los servicios de tu complejo en Vibe, copiar el '
      + 'link de tu página de reservas y qué ven los jugadores.',
    excerpt: 'La portada, el logo, los datos y los servicios de tu complejo, y el link que compartís con los jugadores.',
    datePublished: '2026-11-16',
    respuesta:
      'El perfil del complejo es lo que ven los jugadores cuando abren tu link. Lo armás en '
      + 'Configuración, pestaña General: portada, logo, nombre, URL pública, descripción, dirección, '
      + 'teléfono, email, Instagram y servicios, y lo guardás con Guardar cambios. Los horarios se cargan'
      + ' en la pestaña Horarios. Tu link está arriba de todo en el Dashboard, con el botón Copiar '
      + 'enlace, y los jugadores lo pasan desde Compartir, en tu misma página.',
    bloques: [
      {
        tipo: 'pasos',
        heading: 'Paso a paso, de Configuración a tu página',
        intro:
          'Empezás en Configuración, pasás por el Dashboard y terminás en tu página de reservas. El número '
          + 'marcado en cada captura es el del paso. Si tocás la captura, ves la pantalla completa.',
        pasos: [
          {
            titulo: 'Tocá la portada',
            texto:
              'En Configuración, la pestaña General abre con la portada y el logo, puestos como se ven en tu '
              + 'página. Tocá la portada para elegir una imagen. Puede ser JPG, PNG o WebP, de hasta 5 MB.',
            imagen: {
              src: '/tutoriales/pasos/perfil-del-complejo-y-link-publico/01-portada',
              alt: 'Portada del complejo arriba de la pestaña General, marcada con el número 1.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/perfil-del-complejo-y-link-publico/zoom-01-portada', width: 509, height: 345 },
            },
          },
          {
            titulo: 'Tocá el logo',
            texto:
              'El logo va sobre la esquina de la portada. Tocalo para cambiarlo. Para quitar una imagen, pasá '
              + 'por encima y tocá el tacho que aparece arriba a la derecha.',
            imagen: {
              src: '/tutoriales/pasos/perfil-del-complejo-y-link-publico/02-logo',
              alt: 'Logo del complejo sobre la esquina de la portada, marcado con el número 2.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/perfil-del-complejo-y-link-publico/zoom-02-logo', width: 309, height: 209 },
            },
          },
          {
            titulo: 'Recortala y guardá',
            texto:
              'Antes de subirla, movés la imagen y usás Zoom para elegir qué se ve. Después tocá Guardar.',
            imagen: {
              src: '/tutoriales/pasos/perfil-del-complejo-y-link-publico/03-recortar',
              alt: 'Botón Guardar de la ventana Recortá la imagen, debajo del control de Zoom, marcado con el número 3.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/perfil-del-complejo-y-link-publico/zoom-03-recortar', width: 516, height: 567 },
            },
          },
          {
            titulo: 'Revisá el nombre y la URL pública',
            texto:
              'El nombre es el que ven los jugadores. La URL pública es la última parte del link de tu página. '
              + 'Si la cambiás, los links y códigos QR que ya compartiste dejan de funcionar.',
            imagen: {
              src: '/tutoriales/pasos/perfil-del-complejo-y-link-publico/04-nombre-y-url',
              alt: 'Campos Nombre del complejo y URL pública, marcados con el número 4.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/perfil-del-complejo-y-link-publico/zoom-04-nombre-y-url', width: 511, height: 211 },
            },
          },
          {
            titulo: 'Escribí la descripción',
            texto:
              'Contá cómo es tu complejo: las canchas, los servicios y qué lo hace distinto. Tiene hasta 600 '
              + 'caracteres.',
            imagen: {
              src: '/tutoriales/pasos/perfil-del-complejo-y-link-publico/05-descripcion',
              alt: 'Campo Descripción con el contador de caracteres, marcado con el número 5.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/perfil-del-complejo-y-link-publico/zoom-05-descripcion', width: 491, height: 183 },
            },
          },
          {
            titulo: 'Cargá la dirección y el contacto',
            texto:
              'Escribí la dirección y elegila entre las sugerencias. El teléfono, el email y el Instagram '
              + 'aparecen en tu página, y el teléfono es el WhatsApp al que te escriben los jugadores.',
            imagen: {
              src: '/tutoriales/pasos/perfil-del-complejo-y-link-publico/06-direccion-y-contacto',
              alt: 'Campos Dirección, Teléfono, Email de contacto e Instagram, marcados con el número 6.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/perfil-del-complejo-y-link-publico/zoom-06-direccion-y-contacto', width: 495, height: 351 },
            },
          },
          {
            titulo: 'Marcá los servicios',
            texto:
              'Tildá lo que tiene tu complejo, como estacionamiento, vestuarios, bar o parrilla. Se muestran en'
              + ' tu página para que un jugador sepa qué encuentra en el club.',
            imagen: {
              src: '/tutoriales/pasos/perfil-del-complejo-y-link-publico/07-servicios',
              alt: 'Lista de servicios con casillas, algunos tildados, marcada con el número 7.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/perfil-del-complejo-y-link-publico/zoom-07-servicios', width: 511, height: 571 },
            },
          },
          {
            titulo: 'Guardá los cambios',
            texto:
              'Al final de General, tocá Guardar cambios. La portada y el logo se guardan solos al recortarlos.',
            imagen: {
              src: '/tutoriales/pasos/perfil-del-complejo-y-link-publico/08-guardar',
              alt: 'Botón Guardar cambios al final de la pestaña General, marcado con el número 8.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/perfil-del-complejo-y-link-publico/zoom-08-guardar', width: 305, height: 113 },
            },
          },
          {
            titulo: 'Copiá tu link',
            texto:
              'En el Dashboard, arriba de todo, está el link de tu página. Tocá Copiar enlace y pegalo en tu '
              + 'WhatsApp, tu Instagram o donde quieras.',
            imagen: {
              src: '/tutoriales/pasos/perfil-del-complejo-y-link-publico/09-copiar-enlace',
              alt: 'Link de la página de reservas con el botón Copiar enlace en el Dashboard, marcados con el número 9.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/perfil-del-complejo-y-link-publico/zoom-09-copiar-enlace', width: 435, height: 91 },
            },
          },
          {
            titulo: 'Así lo ven los jugadores',
            texto:
              'Al abrir tu link ven la portada, el logo, el nombre, la dirección y cómo contactarte. Debajo '
              + 'están tus canchas, los horarios de cada día y los turnos para reservar.',
            imagen: {
              src: '/tutoriales/pasos/perfil-del-complejo-y-link-publico/10-pagina-publica',
              alt: 'Nombre, dirección, teléfono, email e Instagram del complejo en su página pública, marcados con el número 10.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/perfil-del-complejo-y-link-publico/zoom-10-pagina-publica', width: 457, height: 343 },
            },
          },
          {
            titulo: 'Lo pueden compartir',
            texto:
              'Con Compartir, en el celular se abren las opciones para compartir del teléfono, y en la '
              + 'computadora se copia el link.',
            imagen: {
              src: '/tutoriales/pasos/perfil-del-complejo-y-link-publico/11-compartir',
              alt: 'Botón Compartir debajo de la portada en la página pública, marcado con el número 11.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/perfil-del-complejo-y-link-publico/zoom-11-compartir', width: 415, height: 137 },
            },
          },
          {
            titulo: 'Y ven tus servicios',
            texto:
              'Al final de la página aparecen los servicios que marcaste en el paso 7.',
            imagen: {
              src: '/tutoriales/pasos/perfil-del-complejo-y-link-publico/12-servicios',
              alt: 'Servicios del complejo al pie de su página pública, marcados con el número 12.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/perfil-del-complejo-y-link-publico/zoom-12-servicios', width: 507, height: 185 },
            },
          },
        ],
      },
      {
        tipo: 'parrafos',
        heading: 'Los horarios de tu página',
        parrafos: [
          'Los días y horarios que ven los jugadores salen de la pestaña Horarios de Configuración. Cómo '
            + 'cargarlos está en el tutorial de <a href="/tutoriales/configuracion-del-complejo">configuración '
            + 'del complejo</a>.',
        ],
      },
    ],
    faq: [
      {
        question: '¿Puedo cambiar la URL de mi página?',
        answer:
          'Sí, desde URL pública en la pestaña General. Si la cambiás, los links y códigos QR que ya '
          + 'compartiste dejan de funcionar, así que conviene elegirla una vez y dejarla.',
      },
      {
        question: '¿Qué medida tienen que tener las imágenes?',
        answer:
          'El logo se ve mejor cuadrado, de 512 por 512 píxeles, y la portada de 1280 por 720. Pueden ser '
          + 'JPG, PNG o WebP de hasta 5 MB, y antes de subirlas elegís qué parte se ve.',
      },
      {
        question: '¿Los jugadores necesitan una cuenta para usar el link?',
        answer:
          'No. Entran a tu link, eligen cancha y horario y reservan. No necesitan app ni cuenta.',
      },
    ],
    relacionados: ['panel-de-control', 'configuracion-del-complejo', 'conectar-mercado-pago'],
  },
  {
    slug: 'conectar-mercado-pago',
    title: 'Conectá Mercado Pago y definí tu política de cancelación',
    seoTitle: 'Cómo conectar Mercado Pago a tu complejo en Vibe | Vibe',
    metaDescription:
      'Cómo conectar tu cuenta de Mercado Pago en Vibe para cobrar la seña al reservar, y cómo elegir '
      + 'el porcentaje de seña y las horas para cancelar sin cargo.',
    excerpt: 'Conectás tu cuenta una vez y la seña de cada reserva online entra directo a tu Mercado Pago.',
    datePublished: '2026-11-16',
    respuesta:
      'En Configuración, pestaña Cobros, tocás Conectar Mercado Pago y autorizás la conexión en la '
      + 'página de Mercado Pago. Al volver, Cobros dice Mercado Pago conectado, y desde ese momento tu '
      + 'página toma reservas online: el cliente paga la seña al reservar y la plata entra directo a tu '
      + 'cuenta. Mientras no esté conectado, tu página muestra tu WhatsApp para que te escriban. En la '
      + 'pestaña General elegís el porcentaje de seña y las horas de cancelación, la ventana en la que el'
      + ' cliente puede cancelar sin cargo.',
    bloques: [
      {
        tipo: 'pasos',
        heading: 'Paso a paso, en Configuración',
        intro:
          'Entrá a Configuración desde el menú de la izquierda. El número marcado en cada captura es el del'
          + ' paso. Si tocás la captura, ves la pantalla completa.',
        pasos: [
          {
            titulo: 'Abrí Cobros',
            texto:
              'En la columna de pestañas de Configuración, tocá Cobros.',
            imagen: {
              src: '/tutoriales/pasos/conectar-mercado-pago/01-cobros',
              alt: 'Pestaña Cobros de Configuración, marcada con el número 1.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/conectar-mercado-pago/zoom-01-cobros', width: 317, height: 119 },
            },
          },
          {
            titulo: 'Tocá Conectar Mercado Pago',
            texto:
              'Si tu cuenta no está conectada, Cobros dice Mercado Pago no conectado. Tocá Conectar Mercado '
              + 'Pago: se abre la página de Mercado Pago para que autorices la conexión con tu cuenta.',
            imagen: {
              src: '/tutoriales/pasos/conectar-mercado-pago/02-conectar',
              alt: 'Botón Conectar Mercado Pago debajo del estado Mercado Pago no conectado, marcado con el número 2.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/conectar-mercado-pago/zoom-02-conectar', width: 496, height: 283 },
            },
          },
          {
            titulo: 'Mientras tanto, te escriben por WhatsApp',
            texto:
              'Sin Mercado Pago conectado, tu página no toma reservas online. Muestra Reservá por WhatsApp con '
              + 'tu teléfono, y debajo tus canchas.',
            imagen: {
              src: '/tutoriales/pasos/conectar-mercado-pago/03-sin-conectar',
              alt: 'Recuadro Reservá por WhatsApp de la página pública de un complejo sin Mercado Pago, marcado con el número 3.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/conectar-mercado-pago/zoom-03-sin-conectar', width: 483, height: 231 },
            },
          },
          {
            titulo: 'Listo, quedó conectado',
            texto:
              'Cuando autorizás, volvés a Vibe, que te avisa que Mercado Pago quedó conectado. En Cobros ves '
              + 'Mercado Pago conectado con el ID de tu cuenta. Desde ese momento tu página toma reservas online '
              + 'y la seña entra directo a tu cuenta.',
            imagen: {
              src: '/tutoriales/pasos/conectar-mercado-pago/04-conectado',
              alt: 'Estado Mercado Pago conectado con el ID de la cuenta, marcado con el número 4.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/conectar-mercado-pago/zoom-04-conectado', width: 468, height: 153 },
            },
          },
          {
            titulo: 'Elegí la seña y las horas de cancelación',
            texto:
              'En la pestaña General, el Porcentaje de seña es la parte del turno que se cobra al reservar '
              + 'online. Las Horas de cancelación son las horas antes del turno hasta las que el cliente puede '
              + 'cancelar sin cargo, de 1 a 168.',
            imagen: {
              src: '/tutoriales/pasos/conectar-mercado-pago/05-sena-y-cancelacion',
              alt: 'Campos Porcentaje de seña y Horas de cancelación de la pestaña General, marcados con el número 5.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/conectar-mercado-pago/zoom-05-sena-y-cancelacion', width: 511, height: 167 },
            },
          },
          {
            titulo: 'Guardá los cambios',
            texto:
              'Tocá Guardar cambios. El porcentaje nuevo vale para las reservas que entran desde ese momento.',
            imagen: {
              src: '/tutoriales/pasos/conectar-mercado-pago/06-guardar',
              alt: 'Botón Guardar cambios al final de la pestaña General, marcado con el número 6.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/conectar-mercado-pago/zoom-06-guardar', width: 305, height: 113 },
            },
          },
          {
            titulo: 'Si alguna vez la desconectás',
            texto:
              'En Cobros, tocá Desconectar y confirmá. Si tenés reservas próximas que pueden necesitar Mercado '
              + 'Pago para cobrar o devolver una seña, vas a poder desconectarla cuando hayan pasado.',
            imagen: {
              src: '/tutoriales/pasos/conectar-mercado-pago/07-desconectar',
              alt: 'Botón Desconectar debajo del estado Mercado Pago conectado, marcado con el número 7.',
              width: 1440,
              height: 900,
              recorte: { src: '/tutoriales/pasos/conectar-mercado-pago/zoom-07-desconectar', width: 443, height: 229 },
            },
          },
        ],
      },
      {
        tipo: 'parrafos',
        heading: 'Lo que sigue',
        parrafos: [
          'Cuándo te queda disponible la plata de cada seña lo elegís en tu cuenta de Mercado Pago: está en'
            + ' <a href="/tutoriales/plazo-de-acreditacion-mercado-pago">cambiar el plazo de acreditación</a>. '
            + 'Qué pasa con la seña cuando se cancela una reserva está en <a href="/tutoriales/cancelar-una-'
            + 'reserva-y-devolver">cancelar una reserva y devolver la plata</a>, y el resto de la pantalla, en '
            + '<a href="/tutoriales/configuracion-del-complejo">configuración del complejo</a>.',
        ],
      },
    ],
    faq: [
      {
        question: '¿Cuánto me cuesta conectar Mercado Pago?',
        answer:
          'Usar Vibe es gratis y conectar tu cuenta también. Mercado Pago te cobra su propia comisión por '
          + 'cada cobro, que depende del plazo de acreditación que elegiste en tu cuenta.',
      },
      {
        question: '¿Qué pasa si cancelo la conexión a mitad de camino?',
        answer:
          'No se conecta nada y Vibe te lo avisa. Podés volver a intentarlo cuando quieras desde Cobros, '
          + 'con Conectar Mercado Pago.',
      },
      {
        question: '¿Puedo conectar otra cuenta de Mercado Pago?',
        answer:
          'Mientras tengas reservas activas, solo podés volver a conectar la misma cuenta de antes. Cuando '
          + 'ya no tengas reservas activas, podés conectar otra.',
      },
    ],
    relacionados: ['configuracion-del-complejo', 'plazo-de-acreditacion-mercado-pago', 'cancelar-una-reserva-y-devolver'],
    guia: {
      slug: 'cuanto-cobra-mercadopago-por-una-sena',
      texto: 'Cuánto cobra Mercado Pago por una seña',
    },
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
