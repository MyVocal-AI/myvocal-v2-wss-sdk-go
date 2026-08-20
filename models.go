package myvocalwss

import (
	"encoding/json"
	"errors"
	"fmt"
)

type AudioFormat string

const (
	FormatMP3     AudioFormat = "mp3"
	FormatPCM     AudioFormat = "pcm"
	FormatWAV     AudioFormat = "wav"
	FormatFLAC    AudioFormat = "flac"
	FormatPCMURaw AudioFormat = "pcmu_raw"
	FormatPCMUWAV AudioFormat = "pcmu_wav"
)

type SubtitleType string

const (
	SubtitleSentence      SubtitleType = "sentence"
	SubtitleWord          SubtitleType = "word"
	SubtitleWordStreaming SubtitleType = "word_streaming"
)

type VoiceSetting struct {
	Speed                *float64 `json:"speed,omitempty"`
	Volume               *float64 `json:"vol,omitempty"`
	Pitch                *int     `json:"pitch,omitempty"`
	Emotion              *string  `json:"emotion,omitempty"`
	EnglishNormalization *bool    `json:"english_normalization,omitempty"`
	LatexRead            *bool    `json:"latex_read,omitempty"`
}

type PronunciationDict struct {
	Tone []string `json:"tone"`
}

type TimbreWeight struct {
	VoiceID string `json:"voice_id"`
	Weight  int    `json:"weight"`
}

type VoiceModify struct {
	Pitch        *int    `json:"pitch,omitempty"`
	Intensity    *int    `json:"intensity,omitempty"`
	Timbre       *int    `json:"timbre,omitempty"`
	SoundEffects *string `json:"sound_effects,omitempty"`
}

type StartRequest struct {
	Voice             string
	Language          string
	Format            AudioFormat
	SampleRate        int
	Bitrate           int
	Channel           int
	VoiceSetting      *VoiceSetting
	PronunciationDict *PronunciationDict
	TimbreWeights     []TimbreWeight
	VoiceModify       *VoiceModify
	SubtitleEnable    *bool
	SubtitleType      SubtitleType
	SessionID         string
}

func (r StartRequest) Validate() error {
	if r.Voice == "" {
		return errors.New("voice is required")
	}
	format := r.Format
	if format == "" {
		format = FormatMP3
	}
	if !containsString([]string{
		"mp3", "pcm", "wav", "flac", "pcmu_raw", "pcmu_wav",
	}, string(format)) {
		return fmt.Errorf("unsupported format: %s", format)
	}
	sampleRate := r.SampleRate
	if sampleRate == 0 {
		sampleRate = 32000
	}
	if !containsInt(
		[]int{8000, 16000, 22050, 24000, 32000, 44100}, sampleRate,
	) {
		return fmt.Errorf("unsupported sample_rate: %d", sampleRate)
	}
	if r.Bitrate != 0 &&
		!containsInt([]int{32000, 64000, 128000, 256000}, r.Bitrate) {
		return fmt.Errorf("unsupported bitrate: %d", r.Bitrate)
	}
	channel := r.Channel
	if channel == 0 {
		channel = 1
	}
	if channel != 1 && channel != 2 {
		return fmt.Errorf("unsupported channel: %d", channel)
	}
	subtitleType := r.SubtitleType
	if subtitleType == "" {
		subtitleType = SubtitleSentence
	}
	if !containsString(
		[]string{"sentence", "word", "word_streaming"},
		string(subtitleType),
	) {
		return fmt.Errorf("unsupported subtitle_type: %s", subtitleType)
	}
	if len(r.TimbreWeights) > 4 {
		return errors.New("timbre_weights must contain 1 to 4 items")
	}
	for _, item := range r.TimbreWeights {
		if item.VoiceID == "" || item.Weight < 1 || item.Weight > 100 {
			return errors.New("invalid timbre weight")
		}
	}
	if r.VoiceModify != nil && format != FormatMP3 {
		return errors.New("voice_modify is supported only for mp3")
	}
	if r.Bitrate != 0 && format != FormatMP3 {
		return errors.New("bitrate is supported only for mp3")
	}
	if err := validateVoiceSetting(r.VoiceSetting); err != nil {
		return err
	}
	if err := validateVoiceModify(r.VoiceModify); err != nil {
		return err
	}
	if r.PronunciationDict != nil {
		for _, tone := range r.PronunciationDict.Tone {
			if tone == "" {
				return errors.New(
					"pronunciation_dict.tone contains an empty value",
				)
			}
		}
	}
	return nil
}

