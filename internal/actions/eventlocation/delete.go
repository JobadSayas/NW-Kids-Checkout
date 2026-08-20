package eventlocation

import (
	"context"
	"database/sql"

	"kids-checkin/internal/repo"
	"kids-checkin/internal/repo/checkin"
	"kids-checkin/internal/repo/event"
	"kids-checkin/internal/repo/eventcheckwindow"
	"kids-checkin/internal/repo/location"
)

func DeleteEventWithDependents(ctx context.Context, db *sql.DB, eventID int64) error {
	return repo.WithTx(ctx, db, func(tx *sql.Tx) error {
		windowRepo := eventcheckwindow.NewRepo(tx)
		windows, err := windowRepo.GetCheckWindowsForEvent(ctx, eventID)
		if err != nil {
			return err
		}
		for _, w := range windows {
			if err := windowRepo.DeleteCheckWindow(ctx, w.ID); err != nil {
				return err
			}
		}

		locRepo := location.NewRepo(tx)
		locs, err := locRepo.ListLocations(ctx, location.LocationFilter{EventID: eventID})
		if err != nil {
			return err
		}

		checkinRepo := checkin.NewRepo(tx)
		seenCheckinIDs := make(map[int64]bool)

		for _, loc := range locs {
			locCheckins, err := checkinRepo.ListCheckins(ctx, checkin.Filter{LocationID: loc.ID})
			if err != nil {
				return err
			}
			for _, chk := range locCheckins {
				if !seenCheckinIDs[chk.ID] {
					seenCheckinIDs[chk.ID] = true
					if err := checkinRepo.DeleteCheckin(ctx, chk.ID); err != nil {
						return err
					}
				}
			}
		}

		eventCheckins, err := checkinRepo.ListCheckins(ctx, checkin.Filter{EventID: eventID})
		if err != nil {
			return err
		}
		for _, chk := range eventCheckins {
			if !seenCheckinIDs[chk.ID] {
				seenCheckinIDs[chk.ID] = true
				if err := checkinRepo.DeleteCheckin(ctx, chk.ID); err != nil {
					return err
				}
			}
		}

		for _, loc := range locs {
			if err := locRepo.DeleteLocation(ctx, loc.ID); err != nil {
				return err
			}
		}

		eventRepo := event.NewRepo(tx)
		if err := eventRepo.DeleteEvent(ctx, eventID); err != nil {
			return err
		}

		return nil
	})
}
