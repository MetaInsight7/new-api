package helper

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFirstEventDeadlineEndsSilentUpstream(test *testing.T) {
	settings := config.GlobalConfig.Get("streaming_setting")
	previous, err := config.ConfigToMap(settings)
	require.NoError(test, err)
	require.NoError(test, config.UpdateConfigFromMap(settings, map[string]string{
		"first_response_timeout_enabled": "true", "first_response_timeout_seconds": "1",
	}))
	test.Cleanup(func() { require.NoError(test, config.UpdateConfigFromMap(settings, previous)) })
	synctest.Test(test, func(test *testing.T) {
		reader, writer := io.Pipe()
		defer writer.Close()
		requestContext, response, _ := setupStreamTest(test, reader)
		response.Body = reader
		info := relaycommon.GenRelayInfoOpenAI(requestContext, nil)
		info.DisablePing = true
		StreamScannerHandler(requestContext, response, info, func(data string, result *StreamResult) {
			test.Errorf("unexpected upstream data: %s", data)
		})
		assert.Equal(test, "first_response_timeout", string(info.StreamStatus.EndReason))
		_, err := writer.Write([]byte("data: late\n\n"))
		assert.Error(test, err)
	})
}

func TestFirstEventDeadlineIgnoresTransportActivity(test *testing.T) {
	previousPing := *operation_setting.GetGeneralSetting()
	operation_setting.GetGeneralSetting().PingIntervalEnabled = true
	operation_setting.GetGeneralSetting().PingIntervalSeconds = 1
	test.Cleanup(func() { *operation_setting.GetGeneralSetting() = previousPing })
	synctest.Test(test, func(test *testing.T) {
		reader, writer := io.Pipe()
		defer writer.Close()
		requestContext, response, info := setupStreamTest(test, reader)
		response.Body = reader
		info.FirstResponseTimeout = 3 * time.Second
		finished := make(chan *types.NewAPIError, 1)
		go func() {
			finished <- StreamScannerHandler(requestContext, response, info, func(data string, result *StreamResult) {
				test.Errorf("heartbeat became data: %s", data)
			})
		}()
		synctest.Wait()
		_, err := io.WriteString(writer, ": upstream keep-alive\n\nevent: ping\ndata: \n\n")
		require.NoError(test, err)
		<-time.NewTimer(time.Second).C
		synctest.Wait()
		_, err = io.WriteString(writer, ": another keep-alive\n\n")
		require.NoError(test, err)
		apiErr := <-finished
		require.NotNil(test, apiErr)
		assert.True(test, requestContext.Writer.Written())
		assert.Equal(test, http.StatusGatewayTimeout, apiErr.StatusCode)
		assert.Equal(test, "upstream_first_response_timeout", string(apiErr.GetErrorCode()))
		assert.Equal(test, relaycommon.StreamEndReasonFirstResponseTimeout, info.StreamStatus.EndReason)
		written := requestContext.Writer.Size()
		<-time.NewTimer(2 * time.Second).C
		synctest.Wait()
		assert.Equal(test, written, requestContext.Writer.Size(), "local pings must stop before returning")
	})
}

func TestFirstEventDeadlineStopsAtDataButNotTheGeneration(test *testing.T) {
	for _, data := range []string{
		`{"choices":[{"delta":{"role":"assistant"}}]}`,
		`{"choices":[{"delta":{"reasoning_content":"thinking"}}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"id":"call_1"}]}}]}`,
		`{"type":"response.created"}`,
		`{"type":"image_generation.partial_image"}`,
		`{"type":"unknown_accepted_event","note":"ping is only a word"}`,
	} {
		test.Run(data, func(test *testing.T) {
			synctest.Test(test, func(test *testing.T) {
				reader, writer := io.Pipe()
				defer writer.Close()
				requestContext, response, info := setupStreamTest(test, reader)
				response.Body = reader
				info.DisablePing = true
				info.FirstResponseTimeout = time.Second
				finished := make(chan *types.NewAPIError, 1)
				var observed []string
				go func() {
					finished <- StreamScannerHandler(requestContext, response, info, func(data string, result *StreamResult) { observed = append(observed, data) })
				}()
				synctest.Wait()
				_, err := io.WriteString(writer, "data: "+data+"\n\n")
				require.NoError(test, err)
				synctest.Wait()
				<-time.NewTimer(2 * time.Second).C
				synctest.Wait()
				select {
				case apiErr := <-finished:
					test.Fatalf("stream stopped after first event: %v", apiErr)
				default:
				}
				_, err = io.WriteString(writer, "data: [DONE]\n\n")
				require.NoError(test, err)
				require.Nil(test, <-finished)
				assert.Equal(test, []string{data}, observed)
				assert.Equal(test, relaycommon.StreamEndReasonDone, info.StreamStatus.EndReason)
			})
		})
	}
}

