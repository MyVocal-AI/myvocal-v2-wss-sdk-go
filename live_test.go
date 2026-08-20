package myvocalwss

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

func TestLiveFullFieldSynthesis(t *testing.T) {
	apiKey := os.Getenv("MYVOCAL_TTS_API_KEY")
	voiceID := os.Getenv("MYVOCAL_VOICE_ID")
	if apiKey == "" || voiceID == "" {
		t.Skip("live credentials are not present")
	}
	var countsMu sync.Mutex
	counts := map[string]int{}
	client := NewClient(DefaultEndpoint, apiKey, Handlers{
		OnEvent: func(event Event) {
			countsMu.Lock()
			counts[event.Event]++
			countsMu.Unlock()
		},
	})
	defer client.Close()
	ctx, cancel := context.WithTimeout(
		context.Background(), 90*time.Second,
	)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	sessionID := fmt.Sprintf("sdk-go-%d", time.Now().UnixMilli())
	speed, volume, pitch := 1.0, 1.0, 0
	emotion, normalization, latexRead := "calm", true, false
	modifyValue, effect, subtitle := 0, "robotic", true
	request := StartRequest{
		Voice:      voiceID,
		Language:   "en",
		Format:     FormatMP3,
		SampleRate: 32000,
		Bitrate:    128000,
		Channel:    1,
		VoiceSetting: &VoiceSetting{
			Speed:                &speed,
			Volume:               &volume,
			Pitch:                &pitch,
			Emotion:              &emotion,
			EnglishNormalization: &normalization,
			LatexRead:            &latexRead,
		},
		PronunciationDict: &PronunciationDict{
			Tone: []string{"MyVocal/My Vocal"},
		},
		TimbreWeights: []TimbreWeight{
			{VoiceID: voiceID, Weight: 100},
		},
		VoiceModify: &VoiceModify{
			Pitch:        &modifyValue,
			Intensity:    &modifyValue,
			Timbre:       &modifyValue,
			SoundEffects: &effect,
		},
		SubtitleEnable: &subtitle,
		SubtitleType:   SubtitleWordStreaming,
		SessionID:      sessionID,
	}
	if err := client.Start(ctx, request); err != nil {
		t.Fatal(err)
	}
	if err := client.SendText(
		"MyVocal live Go SDK validation is complete.",
	); err != nil {
		t.Fatal(err)
	}
	if err := client.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	result, err := client.Finish(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Audio) == 0 || result.CloseCode != 1000 {
		t.Fatalf(
			"audio=%d close=%d", len(result.Audio), result.CloseCode,
		)
	}
	if result.SessionID != sessionID {
		t.Fatalf("session ID was not echoed")
	}
	hash := sha256.Sum256(result.Audio)
	countsMu.Lock()
	safe := map[string]interface{}{
		"sdk":         "go",
		"passed":      true,
		"audioBytes":  len(result.Audio),
		"audioSha256": hex.EncodeToString(hash[:]),
		"closeCode":   result.CloseCode,
		"sessionEcho": result.SessionID == sessionID,
		"eventCounts": counts,
	}
	countsMu.Unlock()
	encoded, _ := json.Marshal(safe)
	fmt.Printf("LIVE_RESULT=%s\n", encoded)
}
