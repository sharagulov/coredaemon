package storage

import (
	"net/http"
	"os"
	"path"
	"strings"
)

func driveKind(name string, isDir bool) (kind, mime string) {
	if isDir {
		return "folder", "inode/directory"
	}
	ext := strings.ToLower(path.Ext(name))
	if hit, ok := driveExt[ext]; ok {
		return hit.kind, hit.mime
	}
	return "other", "application/octet-stream"
}

func sniffDriveFile(abs, name string, isDir bool) (kind, mime string) {
	kind, mime = driveKind(name, isDir)
	if isDir || kind != "other" {
		return kind, mime
	}
	f, err := os.Open(abs)
	if err != nil {
		return kind, mime
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	if n == 0 {
		return kind, mime
	}
	mime = http.DetectContentType(buf[:n])
	return kindFromMIME(mime), mime
}

func kindFromMIME(mime string) string {
	switch {
	case strings.HasPrefix(mime, "image/"):
		return "image"
	case strings.HasPrefix(mime, "video/"):
		return "video"
	case strings.HasPrefix(mime, "audio/"):
		return "audio"
	case mime == "application/pdf":
		return "pdf"
	case strings.HasPrefix(mime, "text/"):
		return "text"
	default:
		return "other"
	}
}

var driveExt = map[string]struct{ kind, mime string }{
	".jpg":  {kind: "image", mime: "image/jpeg"},
	".jpeg": {kind: "image", mime: "image/jpeg"},
	".png":  {kind: "image", mime: "image/png"},
	".gif":  {kind: "image", mime: "image/gif"},
	".webp": {kind: "image", mime: "image/webp"},
	".svg":  {kind: "image", mime: "image/svg+xml"},
	".mp4":  {kind: "video", mime: "video/mp4"},
	".webm": {kind: "video", mime: "video/webm"},
	".mov":  {kind: "video", mime: "video/quicktime"},
	".mkv":  {kind: "video", mime: "video/x-matroska"},
	".mp3":  {kind: "audio", mime: "audio/mpeg"},
	".wav":  {kind: "audio", mime: "audio/wav"},
	".flac": {kind: "audio", mime: "audio/flac"},
	".ogg":  {kind: "audio", mime: "audio/ogg"},
	".pdf":  {kind: "pdf", mime: "application/pdf"},
	".zip":  {kind: "archive", mime: "application/zip"},
	".rar":  {kind: "archive", mime: "application/vnd.rar"},
	".7z":   {kind: "archive", mime: "application/x-7z-compressed"},
	".tar":  {kind: "archive", mime: "application/x-tar"},
	".gz":   {kind: "archive", mime: "application/gzip"},
	".txt":  {kind: "text", mime: "text/plain"},
	".md":   {kind: "text", mime: "text/markdown"},
	".csv":  {kind: "text", mime: "text/csv"},
	".json": {kind: "text", mime: "application/json"},
	".doc":  {kind: "other", mime: "application/msword"},
	".docx": {kind: "other", mime: "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
}