type headerCommitWriter struct {
	*httptest.ResponseRecorder
	headerStarted chan struct{}
	once          sync.Once
}

func (w *headerCommitWriter) WriteHeader(code int) {
	// Let the scanner start while the first keepalive snapshots the headers.
	w.once.Do(func() { close(w.headerStarted) })
	w.ResponseRecorder.WriteHeader(code)
}

func TestStreamScannerHandler_RequestKeepAliveHandoff(t *testing.T) {
	settings := operation_setting.GetGeneralSetting()
	previousEnabled, previousInterval := settings.PingIntervalEnabled, settings.PingIntervalSeconds
	settings.PingIntervalEnabled, settings.PingIntervalSeconds = true, 60
	t.Cleanup(func() {
		settings.PingIntervalEnabled, settings.PingIntervalSeconds = previousEnabled, previousInterval
	})

	for _, test := range []struct {
		name        string
		interval    time.Duration
		waitForPing bool
	}{
		{name: "before first keepalive", interval: time.Hour},
		{name: "during header commit", interval: time.Millisecond, waitForPing: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			writer := &headerCommitWriter{
				ResponseRecorder: httptest.NewRecorder(),
				headerStarted:    make(chan struct{}),
			}
			c, _ := gin.CreateTestContext(writer)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			stop := StartStreamKeepAlive(c, test.interval)
			defer stop()
			if test.waitForPing {
				select {
				case <-writer.headerStarted:
				case <-time.After(time.Second):
					t.Fatal("keepalive did not start")
				}
			}

			resp := &http.Response{
				Header: http.Header{
					"X-Reasoning-Included": {"true"},
					"X-Codex-Turn-State":   {"turn-state"},
				},
				Body: io.NopCloser(strings.NewReader("data: chunk\n\ndata: [DONE]\n\n")),
			}
			info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
			var received []string
			StreamScannerHandler(c, resp, info, func(data string, _ *StreamResult) {
				received = append(received, data)
				assert.NoError(t, StringData(c, data))
			})
			stop()

			require.NotNil(t, info.StreamStatus)
			assert.True(t, info.StreamStatus.IsNormalEnd())
			assert.Equal(t, []string{"chunk"}, received)
			assert.True(t, strings.HasSuffix(writer.Body.String(), "data: chunk\n\n"))
			if test.waitForPing {
				assert.True(t, strings.HasPrefix(writer.Body.String(), ": PING\n\n"))
			} else {
				assert.Equal(t, "data: chunk\n\n", writer.Body.String())
				assert.Equal(t, "true", writer.Result().Header.Get("X-Reasoning-Included"))
				assert.Equal(t, "turn-state", writer.Result().Header.Get("X-Codex-Turn-State"))
			}
		})
	}
}

func TestFirstEventDeadlinePreservesEndAndCancellation(test *testing.T) {
	for _, ending := range []string{"eof", "done", "cancel", "disabled"} {
		test.Run(ending, func(test *testing.T) {
			synctest.Test(test, func(test *testing.T) {
				reader, writer := io.Pipe()
				defer writer.Close()
				requestContext, response, info := setupStreamTest(test, reader)
				response.Body = reader
				info.DisablePing = true
				info.FirstResponseTimeout = time.Second
				request, cancel := context.WithCancel(requestContext.Request.Context())
				defer cancel()
				requestContext.Request = requestContext.Request.WithContext(request)
				if ending == "disabled" {
					info.FirstResponseTimeout = 0
				}
				finished := make(chan *types.NewAPIError, 1)
				go func() {
					finished <- StreamScannerHandler(requestContext, response, info, func(data string, result *StreamResult) {})
				}()
				synctest.Wait()
				wantReason := relaycommon.StreamEndReasonEOF
				switch ending {
				case "done":
					_, err := io.WriteString(writer, "data: [DONE]\n\n")
					require.NoError(test, err)
					wantReason = relaycommon.StreamEndReasonDone
				case "cancel":
					cancel()
					wantReason = relaycommon.StreamEndReasonClientGone
				case "disabled":
					<-time.NewTimer(2 * time.Second).C
					require.NoError(test, writer.Close())
				default:
					require.NoError(test, writer.Close())
				}
				require.Nil(test, <-finished)
				assert.Equal(test, wantReason, info.StreamStatus.EndReason)
			})
		})
	}
}

