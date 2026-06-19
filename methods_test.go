package gogram_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/darxnet/gogram"
)

// ---------------------------------------------------------------------------
// RichText.UnmarshalJSON — pure unit tests, no HTTP involved.
// ---------------------------------------------------------------------------

// TestRichText_UnmarshalJSON_PlainString verifies that a bare JSON string
// is decoded into RichText.Plain.
func TestRichText_UnmarshalJSON_PlainString(t *testing.T) {
	t.Parallel()

	var rt gogram.RichText
	if err := json.Unmarshal([]byte(`"hello world"`), &rt); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rt.Plain != "hello world" {
		t.Errorf("Plain: got %q, want %q", rt.Plain, "hello world")
	}
}

// TestRichText_UnmarshalJSON_Array verifies that a JSON array of rich-text
// items is decoded into RichText.Slice.
func TestRichText_UnmarshalJSON_Array(t *testing.T) {
	t.Parallel()

	data := `[{"type":"bold","text":"hello"},{"type":"italic","text":"world"}]`
	var rt gogram.RichText
	if err := json.Unmarshal([]byte(data), &rt); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rt.Slice) != 2 {
		t.Errorf("Slice len: got %d, want 2", len(rt.Slice))
	}
}

// TestRichText_UnmarshalJSON_TypedObject verifies that a JSON object with a
// "type" discriminator is decoded into the appropriate subtype pointer.
func TestRichText_UnmarshalJSON_TypedObject(t *testing.T) {
	t.Parallel()

	data := `{"type":"bold","text":"hello"}`
	var rt gogram.RichText
	if err := json.Unmarshal([]byte(data), &rt); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rt.RichTextBold == nil {
		t.Fatal("expected RichTextBold to be set")
	}
}

// ---------------------------------------------------------------------------
// SendRichMessage — JSON body (no file uploads; always application/json).
// ---------------------------------------------------------------------------

