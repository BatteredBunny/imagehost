package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"

	"github.com/robfig/cron/v3"
)

func auto_deletion() {
	c := cron.New()

	c.AddFunc("@hourly", func() {
		fmt.Println("Starting hourly cron job")

		db, err := sql.Open("postgres", os.Getenv("POSTGRES_CONN"))
		if err != nil { // This error occurs when it can't connect to database
			return
		}

		rows, err := db.Query("select file_name from public.images WHERE created_date < NOW() - INTERVAL '7 days'")
		if err != nil { // Im guessing this happens when it gets no results
			return
		}

		for rows.Next() {
			var file_name string
			rows.Scan(&file_name)

			os.Remove("/app/data/" + file_name)
		}

		db.Query("delete from public.images WHERE created_date < NOW() - INTERVAL '7 days'")

		db.Close()
	})

	fmt.Println("Starting auto deletion server")
	c.Start()
}
