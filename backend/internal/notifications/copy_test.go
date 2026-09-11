package notifications

import (
	"strings"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/data"
)

func TestFormatARS(t *testing.T) {
	for _, c := range []struct {
		centavos int
		want     string
	}{
		{0, "$0"},
		{50, "$0,50"},
		{300050, "$3.000,50"},
		{1234567890, "$12.345.678,90"},
		{-500000, "-$5.000"},
		{500000, "$5.000"},
	} {
		if got := FormatARS(c.centavos); got != c.want {
			t.Errorf("FormatARS(%d) = %q; want %q", c.centavos, got, c.want)
		}
	}
}

// A booking a staff member entered with nothing collected has to say so. The
// confirmation used to say nothing about money at all, on either channel, so a
// client had no idea whether they owed the venue the whole price or nothing.
func TestPaymentAmounts(t *testing.T) {
	for _, c := range []struct {
		name                     string
		price, deposit           int
		collectionStatus         string
		wantDeposit, wantBalance string
	}{
		{
			name: "deposit paid online", price: 2000000, deposit: 500000,
			collectionStatus: data.CollectionStatusDepositPaid,
			wantDeposit:      "$5.000", wantBalance: "$15.000",
		},
		{
			name: "nothing collected", price: 2000000, deposit: 500000,
			collectionStatus: data.CollectionStatusUnpaid,
			wantDeposit:      "$0", wantBalance: "$20.000",
		},
		{
			name: "paid in full", price: 2000000, deposit: 2000000,
			collectionStatus: data.CollectionStatusFullyPaid,
			wantDeposit:      "$20.000", wantBalance: "$0",
		},
		{
			name: "a deposit covering the whole price leaves nothing owed", price: 2000000, deposit: 2000000,
			collectionStatus: data.CollectionStatusDepositPaid,
			wantDeposit:      "$20.000", wantBalance: "$0",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			gotDeposit, gotBalance := PaymentAmounts(c.price, c.deposit, c.collectionStatus)
			if gotDeposit != c.wantDeposit || gotBalance != c.wantBalance {
				t.Errorf("PaymentAmounts = (%q, %q); want (%q, %q)", gotDeposit, gotBalance, c.wantDeposit, c.wantBalance)
			}
		})
	}
}

func TestBalanceAmount(t *testing.T) {
	for _, c := range []struct {
		name             string
		price, deposit   int
		collectionStatus string
		want             string
	}{
		{"deposit paid", 2000000, 500000, data.CollectionStatusDepositPaid, "$15.000"},
		{"nothing collected", 2000000, 0, data.CollectionStatusUnpaid, "$20.000"},
		{"paid in full", 2000000, 2000000, data.CollectionStatusFullyPaid, "$0"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := BalanceAmount(c.price, c.deposit, c.collectionStatus); got != c.want {
				t.Errorf("BalanceAmount = %q; want %q", got, c.want)
			}
		})
	}
}

// The grace period is operator-configured. The confirmation quoted a hardcoded
// fifteen minutes at every venue, so anybody running a different number had
// their confirmation contradict the rule the cancel path enforces — and the
// client found out at the moment they were refused.
func TestCancellationLineQuotesTheConfiguredValues(t *testing.T) {
	for _, c := range []struct {
		name             string
		hours            int
		grace            time.Duration
		inStandardWindow bool
		want             string
	}{
		{
			name: "inside the window", hours: 24, inStandardWindow: true,
			want: "con devolución hasta 24 horas antes del turno.",
		},
		{
			name: "a one-hour window reads as one hour", hours: 1, inStandardWindow: true,
			want: "con devolución hasta 1 hora antes del turno.",
		},
		{
			name: "a complex with no window at all", hours: 0, inStandardWindow: true,
			want: "con devolución hasta el inicio del turno.",
		},
		{
			name: "outside the window, fifteen-minute grace", hours: 24, grace: 15 * time.Minute,
			want: "con devolución hasta 15 minutos después de la reserva.",
		},
		{
			name: "outside the window, two-hour grace", hours: 24, grace: 2 * time.Hour,
			want: "con devolución hasta 2 horas después de la reserva.",
		},
		{
			name: "outside the window, no grace configured", hours: 24, grace: 0,
			want: "sin devolución.",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := CancellationLine(c.hours, c.grace, c.inStandardWindow)
			if got != c.want {
				t.Errorf("CancellationLine = %q; want %q", got, c.want)
			}
			if c.grace != 15*time.Minute && strings.Contains(got, "15 minutos") {
				t.Errorf("the sentence quoted a hardcoded 15 minutos: %q", got)
			}
		})
	}
}

// Everything this package renders is read by a client, so it obeys the
// product's vocabulary: "devolución" not "reembolso", "complejo" not "club",
// "link" not "enlace".
func TestCopyObeysTheProductVocabulary(t *testing.T) {
	depositA, balanceA := PaymentAmounts(2000000, 500000, data.CollectionStatusDepositPaid)
	depositB, balanceB := PaymentAmounts(2000000, 0, data.CollectionStatusUnpaid)
	depositC, balanceC := PaymentAmounts(2000000, 2000000, data.CollectionStatusFullyPaid)
	lines := []string{
		depositA, balanceA,
		depositB, balanceB,
		depositC, balanceC,
		BalanceAmount(2000000, 500000, data.CollectionStatusDepositPaid),
		BalanceAmount(2000000, 0, data.CollectionStatusUnpaid),
		BalanceAmount(2000000, 2000000, data.CollectionStatusFullyPaid),
		CancellationLine(24, 15*time.Minute, true),
		CancellationLine(0, 15*time.Minute, true),
		CancellationLine(24, 15*time.Minute, false),
		CancellationLine(24, 0, false),
		ExpiredUnpaidRefundLine,
	}
	for _, line := range lines {
		for _, forbidden := range []string{"reembolso", "enlace", "club"} {
			if strings.Contains(strings.ToLower(line), forbidden) {
				t.Errorf("%q uses the forbidden word %q", line, forbidden)
			}
		}
	}
}
