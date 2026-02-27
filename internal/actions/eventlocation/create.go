package eventlocation

import (
	"context"
	"database/sql"
	"errors"

	"kids-checkin/internal/repo"
	"kids-checkin/internal/repo/event"
	"kids-checkin/internal/repo/location"
)

func CreateEventWithLocations(ctx context.Context, db *sql.DB, newEvent event.Event, locations []location.Location) (event.Event, []location.Location, bool, error) {
	syncedLocations := make([]location.Location, 0, len(locations))
	var createdEvent event.Event
	var eventCreated bool

	err := repo.WithTx(ctx, db, func(tx *sql.Tx) error {
		var err error
		eventRepo := event.NewRepo(tx)
		existingEvent, err := eventRepo.GetEventByPlanningCenterID(ctx, newEvent.PlanningCenterID)
		if err != nil {
			if !errors.Is(err, repo.ErrNotFound) {
				return err
			}

			createdEvent, err = eventRepo.CreateEvent(ctx, newEvent)
			if err != nil {
				return err
			}
			eventCreated = true
		} else {
			createdEvent = existingEvent
			if newEvent.Name != "" && newEvent.Name != existingEvent.Name {
				if err := eventRepo.UpdateEventName(ctx, existingEvent.ID, newEvent.Name); err != nil {
					return err
				}
				createdEvent.Name = newEvent.Name
			}
		}

		locRepo := location.NewRepo(tx)
		existingLocations, err := locRepo.ListLocations(ctx, location.LocationFilter{EventID: createdEvent.ID})
		if err != nil {
			return err
		}

		existingByPlanningCenterID := make(map[string]location.Location, len(existingLocations))
		for _, existing := range existingLocations {
			existingByPlanningCenterID[existing.PlanningCenterID] = existing
		}

		seenPlanningCenterIDs := make(map[string]bool, len(locations))
		for _, loc := range locations {
			loc.EventID = createdEvent.ID
			seenPlanningCenterIDs[loc.PlanningCenterID] = true

			if existing, ok := existingByPlanningCenterID[loc.PlanningCenterID]; ok {
				existing.Name = loc.Name
				existing.PlanningCenterParentID = loc.PlanningCenterParentID
				existing.EventID = createdEvent.ID
				if err := locRepo.UpdateLocation(ctx, existing); err != nil {
					return err
				}
				syncedLocations = append(syncedLocations, existing)
				continue
			}

			created, err := locRepo.CreateLocation(ctx, loc)
			if err != nil {
				return err
			}
			syncedLocations = append(syncedLocations, created)
		}

		for _, existing := range existingLocations {
			if !seenPlanningCenterIDs[existing.PlanningCenterID] {
				if err := locRepo.DeleteLocation(ctx, existing.ID); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return event.Event{}, nil, false, err
	}

	return createdEvent, syncedLocations, eventCreated, nil
}
