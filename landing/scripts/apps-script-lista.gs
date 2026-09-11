/**
 * Lista de espera de Vibe: guarda cada alta en la planilla.
 *
 * Este archivo NO se despliega con la landing. Se pega en el editor de Apps
 * Script de la planilla y se publica como aplicacion web. Vive en el repo para
 * que el codigo que corre en Google no quede solo dentro de Google.
 *
 * COMO INSTALARLO
 *  1. Crear una planilla en Google Sheets. La primera hoja se llama "Lista".
 *  2. Extensiones -> Apps Script. Borrar lo que haya y pegar este archivo.
 *  3. Cambiar TOKEN por un secreto largo e inventado (el mismo que va despues
 *     en la variable SHEET_TOKEN de Vercel).
 *  4. Poner AVISAR_A con tu mail si querés el aviso, o dejarlo en '' si no.
 *  5. Implementar -> Nueva implementación -> Aplicación web.
 *       Ejecutar como: Yo
 *       Quién tiene acceso: Cualquier usuario
 *     El segundo es obligatorio: Vercel llama sin sesión de Google.
 *  6. Copiar la URL que termina en /exec y ponerla en SHEET_WEBHOOK_URL.
 *
 * Cada vez que se cambia este código hay que volver a implementar, eligiendo
 * "Nueva versión". Editar y guardar no actualiza la aplicación publicada.
 * El cambio del control de repetidos (por email y origen, no solo por email)
 * necesita esa nueva implementación para estar activo. Lo mismo la versión
 * que agrega las columnas Nombre y Teléfono: hasta que no se implemente de
 * nuevo, esos datos llegan y se descartan.
 */

var TOKEN = 'CAMBIAR-POR-UN-SECRETO-LARGO';
var HOJA = 'Lista';
/* Vacío desactiva el aviso. MailApp usa tu propia cuenta de Google, así que no
   hace falta SMTP ni tocar el SPF del dominio. */
var AVISAR_A = '';

function doPost(e) {
  try {
    var cuerpo = JSON.parse((e && e.postData && e.postData.contents) || '{}');

    /* La URL de una aplicación web es pública: sin token, cualquiera que la
       descubra puede escribir filas. */
    if (cuerpo.token !== TOKEN) return responder({ ok: false, error: 'token invalido' });

    var email = String(cuerpo.email || '').trim().toLowerCase();
    if (!email || email.indexOf('@') < 1) return responder({ ok: false, error: 'mail invalido' });

    var hoja = obtenerHoja();

    var origen = String(cuerpo.origen || '').trim();
    /* Lo que la persona había cargado antes de irse, hasta donde llegó. Pueden
       venir vacíos; el teléfono llega tal cual se tipeó, sin validar. */
    var nombre = String(cuerpo.nombre || '').trim();
    var telefono = String(cuerpo.telefono || '').trim();

    /* Sin esto, alguien que hace doble click queda dos veces en la lista. El
       repetido se mide por email Y origen: la misma dirección puede anotarse en
       la lista de espera desde la landing y, meses después, abandonar el
       registro en la app. Son dos filas distintas y las dos importan.

       Si la fila ya existe, no se descarta el pedido: se completan las celdas
       de Nombre y Teléfono que estén vacías con lo que llegó ahora. Alguien
       que abandonó dos veces suele haber llegado más lejos la segunda. Una
       celda con dato nunca se pisa. */
    var fila = filaExistente(hoja, email, origen);
    if (fila > 0) {
      var completado = completarFila(hoja, fila, nombre, telefono);
      return responder({ ok: true, repetido: true, completado: completado });
    }

    hoja.appendRow([
      cuerpo.fecha || new Date().toISOString(),
      email,
      origen,
      nombre,
      telefono,
    ]);

    if (AVISAR_A) {
      var lineas = [email, cuerpo.fecha || '', origen];
      if (nombre) lineas.push('Nombre: ' + nombre);
      if (telefono) lineas.push('Teléfono: ' + telefono);
      MailApp.sendEmail({
        to: AVISAR_A,
        subject: 'Nuevo anotado en la lista de Vibe: ' + email,
        body: lineas.join('\n'),
      });
    }

    return responder({ ok: true });
  } catch (err) {
    return responder({ ok: false, error: String(err) });
  }
}

