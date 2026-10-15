# MyVocal V2 WebSocket TTS SDK for Go

Stream text to speech with `Skyblight-Turbo`. Requires Go 1.22 or later, an enterprise TTS API Key with dedicated concurrency, and a voice ID.

## Install

From your application's Go module:

```bash
go get github.com/MyVocal-AI/myvocal-v2-wss-sdk-go@v1.0.2
```

To use the [downloadable ZIP](https://github.com/MyVocal-AI/myvocal-v2-wss-sdk-go/releases/tag/v1.0.2), extract it beside your application and add these entries to your application's `go.mod`:

```go
require github.com/MyVocal-AI/myvocal-v2-wss-sdk-go v1.0.2

replace github.com/MyVocal-AI/myvocal-v2-wss-sdk-go => ../myvocal-v2-wss-go-sdk-1.0.2
```

Adjust the path to your extracted folder, then run `go mod tidy`.

## Run the example

Set your API Key and voice ID:

```bash
export MYVOCAL_TTS_API_KEY='YOUR_API_KEY'
export MYVOCAL_VOICE_ID='YOUR_VOICE_ID'
```

PowerShell:

```powershell
$env:MYVOCAL_TTS_API_KEY='YOUR_API_KEY'
$env:MYVOCAL_VOICE_ID='YOUR_VOICE_ID'
```

From the extracted SDK folder:

```bash
go run ./examples/basic
```

Running the example makes a real synthesis request and consumes character usage. It prints the final audio byte count and close code. To save or play audio, use the callbacks or the completed audio result described in the usage guide.

## Use in your application

Connect, start a task, send text fragments, and call finish. Finish waits for `task_finished` and Close Code `1000`. Sentence-end events and individual audio-final flags do not indicate completion of the whole task.

- [Usage guide](USAGE.md): options, audio callbacks, subtitles, cancellation, timeouts, and errors.
- [API reference](https://api-docs.myvocal.ai/3891085w0): request and response messages.
- [Character usage](https://api-docs.myvocal.ai/336818975e0).

The SDK does not automatically replay submitted text after a connection failure.

Licensed under [Apache License 2.0](LICENSE). Dependency notices are in [NOTICE](NOTICE).