func TestFirstEventDeadlineUsesRequestSnapshotAndFreshAttempt(test *testing.T) {
	settings := config.GlobalConfig.Get("streaming_setting")
	previous, err := config.ConfigToMap(settings)
	require.NoError(test, err)
	require.NoError(test, config.UpdateConfigFromMap(settings, map[string]string{"first_response_timeout_enabled": "true", "first_response_timeout_seconds": "1"}))
	test.Cleanup(func() { require.NoError(test, config.UpdateConfigFromMap(settings, previous)) })
	synctest.Test(test, func(test *testing.T) {
		requestContext, response, _ := setupStreamTest(test, strings.NewReader("data: {\"choices\":[]}\n\n"))
		info := relaycommon.GenRelayInfoOpenAI(requestContext, nil)
		info.DisablePing = true
		require.Nil(test, StreamScannerHandler(requestContext, response, info, func(data string, result *StreamResult) { result.Stop(fmt.Errorf("attempt failed after data")) }))
		require.Positive(test, info.ReceivedResponseCount)
		require.NoError(test, config.UpdateConfigFromMap(settings, map[string]string{"first_response_timeout_enabled": "false", "first_response_timeout_seconds": "600"}))

		reader, writer := io.Pipe()
		defer writer.Close()
		response.Body = reader
		apiErr := StreamScannerHandler(requestContext, response, info, func(data string, result *StreamResult) {})
		require.NotNil(test, apiErr)
		assert.Equal(test, "upstream_first_response_timeout", string(apiErr.GetErrorCode()))

		newInfo := relaycommon.GenRelayInfoOpenAI(requestContext, nil)
		newInfo.DisablePing = true
		newReader, newWriter := io.Pipe()
		defer newWriter.Close()
		response.Body = newReader
		finished := make(chan *types.NewAPIError, 1)
		go func() {
			finished <- StreamScannerHandler(requestContext, response, newInfo, func(data string, result *StreamResult) {})
		}()
		synctest.Wait()
		<-time.NewTimer(2 * time.Second).C
		synctest.Wait()
		select {
		case apiErr := <-finished:
			test.Fatalf("new request did not use disabled policy: %v", apiErr)
		default:
		}
		require.NoError(test, newWriter.Close())
		require.Nil(test, <-finished)
	})
}

func TestFirstEventDeadlineRejectsLateProgressAfterTimeout(test *testing.T) {
	synctest.Test(test, func(test *testing.T) {
		reader, writer := io.Pipe()
		defer writer.Close()
		requestContext, response, info := setupStreamTest(test, reader)
		response.Body = reader
		info.DisablePing = true
		info.FirstResponseTimeout = time.Second
		classifying, release := make(chan struct{}), make(chan struct{})
		finished := make(chan *types.NewAPIError, 1)
		var forwarded atomic.Bool
		go func() {
			finished <- StreamScannerHandler(requestContext, response, info, func(data string, result *StreamResult) { forwarded.Store(true) }, func(data string) bool {
				close(classifying)
				<-release
				return false
			})
		}()
		synctest.Wait()
		_, err := io.WriteString(writer, "data: {\"role\":\"assistant\"}\n\n")
		require.NoError(test, err)
		<-classifying
		<-time.NewTimer(2 * time.Second).C
		close(release)
		apiErr := <-finished
		require.NotNil(test, apiErr)
		assert.Equal(test, "upstream_first_response_timeout", string(apiErr.GetErrorCode()))
		assert.False(test, forwarded.Load())
		assert.Equal(test, relaycommon.StreamEndReasonFirstResponseTimeout, info.StreamStatus.EndReason)
	})
}

