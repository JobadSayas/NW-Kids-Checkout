package eventlocation

import (
	"context"
	"database/sql"
	"errors"

	"kids-checkin/internal/repo"
	"kids-checkin/internal/repo/event"
	"kids-checkin/internal/repo/location"
)

func CreateEventWithLocations(ctx context.Context, db *sql.DB, newEvent event.Event, locations []location.Location) (event.Event, []location.Location, error) {
	createdLocations := make([]location.Location, 0, len(locations))
	var createdEvent event.Event

	err := repo.WithTx(ctx, db, func(tx *sql.Tx) error {
		var err error
		eventRepo := event.NewRepo(tx)
		_, err = eventRepo.GetEventByPlanningCenterID(ctx, newEvent.PlanningCenterID)
		if err == nil {
			return event.ErrEventExists
		}
		if !errors.Is(err, repo.ErrNotFound) {
			return err
		}

		createdEvent, err = eventRepo.CreateEvent(ctx, newEvent)
		if err != nil {
			return err
		}

		locRepo := location.NewRepo(tx)
		for _, loc := range locations {
			loc.EventID = createdEvent.ID
			created, err := locRepo.CreateLocation(ctx, loc)
			if err != nil {
				return err
			}
			createdLocations = append(createdLocations, created)
		}
		return nil
	})
	if err != nil {
		return event.Event{}, nil, err
	}

	return createdEvent, createdLocations, nil
}