// TestSendRichMessage_JSONBody verifies that (*Context).SendRichMessage issues
// an application/json POST to /sendRichMessage and encodes InputRichMessage
// into the "rich_message" JSON field.
func TestSendRichMessage_JSONBody(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if !strings.HasSuffix(req.URL.Path, "/sendRichMessage") {
			t.Errorf("unexpected path: %s", req.URL.Path)
		}
		if ct := req.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type: got %q, want application/json", ct)
		}
		b, _ := io.ReadAll(req.Body)
		capturedBody = b

		msg := gogram.Message{Text: "rich"}
		resp := gogram.Response{OK: true, Result: json.RawMessage(mustMarshal(t, &msg))}
		return jsonHTTPResponse(t, http.StatusOK, &resp), nil
	})

	client, err := gogram.NewClient(testToken,
		gogram.WithHost("example.invalid"),
		gogram.WithHTTPClient(&http.Client{Transport: transport}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ctx := gogram.NewTestContext(t.Context(), client, nil)
	if err := ctx.SendRichMessage(gogram.InputRichMessage{Html: "<b>hello</b>"}); err != nil {
		t.Fatalf("SendRichMessage: %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	rm, ok := body["rich_message"].(map[string]any)
	if !ok {
		t.Fatalf("rich_message field not found in body: %v", body)
	}
	if rm["html"] != "<b>hello</b>" {
		t.Errorf("html: got %v, want <b>hello</b>", rm["html"])
	}
}

// ---------------------------------------------------------------------------
// helpers for multipart assertions
// ---------------------------------------------------------------------------

// multipartFields parses a multipart request body and returns a map from
// field name → field value (as a string). File parts (those with a filename)
// are returned under their Content-Disposition name as well.
func multipartFields(t *testing.T, req *http.Request) (fields map[string]string, hasFile bool) {
	t.Helper()
	_, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parse Content-Type: %v", err)
	}
	boundary, ok := params["boundary"]
	if !ok {
		t.Fatal("no boundary in Content-Type")
	}

	fields = make(map[string]string)
	mr := multipart.NewReader(req.Body, boundary)
	for {
		part, err := mr.NextPart()
		if err != nil {
			break
		}
		data, _ := io.ReadAll(part)
		if part.FileName() != "" {
			hasFile = true
		}
		if name := part.FormName(); name != "" {
			fields[name] = string(data)
		}
	}
	return fields, hasFile
}

// ---------------------------------------------------------------------------
// SetMyProfilePhoto — now multipart (file upload).
// ---------------------------------------------------------------------------

// TestSetMyProfilePhoto_Upload verifies that SetMyProfilePhoto uses
// multipart/form-data and uploads the file part when InputFile.File is set.
func TestSetMyProfilePhoto_Upload(t *testing.T) {
	t.Parallel()

	var capturedFields map[string]string
	var sawFile bool

	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if !strings.HasSuffix(req.URL.Path, "/setMyProfilePhoto") {
			t.Errorf("unexpected path: %s", req.URL.Path)
		}
		ct := req.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "multipart/form-data") {
			t.Errorf("Content-Type: got %q, expected multipart/form-data prefix", ct)
		}
		capturedFields, sawFile = multipartFields(t, req)

		resp := gogram.Response{OK: true, Result: json.RawMessage(`true`)}
		return jsonHTTPResponse(t, http.StatusOK, &resp), nil
	})

	client, err := gogram.NewClient(testToken,
		gogram.WithHost("example.invalid"),
		gogram.WithHTTPClient(&http.Client{Transport: transport}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	fileData := bytes.NewReader([]byte("fake jpeg data"))
	if _, err = client.SetMyProfilePhoto(t.Context(), &gogram.SetMyProfilePhotoParams{
		Photo: gogram.InputProfilePhoto{
			InputProfilePhotoStatic: &gogram.InputProfilePhotoStatic{
				Photo: gogram.InputFile{File: fileData, FileName: "photo.jpg"},
			},
		},
	}); err != nil {
		t.Fatalf("SetMyProfilePhoto: %v", err)
	}

	if !sawFile {
		t.Error("expected a file part in the multipart body")
	}

	photoJSON, ok := capturedFields["photo"]
	if !ok {
		t.Fatalf("photo form field not found; fields: %v", capturedFields)
	}
	var photoObj map[string]any
	if err := json.Unmarshal([]byte(photoJSON), &photoObj); err != nil {
		t.Fatalf("unmarshal photo field: %v", err)
	}
	if photoObj["type"] != "static" {
		t.Errorf("photo.type: got %v, want static", photoObj["type"])
	}
	// The photo value should reference the uploaded file via attach://
	if s, _ := photoObj["photo"].(string); !strings.HasPrefix(s, "attach://") {
		t.Errorf("photo.photo: got %q, expected attach:// prefix", s)
	}
}

// TestSetMyProfilePhoto_FileID verifies that a file_id reference is correctly
// sent (as a multipart field, no file binary part).
func TestSetMyProfilePhoto_FileID(t *testing.T) {
	t.Parallel()

	const wantFileID = "AgACAgIAAx0CABCDEF123"

	var capturedFields map[string]string
	var sawFile bool

	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if !strings.HasSuffix(req.URL.Path, "/setMyProfilePhoto") {
			t.Errorf("unexpected path: %s", req.URL.Path)
		}
		ct := req.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "multipart/form-data") {
			t.Errorf("Content-Type: got %q, expected multipart/form-data prefix", ct)
		}
		capturedFields, sawFile = multipartFields(t, req)

		resp := gogram.Response{OK: true, Result: json.RawMessage(`true`)}
		return jsonHTTPResponse(t, http.StatusOK, &resp), nil
	})

	client, err := gogram.NewClient(testToken,
		gogram.WithHost("example.invalid"),
		gogram.WithHTTPClient(&http.Client{Transport: transport}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if _, err = client.SetMyProfilePhoto(t.Context(), &gogram.SetMyProfilePhotoParams{
		Photo: gogram.InputProfilePhoto{
			InputProfilePhotoStatic: &gogram.InputProfilePhotoStatic{
				Photo: gogram.InputFile{FileID: wantFileID},
			},
		},
	}); err != nil {
		t.Fatalf("SetMyProfilePhoto: %v", err)
	}

	if sawFile {
		t.Error("expected no binary file part for a file_id upload")
	}

	photoJSON, ok := capturedFields["photo"]
	if !ok {
		t.Fatalf("photo form field not found; fields: %v", capturedFields)
	}
	var photoObj map[string]any
	if err := json.Unmarshal([]byte(photoJSON), &photoObj); err != nil {
		t.Fatalf("unmarshal photo field: %v", err)
	}
	if photoObj["photo"] != wantFileID {
		t.Errorf("photo.photo: got %v, want %q", photoObj["photo"], wantFileID)
	}
}

// ---------------------------------------------------------------------------
// SetBusinessAccountProfilePhoto — multipart.
// ---------------------------------------------------------------------------

// TestSetBusinessAccountProfilePhoto_Upload verifies multipart encoding and
// that the business_connection_id field is present.
func TestSetBusinessAccountProfilePhoto_Upload(t *testing.T) {
	t.Parallel()

	var capturedFields map[string]string
	var sawFile bool

	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if !strings.HasSuffix(req.URL.Path, "/setBusinessAccountProfilePhoto") {
			t.Errorf("unexpected path: %s", req.URL.Path)
		}
		ct := req.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "multipart/form-data") {
			t.Errorf("Content-Type: got %q, expected multipart/form-data prefix", ct)
		}
		capturedFields, sawFile = multipartFields(t, req)

		resp := gogram.Response{OK: true, Result: json.RawMessage(`true`)}
		return jsonHTTPResponse(t, http.StatusOK, &resp), nil
	})

	client, err := gogram.NewClient(testToken,
		gogram.WithHost("example.invalid"),
		gogram.WithHTTPClient(&http.Client{Transport: transport}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	fileData := bytes.NewReader([]byte("fake jpeg data"))
	if _, err = client.SetBusinessAccountProfilePhoto(t.Context(), &gogram.SetBusinessAccountProfilePhotoParams{
		BusinessConnectionID: "biz_conn_42",
		Photo: gogram.InputProfilePhoto{
			InputProfilePhotoStatic: &gogram.InputProfilePhotoStatic{
				Photo: gogram.InputFile{File: fileData, FileName: "photo.jpg"},
			},
		},
	}); err != nil {
		t.Fatalf("SetBusinessAccountProfilePhoto: %v", err)
	}

	if !sawFile {
		t.Error("expected a file part in the multipart body")
	}
	if capturedFields["business_connection_id"] != "biz_conn_42" {
		t.Errorf("business_connection_id: got %q, want biz_conn_42", capturedFields["business_connection_id"])
	}

	photoJSON, ok := capturedFields["photo"]
	if !ok {
		t.Fatalf("photo form field not found; fields: %v", capturedFields)
	}
	var photoObj map[string]any
	if err := json.Unmarshal([]byte(photoJSON), &photoObj); err != nil {
		t.Fatalf("unmarshal photo field: %v", err)
	}
	if photoObj["type"] != "static" {
		t.Errorf("photo.type: got %v, want static", photoObj["type"])
	}
}

// ---------------------------------------------------------------------------
// PostStory — multipart.
// ---------------------------------------------------------------------------

// TestPostStory_Upload verifies that PostStory uses multipart/form-data,
// uploads the story photo, and encodes active_period as a form field.
func TestPostStory_Upload(t *testing.T) {
	t.Parallel()

	var capturedFields map[string]string
	var sawFile bool

	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if !strings.HasSuffix(req.URL.Path, "/postStory") {
			t.Errorf("unexpected path: %s", req.URL.Path)
		}
		ct := req.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "multipart/form-data") {
			t.Errorf("Content-Type: got %q, expected multipart/form-data prefix", ct)
		}
		capturedFields, sawFile = multipartFields(t, req)

		story := &gogram.Story{ID: 1}
		resp := gogram.Response{OK: true, Result: json.RawMessage(mustMarshal(t, story))}
		return jsonHTTPResponse(t, http.StatusOK, &resp), nil
	})

	client, err := gogram.NewClient(testToken,
		gogram.WithHost("example.invalid"),
		gogram.WithHTTPClient(&http.Client{Transport: transport}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	fileData := bytes.NewReader([]byte("fake jpeg data"))
	if _, err = client.PostStory(t.Context(), &gogram.PostStoryParams{
		BusinessConnectionID: "biz_conn_story",
		Content: gogram.InputStoryContent{
			InputStoryContentPhoto: &gogram.InputStoryContentPhoto{
				Photo: gogram.InputFile{File: fileData, FileName: "story.jpg"},
			},
		},
		ActivePeriod: 86400,
	}); err != nil {
		t.Fatalf("PostStory: %v", err)
	}

	if !sawFile {
		t.Error("expected a file part in the multipart body")
	}
	if capturedFields["active_period"] != "86400" {
		t.Errorf("active_period: got %q, want 86400", capturedFields["active_period"])
	}

	contentJSON, ok := capturedFields["content"]
	if !ok {
		t.Fatalf("content form field not found; fields: %v", capturedFields)
	}
	var contentObj map[string]any
	if err := json.Unmarshal([]byte(contentJSON), &contentObj); err != nil {
		t.Fatalf("unmarshal content field: %v", err)
	}
	if contentObj["type"] != "photo" {
		t.Errorf("content.type: got %v, want photo", contentObj["type"])
	}
}

// ---------------------------------------------------------------------------
// EditStory — multipart.
// ---------------------------------------------------------------------------

// TestEditStory_Upload verifies that EditStory uses multipart/form-data,
// uploads the story photo, and encodes story_id as a form field.
func TestEditStory_Upload(t *testing.T) {
	t.Parallel()

	var capturedFields map[string]string
	var sawFile bool

	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if !strings.HasSuffix(req.URL.Path, "/editStory") {
			t.Errorf("unexpected path: %s", req.URL.Path)
		}
		ct := req.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "multipart/form-data") {
			t.Errorf("Content-Type: got %q, expected multipart/form-data prefix", ct)
		}
		capturedFields, sawFile = multipartFields(t, req)

		story := &gogram.Story{ID: 42}
		resp := gogram.Response{OK: true, Result: json.RawMessage(mustMarshal(t, story))}
		return jsonHTTPResponse(t, http.StatusOK, &resp), nil
	})

	client, err := gogram.NewClient(testToken,
		gogram.WithHost("example.invalid"),
		gogram.WithHTTPClient(&http.Client{Transport: transport}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	fileData := bytes.NewReader([]byte("fake jpeg data"))
	if _, err = client.EditStory(t.Context(), &gogram.EditStoryParams{
		BusinessConnectionID: "biz_conn_edit",
		StoryID:              42,
		Content: gogram.InputStoryContent{
			InputStoryContentPhoto: &gogram.InputStoryContentPhoto{
				Photo: gogram.InputFile{File: fileData, FileName: "story.jpg"},
			},
		},
	}); err != nil {
		t.Fatalf("EditStory: %v", err)
	}

	if !sawFile {
		t.Error("expected a file part in the multipart body")
	}
	if capturedFields["story_id"] != "42" {
		t.Errorf("story_id: got %q, want 42", capturedFields["story_id"])
	}

	contentJSON, ok := capturedFields["content"]
	if !ok {
		t.Fatalf("content form field not found; fields: %v", capturedFields)
	}
	var contentObj map[string]any
	if err := json.Unmarshal([]byte(contentJSON), &contentObj); err != nil {
		t.Fatalf("unmarshal content field: %v", err)
	}
	if contentObj["type"] != "photo" {
		t.Errorf("content.type: got %v, want photo", contentObj["type"])
	}
}