func TestFirstEventDeadlineClosesUpstreamWhileHeartbeatForwardingIsBlocked(test *testing.T) {
	synctest.Test(test, func(test *testing.T) {
		reader, writer := io.Pipe()
		defer writer.Close()
		requestContext, response, info := setupStreamTest(test, reader)
		response.Body = reader
		info.DisablePing = true
		info.FirstResponseTimeout = time.Second
		blocked, release := make(chan struct{}), make(chan struct{})
		finished := make(chan *types.NewAPIError, 1)
		go func() {
			finished <- StreamScannerHandler(requestContext, response, info, func(data string, result *StreamResult) {
				if data == "heartbeat" {
					close(blocked)
					<-release
				}
			}, func(data string) bool { return data == "heartbeat" })
		}()
		synctest.Wait()
		_, err := io.WriteString(writer, "data: heartbeat\n\n")
		require.NoError(test, err)
		<-blocked
		<-time.NewTimer(2 * time.Second).C
		synctest.Wait()
		_, err = io.WriteString(writer, "data: late response\n\n")
		assert.Error(test, err, "downstream backpressure must not delay closing the stalled upstream")
		close(release)
		_ = writer.Close()
		apiErr := <-finished
		require.NotNil(test, apiErr)
		assert.Equal(test, "upstream_first_response_timeout", string(apiErr.GetErrorCode()))
	})
}

func TestLargeEventSlowHandlerPreservesQueuedUsageAfterCancellation(test *testing.T) {
	synctest.Test(test, func(test *testing.T) {
		reader, writer := io.Pipe()
		defer writer.Close()
		requestContext, response, info := setupStreamTest(test, reader)
		response.Body = reader
		info.DisablePing = true
		request, cancel := context.WithCancel(requestContext.Request.Context())
		defer cancel()
		requestContext.Request = requestContext.Request.WithContext(request)
		blocked, release := make(chan struct{}), make(chan struct{})
		finished := make(chan *types.NewAPIError, 1)
		var observed []string
		largeContent := "content:" + strings.Repeat("x", 2<<20)
		go func() {
			finished <- StreamScannerHandler(requestContext, response, info, func(data string, result *StreamResult) {
				observed = append(observed, data)
				if strings.HasPrefix(data, "content:") {
					close(blocked)
					<-release
				}
			})
		}()
		synctest.Wait()
		_, err := io.WriteString(writer, "data: "+largeContent+"\n\n")
		require.NoError(test, err)
		<-blocked
		_, err = io.WriteString(writer, "data: authoritative usage\n\n")
		require.NoError(test, err)
		synctest.Wait()
		cancel()
		synctest.Wait()
		close(release)
		require.Nil(test, <-finished)
		require.Len(test, observed, 2)
		assert.Equal(test, largeContent, observed[0])
		assert.Equal(test, "authoritative usage", observed[1])
	})
}

func TestCompletedEventWinsConcurrentContextCancellation(test *testing.T) {
	synctest.Test(test, func(test *testing.T) {
		reader, writer := io.Pipe()
		defer writer.Close()
		requestContext, response, info := setupStreamTest(test, reader)
		response.Body = reader
		info.DisablePing = true
		request, cancel := context.WithCancel(requestContext.Request.Context())
		defer cancel()
		requestContext.Request = requestContext.Request.WithContext(request)
		blocked, release := make(chan struct{}), make(chan struct{})
		finished := make(chan *types.NewAPIError, 1)
		go func() {
			finished <- StreamScannerHandler(requestContext, response, info, func(data string, result *StreamResult) {
				info.StreamStatus.MarkCompleted()
				close(blocked)
				<-release
				result.Done()
			})
		}()
		synctest.Wait()
		_, err := io.WriteString(writer, "data: terminal event\n\n")
		require.NoError(test, err)
		<-blocked
		cancel()
		synctest.Wait()
		close(release)
		require.Nil(test, <-finished)
		assert.Equal(test, relaycommon.StreamEndReasonDone, info.StreamStatus.EndReason)
		assert.NoError(test, info.StreamStatus.EndError)
	})
}

func init() {
	gin.SetMode(gin.TestMode)
	if constant.StreamingTimeout == 0 {
		constant.StreamingTimeout = 30
	}
}

func setupStreamTest(t *testing.T, body io.Reader) (*gin.Context, *http.Response, *relaycommon.RelayInfo) {
	t.Helper()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	resp := &http.Response{
		Body: io.NopCloser(body),
	}

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{},
	}

	return c, resp, info
}

func buildSSEBody(n int) string {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "data: {\"id\":%d,\"choices\":[{\"delta\":{\"content\":\"token_%d\"}}]}\n", i, i)
	}
	b.WriteString("data: [DONE]\n")
	return b.String()
}

