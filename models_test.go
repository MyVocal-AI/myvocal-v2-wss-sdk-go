package myvocalwss

import (
	"encoding/json"
	"testing"
)

func TestStartRequestSerializesEverySupportedField(t *testing.T) {
	req := StartRequest{
		Voice:      "voice-a",
		Language:   "en",
		Format:     FormatMP3,
		SampleRate: 32000,
		Bitrate:    128000,
		Channel:    1,
		VoiceSetting: &VoiceSetting{
			Speed:                ptr(1.1),
			Volume:               ptr(1.2),
			Pitch:                ptr(1),
			Emotion:              ptr("calm"),
			EnglishNormalization: ptr(true),
			LatexRead:            ptr(false),
		},
		PronunciationDict: &PronunciationDict{
			Tone: []string{"MyVocal/My Vocal"},
		},
		TimbreWeights: []TimbreWeight{
			{VoiceID: "voice-a", Weight: 100},
		},
		VoiceModify: &VoiceModify{
			Pitch:        ptr(10),
			Intensity:    ptr(10),
			Timbre:       ptr(10),
			SoundEffects: ptr("robotic"),
		},
		SubtitleEnable: ptr(true),
		SubtitleType:   SubtitleWordStreaming,
		SessionID:      "session-001",
	}

	if err := req.Validate(); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]interface{}
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}

	if wire["event"] != "task_start" {
		t.Fatalf("event = %v", wire["event"])
	}
	if wire["model"] != "Skyblight-Turbo" {
		t.Fatalf("model = %v", wire["model"])
	}
	if wire["priority"] != "dedicated_concurrency" {
		t.Fatalf("priority = %v", wire["priority"])
	}
	if _, ok := wire["continuous_sound"]; ok {
		t.Fatal("continuous_sound must not be exposed")
	}
}

func TestStartRequestRejectsUnsupportedValues(t *testing.T) {
	tests := []StartRequest{
		{Voice: "voice-a", Format: "opus"},
		{Voice: "voice-a", SampleRate: 12345},
		{Voice: "voice-a", Bitrate: 12345},
		{Voice: "voice-a", Channel: 3},
		{Voice: "voice-a", SubtitleType: "invalid"},
	}
	for _, req := range tests {
		if err := req.Validate(); err == nil {
			t.Fatalf("expected validation error for %+v", req)
		}
	}
}

func TestNestedRangesAreValidated(t *testing.T) {
	speed := 2.1
	req := StartRequest{
		Voice: "voice-a",
		VoiceSetting: &VoiceSetting{
			Speed: &speed,
		},
	}
	if err := req.Validate(); err == nil {
		t.Fatal("expected speed validation error")
	}
}

func TestEveryDocumentedEnumValueIsAccepted(t *testing.T) {
	formats := []AudioFormat{
		FormatMP3, FormatPCM, FormatWAV, FormatFLAC, FormatPCMURaw,
		FormatPCMUWAV,
	}
	for _, format := range formats {
		req := StartRequest{Voice: "voice-a", Format: format}
		if err := req.Validate(); err != nil {
			t.Fatalf("format %s: %v", format, err)
		}
	}
	for _, rate := range []int{8000, 16000, 22050, 24000, 32000, 44100} {
		if err := (StartRequest{
			Voice: "voice-a", SampleRate: rate,
		}).Validate(); err != nil {
			t.Fatalf("sample rate %d: %v", rate, err)
		}
	}
	for _, bitrate := range []int{32000, 64000, 128000, 256000} {
		if err := (StartRequest{
			Voice: "voice-a", Bitrate: bitrate,
		}).Validate(); err != nil {
			t.Fatalf("bitrate %d: %v", bitrate, err)
		}
	}
	for _, channel := range []int{1, 2} {
		if err := (StartRequest{
			Voice: "voice-a", Channel: channel,
		}).Validate(); err != nil {
			t.Fatalf("channel %d: %v", channel, err)
		}
	}
	for _, subtitle := range []SubtitleType{
		SubtitleSentence, SubtitleWord, SubtitleWordStreaming,
	} {
		if err := (StartRequest{
			Voice: "voice-a", SubtitleType: subtitle,
		}).Validate(); err != nil {
			t.Fatalf("subtitle %s: %v", subtitle, err)
		}
	}
}

func TestMP3OnlyFieldsAreRejectedForOtherFormats(t *testing.T) {
	pitch := 1
	for _, req := range []StartRequest{
		{Voice: "voice-a", Format: FormatWAV, Bitrate: 128000},
		{
			Voice: "voice-a", Format: FormatPCM,
			VoiceModify: &VoiceModify{Pitch: &pitch},
		},
	} {
		if err := req.Validate(); err == nil {
			t.Fatalf("expected validation error for %+v", req)
		}
	}
}

func ptr[T any](value T) *T {
	return &value
}
