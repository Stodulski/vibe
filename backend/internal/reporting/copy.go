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
	// cashboxSalesHeaders and cashboxCategoryHeaders are the "Caja" section's
	// two sub-tables. No Reembolsado/Neto column: a voided sale is excluded
	// outright (there is no refund concept for a counter sale), and a voided
	// cash movement is excluded the same way — see CashMovementsByCategory.
	cashboxSalesHeaders    = []string{"Método", "Cantidad", "Total"}
	cashboxCategoryHeaders = []string{"Categoría", "Cantidad", "Total"}
)

// Labels for the blocks the summary sheet carries below the method table.
const (
	courtBlockTitle    = "Por cancha"
	previousBlockTitle = "Mes anterior"
	deletedCourtLabel  = "Cancha eliminada"
	// cashboxBlockTitle, cashboxSalesTotalLabel and cashboxNetLabel are the
	// "Caja" section's own labels — see writeCashboxBlock.
	cashboxBlockTitle      = "Caja"
	cashboxSalesTotalLabel = "Total ventas POS"
	cashboxNetLabel        = "Neto caja"
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
	case "debit_card":
		return "Débito"
	case "credit_card":
		return "Crédito"
	case "qr_wallet":
		return "QR / billetera"
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

// cashCategoryLabels names every cash_movements category for the "Caja"
// section — the same 21 values
// db/migrations/008_cash_movement_categories.sql's
// cash_movements_category_check accepts: 'sale' plus the 7 income categories
// (income), and 'restock' plus the 12 expense categories (expense). The map
// here is the one place these labels live; the frontend has its own copy for
// the cashbox screens (src/shared/i18n/es_AR/cash.ts) because this package
// cannot be imported from TypeScript.
var cashCategoryLabels = map[string]string{
	"sale":              "Ventas",
	"other_income":      "Otros ingresos",
	"classes":           "Clases",
	"tournaments":       "Torneos",
	"events":            "Eventos",
	"memberships":       "Cuotas y abonos",
	"sponsorship":       "Publicidad y sponsors",
	"cash_contribution": "Aporte / cambio",
	"supplies":          "Insumos",
	"salaries":          "Sueldos",
	"services":          "Servicios",
	"maintenance":       "Mantenimiento",
	"cleaning":          "Limpieza",
	"withdrawal":        "Retiro",
	"other_expense":     "Otros egresos",
	"restock":           "Reposición",
	"rent":              "Alquiler del local",
	"taxes":             "Impuestos",
	"professional_fees": "Honorarios",
	"marketing":         "Marketing",
	"bank_fees":         "Comisiones bancarias",
}

func cashCategoryLabel(category string) string {
	if label, ok := cashCategoryLabels[category]; ok {
		return label
	}
	return category
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
