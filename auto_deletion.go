package main

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq"

	"github.com/robfig/cron/v3"
)

func auto_deletion(db *sql.DB, config Config) {
	c := cron.New()

	c.AddFunc("@hourly", func() {
		fmt.Printf("%s | Starting hourly cron job\n", time.Now().Format(time.RFC3339))

		rows, err := db.Query("SELECT file_name FROM public.images WHERE created_date < NOW() - INTERVAL '7 days'")
		if err != nil { // Im guessing this happens when it gets no results
			return
		}

		for rows.Next() {
			var file_name string
			rows.Scan(&file_name)

			os.Remove(config.Data_folder + file_name)
		}

		db.Exec("DELET FROM public.images WHERE created_date < NOW() - INTERVAL '7 days'")
	})

	fmt.Printf("%s | Starting auto deletion server\n", time.Now().Format(time.RFC3339))
	c.Start()
}
