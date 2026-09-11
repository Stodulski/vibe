package notifications

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/stodulski/vibe-server/internal/data"
)

// This file holds the sentences a client is told about their money and their
// cancellation rights, in one place, in Spanish.
//
// They live here rather than in internal/mailer or internal/whatsapp because
// the same event now goes out on both channels and the two must not be able to
// disagree. They used to: the confirmation email described a cancellation
// window the WhatsApp message never mentioned, and neither said a word about
// what had been paid or what was still owed at the venue. A sentence rendered
// once, at the enqueue site, and carried in the payload is the only shape
// where "the email says the same as the WhatsApp" is true by construction
// rather than by two people editing two templates in step.
//
// Vocabulary is fixed and not negotiable per template: "seña" for the deposit,
// "devolución" for money coming back (never "reembolso"), "complejo" for the
// venue (never "club"), "link" for a URL (never "enlace").

// FormatARS renders centavos as Argentine pesos the way the product writes
// them: $12.345,67 — a dot for thousands, a comma for centavos — except that
// the comma and the centavos are omitted entirely for a whole-peso amount, so
// $5.000,00 is written $5.000.
func FormatARS(centavos int) string {
	sign := ""
	if centavos < 0 {
		sign = "-"
		centavos = -centavos
	}
	whole := strconv.Itoa(centavos / 100)
	var grouped strings.Builder
	for i, digit := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			grouped.WriteByte('.')
		}
		grouped.WriteRune(digit)
	}
	if centavos%100 == 0 {
		return fmt.Sprintf("%s$%s", sign, grouped.String())
	}
	return fmt.Sprintf("%s$%s,%02d", sign, grouped.String(), centavos%100)
}

// PaymentAmounts returns what the client has already paid as a deposit and
// what is still owed at the complex, each as a plain formatted amount rather
// than a sentence.
//
// The WhatsApp template's words around these two values are fixed ("Seña
// abonada: {{5}}. Resta pagar en el complejo: {{6}}."), because Meta rejects a
// template where two variables sit next to each other with nothing fixed
// between them — so "nothing owed" and "nothing collected" now have to be
// $0, not a sentence saying so. A booking a staff member entered with nothing
// collected still reports a deposit of $0 rather than dropping the fact:
// telling nobody anything is what the confirmation used to do.
func PaymentAmounts(priceCentavos, depositCentavos int, collectionStatus string) (depositAmount, balanceAmount string) {
	switch collectionStatus {
	case data.CollectionStatusFullyPaid:
		// The whole price was collected. depositCentavos may still hold
		// whatever the complex's deposit percentage would have produced for a
		// booking paid in one go — ConfirmPayment never rewrites it once the
		// booking reads as fully paid — so the price itself, not that stale
		// field, is what the client actually paid.
		return FormatARS(priceCentavos), FormatARS(0)
	case data.CollectionStatusDepositPaid:
		balance := priceCentavos - depositCentavos
		if balance <= 0 {
			return FormatARS(depositCentavos), FormatARS(0)
		}
		return FormatARS(depositCentavos), FormatARS(balance)
	default:
		// Unpaid, and any status this file does not know: the safe amount is
		// the one that overstates what is owed rather than understating it.
		if priceCentavos <= 0 {
			return FormatARS(0), FormatARS(0)
		}
		return FormatARS(0), FormatARS(priceCentavos)
	}
}

// BalanceAmount is the half of PaymentAmounts that matters for a message sent
// after the booking was made — the two-hour reminder, where what was paid
// three days ago matters less than what is left to bring.
func BalanceAmount(priceCentavos, depositCentavos int, collectionStatus string) string {
	_, balance := PaymentAmounts(priceCentavos, depositCentavos, collectionStatus)
	return balance
}

// CancellationLine states the rule under which this booking can still be
// cancelled with the deposit returned.
//
// It returns the rule alone, lowercase, because the template's fixed word
// sits in front of it ("Cancelación {{7}}"): "Cancelación con devolución
// hasta..." reads as one sentence, and a capital letter after "Cancelación "
// would not.
//
// inStandardWindow is the caller's answer (pricing.WithinStandardWindow):
// inside the complex's own window the rule is that window, and outside it the
// only rule left is the grace period that runs from the moment the booking was
// made. The grace period is operator-configured, which is the defect this
// sentence was extracted over — the email quoted a hardcoded fifteen minutes
// at every venue, whatever they had set.
func CancellationLine(cancellationHours int, gracePeriod time.Duration, inStandardWindow bool) string {
	if inStandardWindow {
		if cancellationHours <= 0 {
			return "con devolución hasta el inicio del turno."
		}
		return fmt.Sprintf("con devolución hasta %s antes del turno.",
			spanishDuration(time.Duration(cancellationHours)*time.Hour))
	}
	if gracePeriod <= 0 {
		return "sin devolución."
	}
	return fmt.Sprintf("con devolución hasta %s después de la reserva.",
		spanishDuration(gracePeriod))
}

// ExpiredUnpaidRefundLine is what a booking cancelled by the payment-expiry
// sweep tells its client.
//
// It is a constant rather than an outcome rendering because there is no
// outcome: nothing was ever collected, so no refund path ran and none of the
// six results describes what happened. The client's question is "why", and the
// generic cancellation copy never answered it.
//
// Lowercase, like every other refundMessage variant in internal/bookings: it
// is a value substituted after the template's fixed "Devolución: ".
const ExpiredUnpaidRefundLine = "reserva vencida por falta de pago, no hay pagos a devolver."

// spanishDuration renders a duration the way a client reads it.
//
// The zero value is the caller's problem, not this function's: a complex
// configured with no grace period should not be told it has "0 minutos" to
// cancel, and CancellationLine above is what decides that sentence is never
// reached.
func spanishDuration(d time.Duration) string {
	minutes := int(d.Round(time.Minute).Minutes())
	switch {
	case minutes >= 120 && minutes%60 == 0:
		return fmt.Sprintf("%d horas", minutes/60)
	case minutes == 60:
		return "1 hora"
	case minutes == 1:
		return "1 minuto"
	default:
		return fmt.Sprintf("%d minutos", minutes)
	}
}
