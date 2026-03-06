package qq

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/memohai/memoh/internal/channel"
	"github.com/memohai/memoh/internal/media"
)

func TestQQSendTextReply(t *testing.T) {
	t.Parallel()

	var tokenCalls int
	var messageBodies []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app/getAppAccessToken":
			tokenCalls++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "token-1",
				"expires_in":   7200,
			})
		case "/v2/users/user-openid/messages":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode message body: %v", err)
			}
			messageBodies = append(messageBodies, body)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "m-1"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	adapter := newTestQQAdapter(server)
	err := adapter.Send(context.Background(), channel.ChannelConfig{
		ID:    "cfg-1",
		BotID: "bot-1",
		Credentials: map[string]any{
			"appId":        "1024",
			"clientSecret": "secret",
		},
	}, channel.OutboundMessage{
		Target: "c2c:user-openid",
		Message: channel.Message{
			Text:  "hello",
			Reply: &channel.ReplyRef{MessageID: "source-msg"},
		},
	})
	if err != nil {
		t.Fatalf("send text: %v", err)
	}

	if tokenCalls != 1 {
		t.Fatalf("unexpected token calls: %d", tokenCalls)
	}
	if len(messageBodies) != 1 {
		t.Fatalf("unexpected message calls: %d", len(messageBodies))
	}
	if messageBodies[0]["msg_id"] != "source-msg" {
		t.Fatalf("unexpected msg_id: %#v", messageBodies[0]["msg_id"])
	}
	if messageBodies[0]["msg_type"] != float64(0) {
		t.Fatalf("unexpected msg_type: %#v", messageBodies[0]["msg_type"])
	}
	if messageBodies[0]["content"] != "hello" {
		t.Fatalf("unexpected content: %#v", messageBodies[0]["content"])
	}
}

func TestQQSendImageAttachment(t *testing.T) {
	t.Parallel()

	var uploadBody map[string]any
	var messageBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app/getAppAccessToken":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "token-1",
				"expires_in":   7200,
			})
		case "/v2/groups/group-openid/files":
			if err := json.NewDecoder(r.Body).Decode(&uploadBody); err != nil {
				t.Fatalf("decode upload body: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"file_info": "file-info-1",
			})
		case "/v2/groups/group-openid/messages":
			if err := json.NewDecoder(r.Body).Decode(&messageBody); err != nil {
				t.Fatalf("decode message body: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "m-2"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	adapter := newTestQQAdapter(server)
	err := adapter.Send(context.Background(), channel.ChannelConfig{
		ID:    "cfg-2",
		BotID: "bot-2",
		Credentials: map[string]any{
			"appId":        "2048",
			"clientSecret": "secret",
		},
	}, channel.OutboundMessage{
		Target: "group:group-openid",
		Message: channel.Message{
			Attachments: []channel.Attachment{{
				Type: channel.AttachmentImage,
				URL:  "https://example.com/image.png",
			}},
			Reply: &channel.ReplyRef{MessageID: "source-msg"},
		},
	})
	if err != nil {
		t.Fatalf("send attachment: %v", err)
	}

	if uploadBody["file_type"] != float64(qqMediaTypeImage) {
		t.Fatalf("unexpected file_type: %#v", uploadBody["file_type"])
	}
	if uploadBody["url"] != "https://example.com/image.png" {
		t.Fatalf("unexpected upload url: %#v", uploadBody["url"])
	}
	if messageBody["msg_type"] != float64(7) {
		t.Fatalf("unexpected msg_type: %#v", messageBody["msg_type"])
	}
	if messageBody["msg_id"] != "source-msg" {
		t.Fatalf("unexpected msg_id: %#v", messageBody["msg_id"])
	}
}

func TestQQProcessingStartedSendsInputHintForDirectMessages(t *testing.T) {
	t.Parallel()

	var messageBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app/getAppAccessToken":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "token-1",
				"expires_in":   7200,
			})
		case "/v2/users/user-openid/messages":
			if err := json.NewDecoder(r.Body).Decode(&messageBody); err != nil {
				t.Fatalf("decode notify body: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "m-3"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	adapter := newTestQQAdapter(server)
	_, err := adapter.ProcessingStarted(context.Background(), channel.ChannelConfig{
		ID: "cfg-3",
		Credentials: map[string]any{
			"appId":           "4096",
			"clientSecret":    "secret",
			"enableInputHint": true,
		},
	}, channel.InboundMessage{}, channel.ProcessingStatusInfo{
		ReplyTarget:     "c2c:user-openid",
		SourceMessageID: "source-msg",
	})
	if err != nil {
		t.Fatalf("processing started: %v", err)
	}
	if messageBody["msg_type"] != float64(6) {
		t.Fatalf("unexpected msg_type: %#v", messageBody["msg_type"])
	}
	if messageBody["msg_id"] != "source-msg" {
		t.Fatalf("unexpected msg_id: %#v", messageBody["msg_id"])
	}
}

func TestQQSendChannelImageFromStoredAsset(t *testing.T) {
	t.Parallel()

	var messageBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app/getAppAccessToken":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "token-1",
				"expires_in":   7200,
			})
		case "/channels/channel-1/messages":
			if err := json.NewDecoder(r.Body).Decode(&messageBody); err != nil {
				t.Fatalf("decode channel body: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "m-4"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	adapter := newTestQQAdapter(server)
	adapter.SetAssetOpener(testAssetOpener{
		data: []byte("png-bytes"),
		asset: media.Asset{
			ContentHash: "hash-1",
			BotID:       "bot-4",
			Mime:        "image/png",
		},
	})

	err := adapter.Send(context.Background(), channel.ChannelConfig{
		ID:    "cfg-4",
		BotID: "bot-4",
		Credentials: map[string]any{
			"appId":        "8192",
			"clientSecret": "secret",
		},
	}, channel.OutboundMessage{
		Target: "channel:channel-1",
		Message: channel.Message{
			Attachments: []channel.Attachment{{
				Type:        channel.AttachmentImage,
				ContentHash: "hash-1",
				Name:        "image.png",
			}},
		},
	})
	if err != nil {
		t.Fatalf("send stored channel image: %v", err)
	}

	want := "![](data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("png-bytes")) + ")"
	if messageBody["content"] != want {
		t.Fatalf("unexpected channel content: %#v", messageBody["content"])
	}
}

type testAssetOpener struct {
	data  []byte
	asset media.Asset
}

func (t testAssetOpener) Open(context.Context, string, string) (io.ReadCloser, media.Asset, error) {
	return io.NopCloser(bytes.NewReader(t.data)), t.asset, nil
}

func newTestQQAdapter(server *httptest.Server) *QQAdapter {
	adapter := NewQQAdapter(nil)
	adapter.httpClient = server.Client()
	adapter.apiBaseURL = server.URL
	adapter.tokenURL = server.URL + "/app/getAppAccessToken"
	return adapter
}