/**
 * Comprobación antes de publicar. Se elige "probar" en el desplegable de
 * funciones y se aprieta Ejecutar.
 *
 * Sirve para dos cosas: dispara el pedido de permisos la primera vez, y avisa
 * si el script NO quedó atado a una planilla. Eso pasa cuando se abre el editor
 * desde script.google.com en vez de Extensiones -> Apps Script, y desde el
 * editor las dos situaciones se ven exactamente igual.
 */
function probar() {
  var libro = SpreadsheetApp.getActiveSpreadsheet();
  if (!libro) {
    throw new Error(
      'Este script NO está atado a ninguna planilla. Cerrá esta pestaña, abrí ' +
      'tu Google Sheet, y entrá por Extensiones > Apps Script.');
  }
  var hoja = obtenerHoja();
  Logger.log('Planilla: ' + libro.getName());
  Logger.log('Hoja: ' + hoja.getName() + ' con ' + hoja.getLastRow() + ' fila(s)');
  if (TOKEN === 'CAMBIAR-POR-UN-SECRETO-LARGO') {
    Logger.log('FALTA: todavía no cambiaste el TOKEN.');
  } else {
    Logger.log('Token configurado. Copialo igual en SHEET_TOKEN de Vercel.');
  }
  Logger.log(AVISAR_A ? 'Aviso por mail a: ' + AVISAR_A : 'Aviso por mail desactivado.');
  return 'ok';
}

/* Un GET al /exec sirve para comprobar de un vistazo que la publicación quedó viva. */
function doGet() {
  return responder({ ok: true, servicio: 'lista de espera de Vibe' });
}

var ENCABEZADO = ['Fecha', 'Email', 'Origen', 'Nombre', 'Teléfono'];

function obtenerHoja() {
  var libro = SpreadsheetApp.getActiveSpreadsheet();
  let hoja = libro.getSheetByName(HOJA);
  if (!hoja) {
    hoja = libro.insertSheet(HOJA);
    hoja.appendRow(ENCABEZADO);
  }
  if (hoja.getLastRow() === 0) hoja.appendRow(ENCABEZADO);
  /* Migración única: una planilla creada por la versión anterior tiene solo
     tres columnas. Se le agregan los dos títulos que faltan; las filas viejas
     quedan con esas celdas vacías. */
  if (hoja.getRange(1, 4).getValue() === '') {
    hoja.getRange(1, 4, 1, 2).setValues([[ENCABEZADO[3], ENCABEZADO[4]]]);
  }
  return hoja;
}

/* Número de la fila que ya tiene ese email Y ese origen, o -1 si no hay. Un
   origen vacío en el pedido coincide con una celda vacía. */
function filaExistente(hoja, email, origen) {
  var filas = hoja.getLastRow();
  if (filas < 2) return -1;
  var columnas = hoja.getRange(2, 2, filas - 1, 2).getValues();
  for (let i = 0; i < columnas.length; i++) {
    var mismoEmail = String(columnas[i][0]).trim().toLowerCase() === email;
    var mismoOrigen = String(columnas[i][1] || '').trim() === origen;
    if (mismoEmail && mismoOrigen) return i + 2;
  }
  return -1;
}

/* Rellena Nombre (D) y Teléfono (E) de una fila existente solo donde la celda
   está vacía y el pedido trae algo. Devuelve true si escribió alguna. */
function completarFila(hoja, fila, nombre, telefono) {
  var escribio = false;
  var celdas = hoja.getRange(fila, 4, 1, 2).getValues()[0];
  if (nombre && String(celdas[0] || '').trim() === '') {
    hoja.getRange(fila, 4).setValue(nombre);
    escribio = true;
  }
  if (telefono && String(celdas[1] || '').trim() === '') {
    hoja.getRange(fila, 5).setValue(telefono);
    escribio = true;
  }
  return escribio;
}

function responder(obj) {
  return ContentService
    .createTextOutput(JSON.stringify(obj))
    .setMimeType(ContentService.MimeType.JSON);
}
