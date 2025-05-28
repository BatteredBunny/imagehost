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
		gocron.DurationJob(time.Hour),
		gocron.NewTask(app.ImageCleaner),
	); err != nil {
		return
	}

	log.Info().Msg("Successfully setup job scheudler")
	app.cron.Start()

	return
}

func (app *Application) ImageCleaner() {
	log.Info().Msg("Starting hourly image cleaning job")

	images, err := app.db.findAllExpiredImages()
	if err != nil {
		log.Err(err).Msg("Failed to find expired images")
		return
	}

	for _, image := range images {
		if err = app.deleteFile(image.FileName); err != nil {
			log.Err(err).Msg("Failed to delete image file")
		}
	}

	if err = app.db.deleteAllExpiredImages(); err != nil {
		log.Err(err).Msg("Failed to delete image entries in database")
		return
	}
}