// ---------- Basic correctness ----------

func TestStreamScannerHandler_NilInputs(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}

	StreamScannerHandler(c, nil, info, func(data string, sr *StreamResult) {})
	StreamScannerHandler(c, &http.Response{Body: io.NopCloser(strings.NewReader(""))}, info, nil)
}

func TestNewStreamScanner_AllowsLargeStreamLine(t *testing.T) {
	oldBufferMB := constant.StreamScannerMaxBufferMB
	constant.StreamScannerMaxBufferMB = 1
	t.Cleanup(func() {
		constant.StreamScannerMaxBufferMB = oldBufferMB
	})

	payload := strings.Repeat("x", 128<<10)
	scanner := NewStreamScanner(strings.NewReader("data: " + payload + "\n"))
	scanner.Split(bufio.ScanLines)

	require.True(t, scanner.Scan())
	assert.Equal(t, "data: "+payload, scanner.Text())
	require.NoError(t, scanner.Err())
}

func TestStreamScannerHandler_EmptyBody(t *testing.T) {
	t.Parallel()

	c, resp, info := setupStreamTest(t, strings.NewReader(""))

	var called atomic.Bool
	StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {
		called.Store(true)
	})

	assert.False(t, called.Load(), "handler should not be called for empty body")
}

func TestStreamScannerHandler_1000Chunks(t *testing.T) {
	t.Parallel()

	const numChunks = 1000
	body := buildSSEBody(numChunks)
	c, resp, info := setupStreamTest(t, strings.NewReader(body))

	var count atomic.Int64
	StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {
		count.Add(1)
	})

	assert.Equal(t, int64(numChunks), count.Load())
	assert.Equal(t, numChunks, info.ReceivedResponseCount)
}

func TestStreamScannerHandler_OrderPreserved(t *testing.T) {
	t.Parallel()

	const numChunks = 500
	body := buildSSEBody(numChunks)
	c, resp, info := setupStreamTest(t, strings.NewReader(body))

	var mu sync.Mutex
	received := make([]string, 0, numChunks)

	StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {
		mu.Lock()
		received = append(received, data)
		mu.Unlock()
	})

	require.Equal(t, numChunks, len(received))
	for i := range numChunks {
		expected := fmt.Sprintf("{\"id\":%d,\"choices\":[{\"delta\":{\"content\":\"token_%d\"}}]}", i, i)
		assert.Equal(t, expected, received[i], "chunk %d out of order", i)
	}
}

func TestStreamScannerHandler_DoneStopsScanner(t *testing.T) {
	t.Parallel()

	body := buildSSEBody(50) + "data: should_not_appear\n"
	c, resp, info := setupStreamTest(t, strings.NewReader(body))

	var count atomic.Int64
	StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {
		count.Add(1)
	})

	assert.Equal(t, int64(50), count.Load(), "data after [DONE] must not be processed")
}

func TestStreamScannerHandler_StopStopsStream(t *testing.T) {
	t.Parallel()

	const numChunks = 200
	body := buildSSEBody(numChunks)
	c, resp, info := setupStreamTest(t, strings.NewReader(body))

	const stopAt int64 = 50
	var count atomic.Int64
	StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {
		n := count.Add(1)
		if n >= stopAt {
			sr.Stop(fmt.Errorf("fatal at %d", n))
		}
	})

	assert.Equal(t, stopAt, count.Load())
	require.NotNil(t, info.StreamStatus)
	assert.Equal(t, relaycommon.StreamEndReasonHandlerStop, info.StreamStatus.EndReason)
}

func TestStreamScannerHandler_SkipsNonDataLines(t *testing.T) {
	t.Parallel()

	var b strings.Builder
	b.WriteString(": comment line\n")
	b.WriteString("event: message\n")
	b.WriteString("id: 12345\n")
	b.WriteString("retry: 5000\n")
	for i := range 100 {
		fmt.Fprintf(&b, "data: payload_%d\n", i)
		b.WriteString(": interleaved comment\n")
	}
	b.WriteString("data: [DONE]\n")

	c, resp, info := setupStreamTest(t, strings.NewReader(b.String()))

	var count atomic.Int64
	StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {
		count.Add(1)
	})

	assert.Equal(t, int64(100), count.Load())
}