func (r StartRequest) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	format := r.Format
	if format == "" {
		format = FormatMP3
	}
	sampleRate := r.SampleRate
	if sampleRate == 0 {
		sampleRate = 32000
	}
	channel := r.Channel
	if channel == 0 {
		channel = 1
	}
	subtitleEnable := false
	if r.SubtitleEnable != nil {
		subtitleEnable = *r.SubtitleEnable
	}
	subtitleType := r.SubtitleType
	if subtitleType == "" {
		subtitleType = SubtitleSentence
	}
	wire := map[string]interface{}{
		"event":           "task_start",
		"model":           "Skyblight-Turbo",
		"voice":           r.Voice,
		"format":          format,
		"sample_rate":     sampleRate,
		"channel":         channel,
		"priority":        "dedicated_concurrency",
		"subtitle_enable": subtitleEnable,
		"subtitle_type":   subtitleType,
	}
	if format == FormatMP3 {
		bitrate := r.Bitrate
		if bitrate == 0 {
			bitrate = 128000
		}
		wire["bitrate"] = bitrate
	}
	if r.Language != "" {
		wire["language"] = r.Language
	}
	if r.VoiceSetting != nil {
		wire["voice_setting"] = r.VoiceSetting
	}
	if r.PronunciationDict != nil {
		wire["pronunciation_dict"] = r.PronunciationDict
	}
	if len(r.TimbreWeights) > 0 {
		wire["timbre_weights"] = r.TimbreWeights
	}
	if r.VoiceModify != nil {
		wire["voice_modify"] = r.VoiceModify
	}
	if r.SessionID != "" {
		wire["session_id"] = r.SessionID
	}
	return json.Marshal(wire)
}

func validateVoiceSetting(value *VoiceSetting) error {
	if value == nil {
		return nil
	}
	if value.Speed != nil && (*value.Speed < 0.5 || *value.Speed > 2) {
		return errors.New("voice_setting.speed is out of range")
	}
	if value.Volume != nil && (*value.Volume <= 0 || *value.Volume > 10) {
		return errors.New("voice_setting.vol is out of range")
	}
	if value.Pitch != nil && (*value.Pitch < -12 || *value.Pitch > 12) {
		return errors.New("voice_setting.pitch is out of range")
	}
	if value.Emotion != nil && !containsString([]string{
		"happy", "sad", "angry", "fearful", "disgusted", "surprised", "calm",
	}, *value.Emotion) {
		return fmt.Errorf("unsupported emotion: %s", *value.Emotion)
	}
	return nil
}

func validateVoiceModify(value *VoiceModify) error {
	if value == nil {
		return nil
	}
	for name, number := range map[string]*int{
		"pitch": value.Pitch, "intensity": value.Intensity,
		"timbre": value.Timbre,
	} {
		if number != nil && (*number < -100 || *number > 100) {
			return fmt.Errorf("voice_modify.%s is out of range", name)
		}
	}
	if value.SoundEffects != nil && !containsString([]string{
		"spacious_echo", "auditorium_echo", "lofi_telephone", "robotic",
	}, *value.SoundEffects) {
		return fmt.Errorf("unsupported sound effect: %s", *value.SoundEffects)
	}
	return nil
}

func containsInt(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
