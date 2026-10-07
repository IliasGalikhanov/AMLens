package storage

import (
	"context"
	"errors"
	"finance.local/amlens/internal/application"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type Files struct {
	Root         string
	MaxFileBytes int64
}

func (f Files) Save(ctx context.Context, source application.UploadSource) (string, func(), error) {
	if err := os.MkdirAll(f.Root, 0700); err != nil {
		return "", nil, err
	}
	dir, err := os.MkdirTemp(f.Root, "upload-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	ok := false
	defer func() {
		if !ok {
			cleanup()
		}
	}()
	limit := f.MaxFileBytes
	if limit <= 0 {
		limit = 25 << 20
	}
	seen := map[string]bool{}
	for {
		if err := ctx.Err(); err != nil {
			return "", nil, err
		}
		upload, err := source.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", nil, uploadError(err)
		}
		if (upload.Name != "nodes" && upload.Name != "edges" && upload.Name != "transactions") || seen[upload.Name] || !strings.HasSuffix(strings.ToLower(upload.Filename), ".parquet") {
			return "", nil, application.Fail("INVALID_SCHEMA", "Нужны ровно три файловых поля nodes, edges, transactions с расширением .parquet")
		}
		seen[upload.Name] = true
		file, err := os.OpenFile(filepath.Join(dir, upload.Name+".parquet"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return "", nil, err
		}
		n, copyErr := io.Copy(file, io.LimitReader(upload.Reader, limit+1))
		closeErr := file.Close()
		if copyErr != nil {
			return "", nil, uploadError(copyErr)
		}
		if n > limit {
			return "", nil, application.Fail("FILE_TOO_LARGE", "Размер файла превышает 25 MiB")
		}
		if closeErr != nil {
			return "", nil, closeErr
		}
	}
	if len(seen) != 3 {
		return "", nil, application.Fail("INVALID_SCHEMA", "Загрузите nodes.parquet, edges.parquet и transactions.parquet")
	}
	ok = true
	return dir, cleanup, nil
}
func uploadError(err error) error {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return application.Fail("FILE_TOO_LARGE", "Размер запроса превышает 76 MiB")
	}
	return application.Fail("INVALID_SCHEMA", "Не удалось прочитать multipart-форму")
}
