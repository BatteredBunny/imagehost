package main

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	_ "github.com/lib/pq"

	"github.com/robfig/cron/v3"
)

func auto_deletion(db *sql.DB, config Config, s3client *s3.S3) {
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

			s3client.DeleteObject(&s3.DeleteObjectInput{
				Bucket: aws.String(config.S3.Bucket),
				Key:    aws.String(file_name),
			})
		}

		db.Exec("DELETE FROM public.images WHERE created_date < NOW() - INTERVAL '7 days'")
	})

	fmt.Printf("%s | Starting auto deletion server\n", time.Now().Format(time.RFC3339))
	c.Start()
}
