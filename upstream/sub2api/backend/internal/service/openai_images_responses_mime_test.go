package service

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"mime"
	"mime/multipart"
	"net/textproto"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIResponsesImageUploadToDataURLNormalizesFallbackMIME(t *testing.T) {
	pngBytes, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	require.NoError(t, err)

	tests := []struct {
		name        string
		contentType string
		data        []byte
		wantPrefix  string
		wantErr     bool
	}{
		{name: "empty MIME detects PNG", data: pngBytes, wantPrefix: "data:image/png;base64,"},
		{name: "octet stream detects PNG", contentType: "application/octet-stream", data: pngBytes, wantPrefix: "data:image/png;base64,"},
		{name: "normalized octet stream detects PNG", contentType: " Application/Octet-Stream; charset=binary ", data: pngBytes, wantPrefix: "data:image/png;base64,"},
		{name: "empty MIME rejects text", data: []byte("plain text"), wantErr: true},
		{name: "octet stream rejects text", contentType: "application/octet-stream", data: []byte("plain text"), wantErr: true},
		{name: "explicit image MIME is preserved", contentType: " IMAGE/PNG; x-existing=1 ", data: []byte("not-sniffable"), wantPrefix: "data:IMAGE/PNG; x-existing=1;base64,"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataURL, err := openAIResponsesImageUploadToDataURL(OpenAIImagesUpload{
				FileName:    "input.png",
				ContentType: tt.contentType,
				Data:        tt.data,
			})
			if tt.wantErr {
				require.Error(t, err)
				require.Empty(t, dataURL)
				return
			}
			require.NoError(t, err)
			require.True(t, strings.HasPrefix(dataURL, tt.wantPrefix), "data URL %q does not have prefix %q", dataURL, tt.wantPrefix)
		})
	}
}

func TestBuildOpenAIImagesResponsesRequestRejectsNonImageFallbackMIME(t *testing.T) {
	nonImageUpload := OpenAIImagesUpload{
		FileName:    "not-an-image.bin",
		ContentType: "application/octet-stream",
		Data:        []byte("plain text"),
	}

	tests := []struct {
		name   string
		parsed *OpenAIImagesRequest
	}{
		{
			name: "input upload",
			parsed: &OpenAIImagesRequest{
				Endpoint: openAIImagesEditsEndpoint,
				Prompt:   "edit the image",
				Uploads:  []OpenAIImagesUpload{nonImageUpload},
			},
		},
		{
			name: "mask upload",
			parsed: &OpenAIImagesRequest{
				Endpoint:       openAIImagesEditsEndpoint,
				Prompt:         "edit the image",
				InputImageURLs: []string{"https://example.com/input.png"},
				MaskUpload:     &nonImageUpload,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := buildOpenAIImagesResponsesRequest(tt.parsed, "gpt-image-2")
			require.EqualError(t, err, `upload "not-an-image.bin" is not an image`)
			require.Nil(t, body)
		})
	}
}

func TestOpenAIImageUploadToDataURLSniffsOctetStream(t *testing.T) {
	pngBytes, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	require.NoError(t, err)
	jpegBytes := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}

	pngURL, err := openAIImageUploadToDataURL(OpenAIImagesUpload{
		FileName:    "input.png",
		ContentType: "application/octet-stream",
		Data:        pngBytes,
	})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(pngURL, "data:image/png;base64,"))

	jpegURL, err := openAIImageUploadToDataURL(OpenAIImagesUpload{
		FileName:    "input.jpg",
		ContentType: "application/octet-stream",
		Data:        jpegBytes,
	})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(jpegURL, "data:image/jpeg;base64,"))

	_, err = openAIImageUploadToDataURL(OpenAIImagesUpload{
		FileName:    "notes.txt",
		ContentType: "application/octet-stream",
		Data:        []byte("plain text"),
	})
	require.EqualError(t, err, `upload "notes.txt" is not an image`)
}

func TestCodexDirectImagesEditSniffsOctetStreamDataURL(t *testing.T) {
	pngBytes, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	require.NoError(t, err)
	octetURL := "data:application/octet-stream;base64," + base64.StdEncoding.EncodeToString(pngBytes)

	body, target, err := buildOpenAIImagesOAuthPayload(&OpenAIImagesRequest{
		Endpoint:       openAIImagesEditsEndpoint,
		Model:          "gpt-image-2",
		Prompt:         "edit",
		InputImageURLs: []string{octetURL},
		MaskImageURL:   octetURL,
	}, "gpt-image-2")
	require.NoError(t, err)
	require.Equal(t, "https://chatgpt.com/backend-api/codex/images/edits", target)
	require.True(t, strings.HasPrefix(gjson.GetBytes(body, "images.0.image_url").String(), "data:image/png;base64,"))
	require.True(t, strings.HasPrefix(gjson.GetBytes(body, "mask.image_url").String(), "data:image/png;base64,"))

	uploadBody, _, err := buildOpenAIImagesOAuthPayload(&OpenAIImagesRequest{
		Endpoint: openAIImagesEditsEndpoint,
		Model:    "gpt-image-2",
		Prompt:   "edit",
		Uploads: []OpenAIImagesUpload{{
			FileName:    "person.jpg",
			ContentType: "application/octet-stream",
			Data:        []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00},
		}},
	}, "gpt-image-2")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(gjson.GetBytes(uploadBody, "images.0.image_url").String(), "data:image/jpeg;base64,"))
}

func TestRewriteOpenAIImagesMultipartSniffsOctetStream(t *testing.T) {
	jpegBytes := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gpt-image-2"))
	require.NoError(t, writer.WriteField("prompt", "edit"))
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="image"; filename="photo.jpg"`)
	header.Set("Content-Type", "application/octet-stream")
	part, err := writer.CreatePart(header)
	require.NoError(t, err)
	_, err = part.Write(jpegBytes)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	rewritten, contentType, err := rewriteOpenAIImagesModel(body.Bytes(), writer.FormDataContentType(), "gpt-image-2")
	require.NoError(t, err)
	_, params, err := mime.ParseMediaType(contentType)
	require.NoError(t, err)
	reader := multipart.NewReader(bytes.NewReader(rewritten), params["boundary"])
	var sawImage bool
	for {
		next, err := reader.NextPart()
		if err != nil {
			break
		}
		if next.FormName() == "image" {
			sawImage = true
			require.Equal(t, "image/jpeg", next.Header.Get("Content-Type"))
		}
		_ = next.Close()
	}
	require.True(t, sawImage)
}

func TestRewriteOpenAIImagesJSONSniffsOctetStreamDataURL(t *testing.T) {
	pngBytes, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	require.NoError(t, err)
	octetURL := "data:application/octet-stream;base64," + base64.StdEncoding.EncodeToString(pngBytes)
	raw := []byte(fmt.Sprintf(`{"model":"client-model","prompt":"edit","images":[{"image_url":%q}],"mask":{"image_url":%q}}`, octetURL, octetURL))

	rewritten, contentType, err := rewriteOpenAIImagesModel(raw, "application/json", "gpt-image-2")
	require.NoError(t, err)
	require.Equal(t, "application/json", contentType)
	require.Equal(t, "gpt-image-2", gjson.GetBytes(rewritten, "model").String())
	require.True(t, strings.HasPrefix(gjson.GetBytes(rewritten, "images.0.image_url").String(), "data:image/png;base64,"))
	require.True(t, strings.HasPrefix(gjson.GetBytes(rewritten, "mask.image_url").String(), "data:image/png;base64,"))
}
