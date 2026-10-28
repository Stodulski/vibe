package notifications

import "testing"

// Every notification payload carries a recipient and, for most, a single-use
// link (a cancel, reset or verification URL). Nothing reads it once the job has
// finished, so each task type must be scrubbed. The list is every Task constant
// in this package: a new task that is missing here is the signal to check it.
func TestEveryNotificationTaskHasItsPayloadScrubbed(t *testing.T) {
	tasks := []string{
		TaskEmailBookingConfirmation,
		TaskEmailReminder2h,
		TaskEmailBookingCancelled,
		TaskEmailDepositRefunded,
		TaskEmailOwnerNewBooking,
		TaskEmailOwnerBookingCancelled,
		TaskEmailOwnerBookingRefunded,
		TaskEmailVerification,
		TaskEmailPasswordReset,
		TaskEmailDuplicateRegistration,
		TaskEmailChangeRequested,
		TaskWABookingConfirmation,
		TaskWAReminder2h,
		TaskWABookingCancelled,
		TaskWADepositRefunded,
		TaskEmailBenefitsCampaign,
		TaskEmailLifecycleNudge,
		TaskEmailWaitlistSlotFreed,
		TaskEmailWelcome,
	}
	for _, task := range tasks {
		if !ScrubsPayload(task) {
			t.Errorf("ScrubsPayload(%q) = false; its payload carries recipient data and links", task)
		}
	}
}

// Another queue's payload is read back after the job is done (the payments
// export builds its download key from it), so the policy must not reach it.
func TestOtherQueuesKeepTheirPayloads(t *testing.T) {
	for _, task := range []string{"export:payments", "indexnow.submit"} {
		if ScrubsPayload(task) {
			t.Errorf("ScrubsPayload(%q) = true; that payload is read after the job finishes", task)
		}
	}
}