func TestStreamScannerHandler_DataWithExtraSpaces(t *testing.T) {
	t.Parallel()

	body := "data:   {\"trimmed\":true}  \ndata: [DONE]\n"
	c, resp, info := setupStreamTest(t, strings.NewReader(body))

	var got string
	StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {
		got = data
	})

	assert.Equal(t, "{\"trimmed\":true}", got)
}

// TestStreamScannerHandler_ClientCancelAbortsUpstreamAndReturns pins the
// disconnect contract: when the client goes away, the handler must return
// promptly (all goroutines joined, so the gin.Context can never leak into a
// pooled reuse), the upstream body must be closed to stop token generation,
// and no data received after the disconnect may be processed or written.
func TestStreamScannerHandler_ClientCancelAbortsUpstreamAndReturns(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pr, pw := io.Pipe()
	t.Cleanup(func() {
		_ = pr.Close()
		_ = pw.Close()
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(ctx)

	resp := &http.Response{Body: pr}
	info := &relaycommon.RelayInfo{
		DisablePing: true,
		ChannelMeta: &relaycommon.ChannelMeta{},
	}

	var count atomic.Int64
	firstHandled := make(chan struct{})
	done := make(chan struct{})
	go func() {
		StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {
			count.Add(1)
			_ = StringData(c, data)
			if data == "first" {
				close(firstHandled)
			}
		})
		close(done)
	}()

	_, err := fmt.Fprint(pw, "data: first\n")
	require.NoError(t, err)

	select {
	case <-firstHandled:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first chunk")
	}

	cancel()

	// The handler must return without any further upstream input: cleanup
	// closes resp.Body, which unblocks the scanner goroutine.
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not return after client disconnect")
	}

	// Upstream read side must be closed so the provider stops generating
	// (and billing) for a request nobody is listening to.
	_, err = fmt.Fprint(pw, "data: second\n")
	require.ErrorIs(t, err, io.ErrClosedPipe, "upstream body should be closed after client disconnect")

	assert.Equal(t, int64(1), count.Load(), "no chunk after disconnect should be processed")
	require.NotNil(t, info.StreamStatus)
	assert.Equal(t, relaycommon.StreamEndReasonClientGone, info.StreamStatus.EndReason)

	body := recorder.Body.String()
	assert.Contains(t, body, "first")
	assert.NotContains(t, body, "second")
}

// ---------- Ping tests ----------

func TestStreamScannerHandler_PingSentDuringSlowUpstream(t *testing.T) {
	setting := operation_setting.GetGeneralSetting()
	oldEnabled := setting.PingIntervalEnabled
	oldSeconds := setting.PingIntervalSeconds
	setting.PingIntervalEnabled = true
	setting.PingIntervalSeconds = 1
	t.Cleanup(func() {
		setting.PingIntervalEnabled = oldEnabled
		setting.PingIntervalSeconds = oldSeconds
	})

	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		for i := range 4 {
			fmt.Fprintf(pw, "data: chunk_%d\n", i)
			time.Sleep(400 * time.Millisecond)
		}
		fmt.Fprint(pw, "data: [DONE]\n")
	}()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	resp := &http.Response{Body: pr}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}

	var count atomic.Int64
	done := make(chan struct{})
	go func() {
		StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {
			count.Add(1)
		})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for stream to finish")
	}

	assert.Equal(t, int64(4), count.Load())

	body := recorder.Body.String()
	pingCount := strings.Count(body, ": PING")
	assert.GreaterOrEqual(t, pingCount, 1,
		"expected at least 1 ping during slow stream with 1s interval; got %d", pingCount)
}

func TestStreamScannerHandler_PingDisabledByRelayInfo(t *testing.T) {
	setting := operation_setting.GetGeneralSetting()
	oldEnabled := setting.PingIntervalEnabled
	oldSeconds := setting.PingIntervalSeconds
	setting.PingIntervalEnabled = true
	setting.PingIntervalSeconds = 1
	t.Cleanup(func() {
		setting.PingIntervalEnabled = oldEnabled
		setting.PingIntervalSeconds = oldSeconds
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	resp := &http.Response{Body: io.NopCloser(strings.NewReader(buildSSEBody(5)))}
	info := &relaycommon.RelayInfo{
		DisablePing: true,
		ChannelMeta: &relaycommon.ChannelMeta{},
	}

	var count atomic.Int64
	done := make(chan struct{})
	go func() {
		StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {
			count.Add(1)
		})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}

	assert.Equal(t, int64(5), count.Load())

	body := recorder.Body.String()
	pingCount := strings.Count(body, ": PING")
	assert.Equal(t, 0, pingCount, "pings should be disabled when DisablePing=true")
}

// ---------- StreamStatus integration ----------

func TestStreamScannerHandler_StreamStatus_DoneReason(t *testing.T) {
	t.Parallel()

	body := buildSSEBody(10)
	c, resp, info := setupStreamTest(t, strings.NewReader(body))

	StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {})

	require.NotNil(t, info.StreamStatus)
	assert.Equal(t, relaycommon.StreamEndReasonDone, info.StreamStatus.EndReason)
	assert.Nil(t, info.StreamStatus.EndError)
	assert.True(t, info.StreamStatus.IsNormalEnd())
	assert.False(t, info.StreamStatus.HasErrors())
}

