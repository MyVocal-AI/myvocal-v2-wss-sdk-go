package main

import (
	"context"
	"log"
	"os"
	"time"

	myvocalwss "github.com/MyVocal-AI/myvocal-v2-wss-sdk-go"
)

func main() {
	ctx, cancel := context.WithTimeout(
		context.Background(), 90*time.Second,
	)
	defer cancel()

	client := myvocalwss.NewClient(
		myvocalwss.DefaultEndpoint,
		os.Getenv("MYVOCAL_TTS_API_KEY"),
		myvocalwss.Handlers{},
	)
	defer client.Close()

	if err := client.Connect(ctx); err != nil {
		log.Fatal(err)
	}
	if err := client.Start(ctx, myvocalwss.StartRequest{
		Voice:        os.Getenv("MYVOCAL_VOICE_ID"),
		Language:     "en",
		Format:       myvocalwss.FormatMP3,
		SubtitleType: myvocalwss.SubtitleWordStreaming,
		SessionID:    "go-example-001",
	}); err != nil {
		log.Fatal(err)
	}
	if err := client.SendText("Hello from MyVocal."); err != nil {
		log.Fatal(err)
	}
	if err := client.Flush(ctx); err != nil {
		log.Fatal(err)
	}
	result, err := client.Finish(ctx)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf(
		"audio_bytes=%d close_code=%d",
		len(result.Audio), result.CloseCode,
	)
}
