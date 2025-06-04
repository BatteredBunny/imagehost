package cmd

import (
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/rs/zerolog/log"
)

func (app *Application) StartJobScheudler() (err error) {
	app.cron, err = gocron.NewScheduler()
	if err != nil {
		return
	}

	if _, err = app.cron.NewJob(
		gocron.DurationJob(time.Minute*10),
		gocron.NewTask(app.CleanUpJob),
	); err != nil {
		return
	}

	log.Info().Msg("Successfully setup job scheudler")
	app.cron.Start()

	go app.CleanUpJob()

	return
}

func (app *Application) CleanUpJob() {
	log.Info().Msg("Starting clean up job")

	log.Info().Msg("Starting cleaning up expired tokens")
	if err := app.db.deleteExpiredSessionTokens(); err != nil {
		log.Err(err).Msg("Failed to delete expired session tokens")
	}

	log.Info().Msg("Starting cleaning up invite tokens")
	if err := app.db.deleteExpiredInviteCodes(); err != nil {
		log.Err(err).Msg("Failed to delete expired invite codes")
	}

	images, err := app.db.findExpiredImages()
	if err != nil {
		log.Err(err).Msg("Failed to find expired images")
		return
	}

	if len(images) == 0 {
		return
	}

	log.Info().Msgf("Found %d expired images", len(images))

	for _, image := range images {
		if err = app.deleteFile(image.FileName); err != nil {
			log.Err(err).Msg("Failed to delete image file")
		}
	}

	if err = app.db.deleteExpiredImages(); err != nil {
		log.Err(err).Msg("Failed to delete image entries in database")
		return
	}
}