func TestStreamScannerHandler_StreamStatus_EOFWithoutDone(t *testing.T) {
	t.Parallel()

	var b strings.Builder
	for i := range 5 {
		fmt.Fprintf(&b, "data: {\"id\":%d}\n", i)
	}
	c, resp, info := setupStreamTest(t, strings.NewReader(b.String()))

	StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {})

	require.NotNil(t, info.StreamStatus)
	assert.Equal(t, relaycommon.StreamEndReasonEOF, info.StreamStatus.EndReason)
	assert.True(t, info.StreamStatus.IsNormalEnd())
}

func TestStreamScannerHandler_StreamStatus_HandlerStop(t *testing.T) {
	t.Parallel()

	body := buildSSEBody(100)
	c, resp, info := setupStreamTest(t, strings.NewReader(body))

	var count atomic.Int64
	StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {
		n := count.Add(1)
		if n >= 10 {
			sr.Stop(fmt.Errorf("stop at 10"))
		}
	})

	require.NotNil(t, info.StreamStatus)
	assert.Equal(t, relaycommon.StreamEndReasonHandlerStop, info.StreamStatus.EndReason)
	assert.True(t, info.StreamStatus.HasErrors())
}

func TestStreamScannerHandler_StreamStatus_HandlerDone(t *testing.T) {
	t.Parallel()

	body := buildSSEBody(20)
	c, resp, info := setupStreamTest(t, strings.NewReader(body))

	var count atomic.Int64
	StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {
		n := count.Add(1)
		if n >= 5 {
			sr.Done()
		}
	})

	assert.Equal(t, int64(5), count.Load())
	require.NotNil(t, info.StreamStatus)
	assert.Equal(t, relaycommon.StreamEndReasonDone, info.StreamStatus.EndReason)
	assert.False(t, info.StreamStatus.HasErrors())
}

func TestStreamScannerHandler_StreamStatus_Timeout(t *testing.T) {
	// Not parallel: modifies global constant.StreamingTimeout
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 1
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	pr, pw := io.Pipe()
	go func() {
		fmt.Fprint(pw, "data: {\"id\":1}\n")
		time.Sleep(2 * time.Second)
		pw.Close()
	}()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	resp := &http.Response{Body: pr}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}

	done := make(chan struct{})
	go func() {
		StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for stream timeout")
	}

	require.NotNil(t, info.StreamStatus)
	assert.Equal(t, relaycommon.StreamEndReasonTimeout, info.StreamStatus.EndReason)
	assert.False(t, info.StreamStatus.IsNormalEnd())
}

func TestStreamScannerHandler_StreamStatus_SoftErrors(t *testing.T) {
	t.Parallel()

	body := buildSSEBody(10)
	c, resp, info := setupStreamTest(t, strings.NewReader(body))

	StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {
		sr.Error(fmt.Errorf("soft error for chunk"))
	})

	require.NotNil(t, info.StreamStatus)
	assert.Equal(t, relaycommon.StreamEndReasonDone, info.StreamStatus.EndReason)
	assert.True(t, info.StreamStatus.HasErrors())
	assert.Equal(t, 10, info.StreamStatus.TotalErrorCount())
}

func TestStreamScannerHandler_StreamStatus_MultipleErrorsPerChunk(t *testing.T) {
	t.Parallel()

	body := buildSSEBody(5)
	c, resp, info := setupStreamTest(t, strings.NewReader(body))

	StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {
		sr.Error(fmt.Errorf("error A"))
		sr.Error(fmt.Errorf("error B"))
	})

	require.NotNil(t, info.StreamStatus)
	assert.Equal(t, relaycommon.StreamEndReasonDone, info.StreamStatus.EndReason)
	assert.Equal(t, 10, info.StreamStatus.TotalErrorCount())
}

