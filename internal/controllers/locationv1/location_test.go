package locationv1

import (
	"kids-checkin/internal/client/planningcenter"
	"kids-checkin/internal/db"
	"kids-checkin/internal/repo/location"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFoo(t *testing.T) {
	db1, err := db.InitDB("/Users/stevenoalfonso/Dev/NW-Kids-Checkout/database/kids-checkin.db")
	require.NoError(t, err)

	locRepo := location.NewRepo(db1)

	pcClient := planningcenter.NewClient()
	locations, err := pcClient.GetLocationsForEvent(t.Context(), "152112")
	require.NoError(t, err)

	for _, l := range locations {
		_, err = locRepo.CreateLocation(t.Context(), location.Location{
			ID:                     0,
			PlanningCenterID:       l.ID,
			PlanningCenterParentID: l.ParentID,
			Name:                   l.Name,
		})
	}
}
