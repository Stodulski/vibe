package reporting

import (
	"fmt"
)

// This file holds every user-facing string this package produces: the labels
// printed into the exported spreadsheet, and the messages returned when a
// requested report period is not valid.
//
// They are Spanish because that is the only locale the product currently
// ships, and they cannot be translated to English — an owner downloads this
// file and reads it. Gathering them here means moving them behind the backend
// i18n layer, when it exists, is one edit in one file. Nothing else in this
// package should contain a user-visible string.

var spanishMonths = [12]string{
	"Enero", "Febrero", "Marzo", "Abril", "Mayo", "Junio",
	"Julio", "Agosto", "Septiembre", "Octubre", "Noviembre", "Diciembre",
}

// The report-period rejections are returned to the frontend, so they are error
// codes rather than Spanish text — see internal/httpx/codes.go. They are the
// exception in this file, which otherwise holds copy that goes straight into a
// spreadsheet the owner downloads.

// Column headers for the exported workbook.
var (
	paymentSheetHeaders = []string{
		"Fecha", "Horario", "Cancha", "Cliente", "Teléfono",
		"Monto", "Seña", "Fee servicio", "Reembolso",
		"Método", "Estado pago", "Estado reserva",
	}
	summarySheetHeaders = []string{"Método", "Cantidad", "Total", "Reembolsado", "Neto"}
	// The per-court block reuses the summary's shape with its first column
	// renamed, so a reader who has just read the method table already knows
	// how to read this one.
	courtSummaryHeaders = []string{"Cancha", "Cantidad", "Total", "Reembolsado", "Neto"}
)

// Labels for the blocks the summary sheet carries below the method table.
const (
	courtBlockTitle    = "Por cancha"
	previousBlockTitle = "Mes anterior"
	deletedCourtLabel  = "Cancha eliminada"
)

// Sheet names in the exported workbook.
const (
	paymentSheetName = "Pagos"
	summarySheetName = "Resumen"
)

// reportTitle is the heading printed at the top of the summary sheet.
func reportTitle(complexName string, month, year int) string {
	return fmt.Sprintf("Reporte Mensual - %s - %s %d", complexName, spanishMonths[month-1], year)
}

func centavosToARS(centavos int) string {
	return fmt.Sprintf("$%.2f", float64(centavos)/100)
}

func methodLabel(method string) string {
	switch method {
	case "mercadopago":
		return "MercadoPago"
	case "cash":
		return "Efectivo"
	case "transfer":
		return "Transferencia"
	default:
		return method
	}
}

func paymentStatusLabel(status string) string {
	switch status {
	case "unpaid":
		return "Sin pagar"
	case "deposit_paid":
		return "Seña pagada"
	case "fully_paid":
		return "Pagado"
	case "refund_pending":
		return "Reembolso pendiente"
	case "partial_refund":
		return "Reembolso parcial"
	case "refunded":
		return "Reembolsado"
	default:
		return status
	}
}

func bookingStatusLabel(status string) string {
	switch status {
	case "pending":
		return "Pendiente"
	case "confirmed":
		return "Confirmada"
	case "cancelled":
		return "Cancelada"
	case "completed":
		return "Completada"
	case "no_show":
		return "No se presentó"
	default:
		return status
	}
}