func TestStreamScannerHandler_StreamStatus_ErrorThenStop(t *testing.T) {
	t.Parallel()

	// Use a large body without [DONE] to avoid race between scanner's [DONE]
	// and handler's Stop on the sync.Once EndReason.
	var b strings.Builder
	for i := range 100 {
		fmt.Fprintf(&b, "data: {\"id\":%d}\n", i)
	}
	c, resp, info := setupStreamTest(t, strings.NewReader(b.String()))

	var count atomic.Int64
	StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {
		count.Add(1)
		sr.Error(fmt.Errorf("soft error"))
		sr.Stop(fmt.Errorf("fatal"))
	})

	assert.Equal(t, int64(1), count.Load())
	require.NotNil(t, info.StreamStatus)
	assert.Equal(t, relaycommon.StreamEndReasonHandlerStop, info.StreamStatus.EndReason)
	assert.Equal(t, 2, info.StreamStatus.TotalErrorCount())
}

func TestStreamScannerHandler_StreamStatus_InitializedIfNil(t *testing.T) {
	t.Parallel()

	body := buildSSEBody(1)
	c, resp, info := setupStreamTest(t, strings.NewReader(body))

	assert.Nil(t, info.StreamStatus)

	StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {})

	assert.NotNil(t, info.StreamStatus)
}

func TestStreamScannerHandler_StreamStatus_ReplacesPreInitialized(t *testing.T) {
	t.Parallel()

	body := buildSSEBody(5)
	c, resp, info := setupStreamTest(t, strings.NewReader(body))

	info.StreamStatus = relaycommon.NewStreamStatus()
	info.StreamStatus.RecordError("pre-existing error")

	StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {})

	assert.Equal(t, relaycommon.StreamEndReasonDone, info.StreamStatus.EndReason)
	assert.Equal(t, 0, info.StreamStatus.TotalErrorCount())
}

func TestNewStreamScannerCallerLimit(t *testing.T) {
	// The smaller buffer must actually constrain a line; a preallocated 64 KiB
	// buffer would otherwise bypass this caller's 1 KiB limit in bufio.Scanner.
	scanner := NewStreamScanner(strings.NewReader(strings.Repeat("x", 2048)+"\n"), 1024)
	assert.False(t, scanner.Scan())
	require.Error(t, scanner.Err())
	scanner = NewStreamScanner(strings.NewReader("data: ok\n"), 1024)
	require.True(t, scanner.Scan())
	assert.Equal(t, "data: ok", scanner.Text())
	require.NoError(t, scanner.Err())
}

// ---------- Write deadlines ----------

type deadlineRecorder struct {
	*httptest.ResponseRecorder
	deadlines []time.Time
}

func (r *deadlineRecorder) SetWriteDeadline(deadline time.Time) error {
	r.deadlines = append(r.deadlines, deadline)
	return nil
}

func TestStreamWriteBudget(t *testing.T) {
	start := time.Unix(1_800_000_000, 0)
	var budget StreamWriteBudget
	steps := []struct {
		name  string
		at    time.Duration
		bytes int
		want  time.Duration
	}{
		{"small event", 0, 8 << 10, 31 * time.Second},
		{"image event gets the time a slow client needs", 0, 3600 << 10, 481 * time.Second},
		{"next write keeps the image budget", 60 * time.Second, 8 << 10, 482 * time.Second},
		{"budget starts over once it ran out", 600 * time.Second, 8 << 10, 631 * time.Second},
	}
	for _, step := range steps {
		assert.Equal(t, start.Add(step.want), budget.Deadline(start.Add(step.at), step.bytes), step.name)
	}
}

func TestSetEventStreamHeadersBoundsStreamWritesByBudget(t *testing.T) {
	recorder := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	SetEventStreamHeaders(c)

	before := time.Now()
	require.NoError(t, StringData(c, strings.Repeat("A", 3600<<10)))

	require.NotEmpty(t, recorder.deadlines)
	assert.False(t, recorder.deadlines[0].Before(before.Add(streamWriteTimeout+450*time.Second)))
	assert.Contains(t, recorder.Body.String(), "data: AAAA")
}
