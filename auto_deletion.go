package main

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"

	"github.com/robfig/cron/v3"
)

func auto_deletion() {
	c := cron.New()

	c.AddFunc("@hourly", func() {
		fmt.Println("Starting hourly cron job")

		db, err := sql.Open("postgres", CONNECTION_STRING)
		if err != nil { // This error occurs when it can't connect to database
			return
		}

		db.Query("delete from public.images WHERE created_date < NOW() - INTERVAL '7 days'")

		db.Close()
	})

	fmt.Println("Starting auto deletion server")
	c.Start()
}
