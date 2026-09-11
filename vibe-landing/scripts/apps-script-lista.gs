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

    /* Sin esto, alguien que hace doble click queda dos veces en la lista. */
    if (yaEsta(hoja, email)) return responder({ ok: true, repetido: true });

    hoja.appendRow([
      cuerpo.fecha || new Date().toISOString(),
      email,
      cuerpo.origen || '',
    ]);

    if (AVISAR_A) {
      MailApp.sendEmail({
        to: AVISAR_A,
        subject: 'Nuevo anotado en la lista de Vibe: ' + email,
        body: email + '\n' + (cuerpo.fecha || '') + '\n' + (cuerpo.origen || ''),
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

function obtenerHoja() {
  var libro = SpreadsheetApp.getActiveSpreadsheet();
  let hoja = libro.getSheetByName(HOJA);
  if (!hoja) {
    hoja = libro.insertSheet(HOJA);
    hoja.appendRow(['Fecha', 'Email', 'Origen']);
  }
  if (hoja.getLastRow() === 0) hoja.appendRow(['Fecha', 'Email', 'Origen']);
  return hoja;
}

function yaEsta(hoja, email) {
  var filas = hoja.getLastRow();
  if (filas < 2) return false;
  var columna = hoja.getRange(2, 2, filas - 1, 1).getValues();
  for (let i = 0; i < columna.length; i++) {
    if (String(columna[i][0]).trim().toLowerCase() === email) return true;
  }
  return false;
}

function responder(obj) {
  return ContentService
    .createTextOutput(JSON.stringify(obj))
    .setMimeType(ContentService.MimeType.JSON);
}
