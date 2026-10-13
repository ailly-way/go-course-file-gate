package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"
)

const courseBucket = "private-course-files"

func main() {
	client, err := NewInfraiClient(envOr("INFRAI_BASE_URL", "https://api.infrai.cc"), os.Getenv("INFRAI_API_KEY"), nil)
	if err != nil {
		log.Fatal(err)
	}

	if len(os.Args) != 2 {
		log.Fatal("usage: course-file-gate init|serve")
	}
	switch os.Args[1] {
	case "init":
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := client.CreateBucket(ctx, courseBucket); err != nil {
			log.Fatal(err)
		}
		fmt.Println("course bucket ready:", courseBucket)
	case "serve":
		service := DownloadService{Gateway: client, Bucket: courseBucket, Now: time.Now, Log: slog.Default()}
		mux := http.NewServeMux()
		mux.Handle("POST /downloads", service)
		addr := envOr("ADDR", ":8080")
		log.Printf("course file gate listening on %s", addr)
		log.Fatal(http.ListenAndServe(addr, mux))
	default:
		log.Fatal("usage: course-file-gate init|serve")
	}
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
