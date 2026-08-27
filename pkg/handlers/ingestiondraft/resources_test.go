package ingestiondraft

import (
	"testing"
	"time"

	"github.com/tyemirov/RSVP/models"
)

func TestDraftRequestInterpretsLocalTimesInProposedTimezone(testingContext *testing.T) {
	localStart := "2030-02-04T10:30"
	localEnd := "2030-02-04T12:00"
	request := draftRequest{
		Mode: models.IngestionModeDatedEvent, CalendarID: "CAL00001", Title: "Timezone correction",
		StartsAt: &localStart, EndsAt: &localEnd, ReferenceTime: "2030-01-02T15:00:00Z", Timezone: "America/New_York",
	}
	proposal, proposalError := request.proposal()
	if proposalError != nil {
		testingContext.Fatalf("construct proposal: %v", proposalError)
	}
	wantStart := time.Date(2030, time.February, 4, 15, 30, 0, 0, time.UTC)
	wantEnd := time.Date(2030, time.February, 4, 17, 0, 0, 0, time.UTC)
	if proposal.StartsAt == nil || !proposal.StartsAt.Equal(wantStart) || proposal.EndsAt == nil || !proposal.EndsAt.Equal(wantEnd) {
		testingContext.Fatalf("proposal times = %v to %v, want %v to %v", proposal.StartsAt, proposal.EndsAt, wantStart, wantEnd)
	}
}
