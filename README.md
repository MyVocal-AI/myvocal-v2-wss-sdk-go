# MyVocal V2 WebSocket TTS SDK for Go

Idiomatic Go 1.22+ client for the MyVocal V2 bidirectional WebSocket TTS
endpoint.

## Install

From the downloaded ZIP, either run the included example directly:

```bash
go run ./examples/basic
```

Or reference the extracted SDK from an application `go.mod`:

```text
require github.com/MyVocal-AI/myvocal-v2-wss-sdk-go v0.0.0
replace github.com/MyVocal-AI/myvocal-v2-wss-sdk-go => ../myvocal-v2-wss-go-sdk-1.0.1
```

After the GitHub release is published:

```bash
go get github.com/MyVocal-AI/myvocal-v2-wss-sdk-go@v1.0.1
```

## Example

```go
ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
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
log.Printf("audio_bytes=%d close_code=%d", len(result.Audio), result.CloseCode)
```

The client sends `API-Key` only in the WebSocket handshake, validates the
documented request fields, decodes ordered hex audio, and treats
`task_finished` followed by close code `1000` as normal completion.

Handlers run synchronously on the WebSocket reader and preserve event order.
Keep them short. Handler panics are recovered and reported to `OnError`
without changing the TTS protocol result.

## License

Apache License 2.0. See `LICENSE` and `NOTICE` in the SDK package.
