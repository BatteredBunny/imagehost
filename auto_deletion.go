package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"

	"github.com/robfig/cron/v3"
)

func auto_deletion(db *sql.DB) {
	c := cron.New()

	c.AddFunc("@hourly", func() {
		fmt.Println("Starting hourly cron job")

		rows, err := db.Query("select file_name from public.images WHERE created_date < NOW() - INTERVAL '7 days'")
		if err != nil { // Im guessing this happens when it gets no results
			return
		}

		for rows.Next() {
			var file_name string
			rows.Scan(&file_name)

			os.Remove("/app/data/" + file_name)
		}

		db.Exec("delete from public.images WHERE created_date < NOW() - INTERVAL '7 days'")
	})

	fmt.Println("Starting auto deletion server")
	c.Start()
}
