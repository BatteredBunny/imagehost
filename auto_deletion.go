package main

import (
	"database/sql"
	"log"

	_ "github.com/lib/pq"

	"github.com/robfig/cron/v3"
)

func auto_deletion(db *sql.DB, config Config, logger *log.Logger) {
	c := cron.New()

	c.AddFunc("@hourly", func() {
		logger.Println("Starting hourly cron job")

		rows, err := db.Query("SELECT file_name FROM public.images WHERE created_date < NOW() - INTERVAL '7 days'")
		if err != nil { // Im guessing this happens when it gets no results
			return
		}

		for rows.Next() {
			var file_name string
			rows.Scan(&file_name)

			delete_file(config, file_name)
		}

		db.Exec("DELETE FROM public.images WHERE created_date < NOW() - INTERVAL '7 days'")
	})

	logger.Println("Starting auto deletion server")
	c.Start()
}
