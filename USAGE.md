# V2 WebSocket TTS usage guide

Use the installation and example commands in [README](README.md).

## Connect and start a task

Endpoint:

```text
wss://openapi.myvocal.ai/v2/ai/speech/websocket
```

Use your enterprise TTS API Key and a voice ID available to your account. Authentication uses the `API-Key` handshake header. Store the Key in server-side configuration or an environment variable; do not include it in a URL, public source code, or logs.

This endpoint supports `Skyblight-Turbo` with `dedicated_concurrency`. The SDK sets these values when starting a task. It does not select ordinary TTS or dialogue models such as `flash_v2_5`, `myvocal_v4`, or `myvocal_v3_turbo`.

```text
connect -> start -> send text fragments -> optional flush -> finish
```

Wait for `start` to succeed before sending text. It waits for the `task_started` response. Java `connect` establishes the connection but may return `null` before the `connected_success` event arrives; a null return alone is not a connection failure.

## Task options

Create a `StartRequest` with the voice ID and any optional settings:

| Purpose | Python | Java builder | Go | Default / values |
| --- | --- | --- | --- | --- |
| Voice | `voice` | `StartRequest.builder(voice)` | `Voice` | Required |
| Language | `language` | `language(...)` | `Language` | Omit for automatic selection; for example `en`, `zh`, `ja` |
| Audio format | `format` | `format(...)` | `Format` | `mp3`; also `pcm`, `wav`, `flac`, `pcmu_raw`, `pcmu_wav` |
| Sample rate | `sample_rate` | `sampleRate(...)` | `SampleRate` | 32000 Hz; 8000, 16000, 22050, 24000, 32000, 44100 |
| MP3 bitrate | `bitrate` | `bitrate(...)` | `Bitrate` | 128000 bit/s; 32000, 64000, 128000, 256000 |
| Channels | `channel` | `channel(...)` | `Channel` | 1; 1 or 2 |
| Voice settings | `voice_setting` | `voiceSetting(...)` | `VoiceSetting` | Optional `VoiceSetting` |
| Subtitles | `subtitle_enable`, `subtitle_type` | `subtitle(enabled, type)` | `SubtitleEnable`, `SubtitleType` | Disabled; type defaults to `sentence` |
| Session label | `session_id` | `sessionId(...)` | `SessionID` | Optional application identifier |

Use `bitrate` only with MP3. `pcmu_raw` and `pcmu_wav` are 8 kHz G.711 mu-law; set the sample rate accordingly. Other advanced options include `PronunciationDict`, `TimbreWeight`, and `VoiceModify`. Voice modification is available only with MP3. A valid primary voice is still required when supplying timbre weights.

In `VoiceSetting`, speed accepts `0.5..2.0`, volume accepts values greater than `0` up to `10`, and pitch accepts `-12..12`. Volume is named `vol` in Python, `volume(...)` in Java, and `Volume` in Go. Emotions include `happy`, `sad`, `angry`, `fearful`, `disgusted`, `surprised`, and `calm`.

See the [API reference](https://api-docs.myvocal.ai/3891085w0) for all field descriptions and valid combinations.

## Send, flush, cancel, and finish

| Action | Python | Java | Go |
| --- | --- | --- | --- |
| Connect | `connect()` | `connect(timeout, unit)` | `Connect(ctx)` |
| Start task | `start(request)` | `start(request, timeout, unit)` | `Start(ctx, request)` |
| Add text | `send_text(text)` | `sendText(text)` | `SendText(text)` |
| Process buffered text now | `flush()` | `flush(timeout, unit)` | `Flush(ctx)` |
| Cancel pending synthesis | `cancel()` | `cancel(timeout, unit)` | `Cancel(ctx)` |
| Finish task | `finish()` | `finish(timeout, unit)` | `Finish(ctx)` |
| Close connection | `close()` | `close()` | `Close()` |

Send text fragments as they become available. Flush requests immediate processing of buffered text and waits for `task_flushed`; the task remains open. Cancel waits for `task_canceled` and leaves the connection available for more text. Cancellation does not mean all submitted text has been synthesized.

Finish processes remaining text, waits for `task_finished`, then waits for normal WebSocket closure with code `1000`. A separate flush is optional before finish. After finish, create a new client connection for another task. Closing the connection directly is cleanup, not a substitute for finish.

## Audio and completion events

| Signal | Meaning |
| --- | --- |
| `is_final=true` | Last audio chunk for the current audio request |
| `sentence_end` | The current sentence's audio has finished |
| `task_finished` followed by Close Code `1000` | The whole task has completed normally |

The SDK converts audio from the server's hexadecimal representation into bytes. Consume chunks in arrival order using the requested format and sample rate:

- Python: register `on_event`; decoded audio is available as `event.audio`.
- Java: implement `V2TtsListener` or extend `V2TtsAdapter`; use `onAudio(byte[], ServerEvent)` for decoded chunks.
- Go: provide `Handlers.OnAudio` to receive decoded bytes and the event.

`finish` also returns accumulated audio: Python `result.audio`, Java `result.getAudio()`, and Go `result.Audio`. Choose streaming chunks or the completed result for playback or file output; appending both duplicates the audio. The supplied examples print the final byte count and close code without saving a file.

Keep callbacks short. Move slow playback or file processing into your application and do not wait for blocking SDK operations inside callbacks. Python callbacks may be regular or async functions. Callback errors are reported through the error callback and do not by themselves change the synthesis result.

For subtitles, enable them and choose `sentence`, `word`, or `word_streaming`. Inspect the event data for subtitle and sentence information; an event with no audio can still carry useful status or subtitle data.

## Errors, timeouts, and retries

- Check request values and call order when validation or state errors occur.
- Check the enterprise API Key and account access for authentication errors.
- Handle `task_failed` and inspect available error codes or messages.
- Treat a timeout or disconnection before normal task completion as an incomplete operation.

Python's `operation_timeout` is in seconds and controls waits for task responses and closure. Java operations take a timeout plus `TimeUnit`. Go operations use `context.Context` deadlines. Use a sufficient timeout for the text length and always close connections when abandoning an operation.

The SDK does not automatically replay submitted text after a connection failure. Python `connect(retries=...)` can retry initial connection attempts; it does not resume an active task. Create a new client when retrying a failed task and account for possible duplicate audio and character usage.

## Usage and support

Use [Query primary key usage](https://api-docs.myvocal.ai/336818975e0) to view character usage. For support, provide the SDK version, failed operation, error code, and a non-sensitive session label when available. Do not share API Keys or private audio/text in public logs.
