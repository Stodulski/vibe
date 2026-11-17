import { courtFee } from './tournamentsCourtFee';
import { createSteps } from './tournamentsCreate';
import { tournamentsForm } from './tournamentsForm';
import { tournamentsTraits } from './tournamentsTraits';

const plural = (count: number, one: string, many: string) => (count === 1 ? `1 ${one}` : `${String(count)} ${many}`);

export const tournaments = {
  title: 'Torneos',
  create: 'Crear torneo',
  // The grid bar: "Torneo · {torneo} · {categoría}".
  blockLabel: (name: string) => `Torneo · ${name}`,
  blockLabelCompact: 'Torneo',
  upcoming: 'Próximos',
  past: 'Pasados',
  listLabel: 'Torneos',
  noUpcoming: 'No hay torneos próximos',
  noUpcomingDescription:
    'Creá un torneo con sus categorías: las parejas se anotan y pagan online, y vos armás zonas y cuadro.',
  noPast: 'No hay torneos pasados',
  loadError: 'No pudimos cargar los torneos',
  detailLoadError: 'No pudimos cargar el torneo',
  backToList: 'Volver a eventos',
  categoriesCount: (count: number) => plural(count, 'categoría', 'categorías'),
  pairsCount: (count: number) => plural(count, 'pareja', 'parejas'),
  pairsTaken: (taken: number, capacity: number) => `${String(taken)} de ${String(capacity)} parejas`,
  days: (from: string, to: string) => (from === to ? from : `${from} al ${to}`),
  cancelledBadge: 'Cancelado',
  pricePerPair: (price: string) => `${price} por pareja`,
  format: { elimination: 'Eliminación directa', groups: 'Zonas y cuadro' },
  // The phone's summary line: "Zonas", "Elim.".
  formatShort: { elimination: 'Elim.', groups: 'Zonas' },
  categoryStatus: { open: 'Inscripción abierta', closed: 'Sorteo hecho', cancelled: 'Cancelada' },
  traits: tournamentsTraits,

  // Create form
  createSteps,
  form: tournamentsForm,
  validation: {
    endBeforeStart: 'El último día no puede ser anterior al primero',
    tooLong: 'El torneo puede durar hasta 60 días',
    categoriesRequired: 'Agregá al menos una categoría',
    categoriesMax: 'Un torneo puede tener hasta 20 categorías',
    modalityRequired: 'Elegí la modalidad: Caballeros, Damas o Mixto',
    levelRequired: 'Elegí el nivel de la categoría',
    sumaRange: 'La suma va de 2 a 18',
    startsOnRequired: 'Elegí la fecha de la categoría',
    startsOnRange: 'La fecha tiene que caer dentro de los días del torneo',
    startsAtRequired: 'Elegí la hora de inicio',
    prizesMax: 'Los premios pueden tener hasta 200 caracteres',
    categoryDuplicate: 'Ya hay una categoría con esa modalidad y nivel o suma en este torneo.',
    capacityRange: 'El cupo va de 2 a 128 parejas',
    priceRequired: 'Ingresá el precio por pareja',
    pricePositive: 'El precio tiene que ser mayor a 0',
    groupSizeRange: 'Las zonas tienen de 2 a 12 parejas',
    qualifiersRange: 'Tienen que pasar menos parejas que las que tiene la zona, y al menos una',
    gamesRange: 'Los sets van de 4 a 9 games',
    durationRange: 'Cada partido dura entre 30 y 480 minutos',
    courtFeeRange: 'El monto de la cancha va de $0 a $1.000.000',
  },

  // Detail
  detail: {
    categoriesLabel: 'Categorías',
    sections: { pairs: 'Parejas', groups: 'Zonas', bracket: 'Cuadro', matches: 'Partidos' },
    sectionLabel: 'Ver',
    categoryActionsLabel: 'Acciones de la categoría',
    prizesLabel: 'Premios',
    pairsRatio: (taken: number, capacity: number) => `${String(taken)}/${String(capacity)} parejas`,
    pairsRatioShort: (taken: number, capacity: number) => `${String(taken)}/${String(capacity)}`,
    groupsSetup: (size: number, qualifiers: number) => `Zonas de ${String(size)} parejas, pasan ${String(qualifiers)}`,
    matchSetup: (setsToWin: number, games: number, superTiebreak: boolean) =>
      setsToWin === 1
        ? `Un set de ${String(games)} games`
        : `Mejor de tres sets de ${String(games)} games${superTiebreak ? ', súper tie-break como tercero' : ''}`,
    cancelledNote: 'Este torneo está cancelado.',
    categoryCancelledNote: 'Esta categoría está cancelada.',
  },

  // Pairs
  pairs: {
    none: 'Todavía no se anotó ninguna pareja',
    seed: (seed: number) => `Serie ${String(seed)}`,
    group: (name: string) => `Zona ${name}`,
    withdrawn: 'Se bajó',
    payer: 'Pagó la inscripción',
    partner: 'Compañero o compañera',
    call: 'Llamar',
    contact: 'Contactar a la pareja',
    whatsapp: 'WhatsApp',
    whatsappAria: (name: string) => `WhatsApp a ${name}`,
    state: {
      held: 'Pagando',
      confirmed: 'Pagó',
      cancelledUnpaid: 'Sin pago',
      refunded: 'Reembolsado',
      refundPending: 'Reembolso en curso',
      partialRefund: 'Reembolso parcial',
    },
    paid: (amount: string) => `Pagó ${amount}`,
    refundedAmount: (amount: string) => `Devuelto ${amount}`,
    swap: 'Intercambiar',
    swapTitle: 'Intercambiar pareja',
    swapDescription: (name: string) => `Elegí con qué pareja intercambia lugar ${name}.`,
    swapWith: 'Intercambiar con',
    swapPlaceholder: 'Elegí una pareja',
    swapConfirm: 'Intercambiar',
    swapError: 'No pudimos intercambiar las parejas',
    swapGroupHint: 'Tienen que estar en zonas distintas.',
  },

  // Draw
  draw: {
    title: 'Sorteo',
    generate: 'Sortear',
    regenerate: 'Sortear de nuevo',
    generateHint: 'Arma las zonas y el cuadro con las parejas que ya pagaron. Se cierra la inscripción.',
    generateHintElimination: 'Arma el cuadro con las parejas que ya pagaron. Se cierra la inscripción.',
    regenerateHint: 'Se puede repetir mientras no haya resultados cargados.',
    seedOrder: 'Orden de cabezas de serie',
    seedOrderHint: 'Subí o bajá las parejas: la de arriba es la mejor cabeza de serie. Las que no ordenes van al azar.',
    seedOrderOff: 'Dejar al azar',
    seedOrderOn: 'Ordenar cabezas de serie',
    moveUp: (name: string) => `Subir a ${name}`,
    moveDown: (name: string) => `Bajar a ${name}`,
    confirmTitle: 'Sortear la categoría',
    confirmDescription: 'Se cierra la inscripción y se arman los cruces con las parejas que ya pagaron.',
    confirmRegenerate:
      'Se rehace el sorteo con las parejas que ya pagaron. Las que están pagando todavía no entran y se les devuelve el pago.',
    confirmLabel: 'Sortear',
    back: 'Volver',
    success: 'Sorteo hecho',
    errors: {
      locked: 'Ya hay resultados cargados: el sorteo no se puede cambiar.',
      notEnough: 'Hacen falta al menos dos parejas que hayan pagado.',
      settings: 'La configuración de zonas no alcanza para esas parejas. Cambiala o esperá más inscriptas.',
      settingsWhy: (pairs: number, size: number, qualifiers: number) =>
        `Con ${String(pairs)} parejas no se pueden armar zonas de ${String(size)} donde pasen ${String(qualifiers)}. Bajá cuántas pasan o el tamaño de la zona.`,
      unknown: 'No pudimos hacer el sorteo',
    },
  },

  // Groups
  groups: {
    none: 'Todavía no hay sorteo: acá vas a ver las zonas y sus posiciones.',
    zone: (name: string) => `Zona ${name}`,
    table: (name: string) => `Posiciones de la zona ${name}`,
    columns: {
      position: 'Pos.',
      pair: 'Pareja',
      played: 'PJ',
      won: 'PG',
      lost: 'PP',
      sets: 'Sets',
      games: 'Games',
      difference: 'Dif.',
    },
    differenceTitle: 'Diferencia de sets',
    qualifies: 'Clasifica',
  },

  // Bracket
  bracket: {
    none: 'Todavía no hay sorteo: acá vas a ver el cuadro.',
    label: 'Cuadro del torneo',
    round: (round: number) => `Ronda ${String(round)}`,
    final: 'Final',
    semifinal: 'Semifinal',
    semifinals: 'Semifinales',
    quarterfinal: 'Cuartos de final',
    roundOf16: 'Octavos de final',
    toBeDefined: 'A definir',
    bye: 'Libre',
    sourceOf: (rank: number, group: string) => `${String(rank)}º de la zona ${group}`,
    walkover: 'W.O.',
    wonBy: (name: string) => `Ganó ${name}`,
  },

  // Matches
  matches: {
    none: 'Todavía no hay partidos. Se arman con el sorteo.',
    actionsLabel: 'Acciones del partido',
    notReady: 'Esperando a las parejas',
    pending: 'Pendiente',
    completed: 'Jugado',
    unscheduled: 'Sin horario',
    scheduledAt: (court: string, date: string, time: string) => `${court} · ${date} · ${time}`,
    schedule: 'Programar',
    reschedule: 'Cambiar horario',
    unschedule: 'Sacar de la cancha',
    loadResult: 'Cargar resultado',
    correctResult: 'Corregir resultado',
    correctLocked: 'No se puede corregir: el partido siguiente ya tiene resultado.',
    matchday: (round: number) => `Fecha ${String(round)}`,
    versus: 'vs',
    walkoverNote: (name: string) => `W.O.: ${name} no se presentó`,
  },

  courtFee,

  // Schedule
  schedule: {
    title: 'Programar partido',
    court: 'Cancha',
    courtPlaceholder: 'Seleccioná una cancha',
    date: 'Día',
    time: 'Hora',
    duration: 'Duración (minutos)',
    durationHint: (minutes: number) => `Por defecto ${String(minutes)} minutos, según la categoría.`,
    submit: 'Guardar horario',
    success: 'Partido programado',
    removed: 'Partido sacado de la cancha',
    courtRequired: 'Elegí la cancha',
    dateRequired: 'Elegí el día',
    timeRequired: 'Elegí la hora',
    durationRange: 'El partido dura entre 30 y 480 minutos',
    outsideDays: 'Elegí un día dentro de los días del torneo.',
    unscheduleError: 'No pudimos sacar el partido de la cancha',
    conflict: {
      title: 'Ese horario no está libre',
      booking: (court: string) => `Ya hay una reserva en ese horario en ${court}.`,
      blocked: (court: string) => `Ese horario está bloqueado en ${court}.`,
      fixed: (court: string) => `Un turno fijo ocupa ese horario en ${court}.`,
      event: (court: string) => `Ya hay un evento de canchas abiertas en ese horario en ${court}.`,
      match: (court: string) => `Ya hay otro partido de torneo en ese horario en ${court}.`,
      unknown: 'La cancha ya está ocupada en ese horario.',
      hint: 'Cambiá la cancha o el horario.',
    },
    error: 'No pudimos programar el partido',
  },

  // Result
  result: {
    title: 'Resultado del partido',
    modeWinner: 'Quién ganó',
    modeSets: 'Cargar resultado por sets',
    modeWalkover: 'W.O.',
    modeLabel: 'Cómo se definió',
    columnsLabel: 'Parejas del partido',
    set: (n: number) => `Set ${String(n)}`,
    superTiebreakSet: (n: number) => `Set ${String(n)} · súper tie-break`,
    games: (name: string) => `Games de ${name}`,
    points: (name: string) => `Puntos de ${name}`,
    optionalSet: 'Solo si hizo falta',
    winnerQuestion: 'Quién ganó el partido',
    winnerHint: 'No hace falta el marcador: el ganador pasa de ronda igual. Si querés, cargalo por sets.',
    walkoverQuestion: 'Quién no se presentó',
    walkoverHint: 'Cuenta como una victoria por el resultado máximo para la otra pareja.',
    submit: 'Guardar resultado',
    success: 'Resultado guardado',
    errors: {
      pickWinner: 'Elegí quién ganó el partido.',
      pickAbsent: 'Elegí quién no se presentó.',
      missing: (n: number) => `Cargá los games del set ${String(n)}.`,
      gap: (n: number) => `Completá el set ${String(n)} antes del siguiente.`,
      invalidSet: (n: number, a: number, b: number, games: number) =>
        `El set ${String(n)} (${String(a)}-${String(b)}) no es un set terminado de ${String(games)} games.`,
      invalidSuper: 'El súper tie-break se juega a 10 puntos con dos de diferencia.',
      decided: (n: number) => `El partido ya estaba definido: no se juega el set ${String(n)}.`,
      undecided: 'Nadie llegó a los sets necesarios: falta cargar un set.',
      decisiveSuper: 'El tercer set es un súper tie-break.',
      notNumber: 'Cargá solo números enteros.',
    },
    apiErrors: {
      locked: 'No se puede corregir: el partido siguiente ya tiene resultado.',
      invalid: 'El resultado no es válido para el formato de esta categoría.',
      notReady: 'El partido todavía no tiene a las dos parejas.',
      unknown: 'No pudimos guardar el resultado',
    },
  },

  // Cancel
  cancel: {
    action: 'Cancelar torneo',
    confirmTitle: 'Cancelar el torneo',
    confirmDescription:
      'Se cancelan todas las categorías y las canchas de los partidos quedan libres. Ninguna pareja pagó todavía.',
    confirmDescriptionPaid: (paid: number) =>
      `Se cancelan todas las categorías y las canchas de los partidos quedan libres. ${
        paid === 1 ? 'A la pareja que ya pagó' : `A las ${String(paid)} parejas que ya pagaron`
      } les devolvemos todo por Mercado Pago, servicio incluido. Les avisamos por email si dejaron uno.`,
    confirmLabel: 'Sí, cancelar torneo',
    back: 'Volver',
    success: (refunded: number) =>
      refunded === 0
        ? 'Torneo cancelado'
        : `Torneo cancelado · ${String(refunded)} ${refunded === 1 ? 'reembolso en curso' : 'reembolsos en curso'}`,
    error: 'No pudimos cancelar el torneo',
    categoryAction: 'Cancelar categoría',
    categoryTitle: (name: string) => `Cancelar la categoría ${name}`,
    categoryDescription: 'Se cancelan sus partidos y las canchas quedan libres. Ninguna pareja pagó todavía.',
    categoryDescriptionPaid: (paid: number) =>
      `Se cancelan sus partidos y las canchas quedan libres. ${
        paid === 1 ? 'A la pareja que ya pagó' : `A las ${String(paid)} parejas que ya pagaron`
      } les devolvemos todo por Mercado Pago, servicio incluido. Les avisamos por email si dejaron uno.`,
    categoryConfirmLabel: 'Sí, cancelar categoría',
    categorySuccess: (refunded: number) =>
      refunded === 0
        ? 'Categoría cancelada'
        : `Categoría cancelada · ${String(refunded)} ${refunded === 1 ? 'reembolso en curso' : 'reembolsos en curso'}`,
    categoryError: 'No pudimos cancelar la categoría',
  },
} as const;
