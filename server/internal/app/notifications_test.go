package app

import "testing"

func TestNotificationTemplateKindRoutesChildApplicationApprovalToLeaveTemplate(t *testing.T) {
	if got := notificationTemplateKind("child_application_approved"); got != "leave" {
		t.Fatalf("child application approval template kind = %q, want leave", got)
	}
}
