package server

import (
	"bytes"
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"github.com/teaelephant/TeaElephantMemory/common"
)

type imageReader interface {
	ReadTeaImage(ctx context.Context, imageID uuid.UUID) (*common.TeaImageContent, error)
}

func (s *Server) serveTeaImage(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil || s.images == nil {
		http.NotFound(w, r)
		return
	}

	content, err := s.images.ReadTeaImage(r.Context(), id)
	if errors.Is(err, common.ErrTeaImageNotFound) {
		http.NotFound(w, r)
		return
	}

	if err != nil {
		logrus.WithError(err).Error("read tea image")
		http.Error(w, "Image unavailable", http.StatusInternalServerError)

		return
	}

	w.Header().Set(responseContentTypeHeader, content.ContentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
	w.Header().Set("ETag", `"`+id.String()+`"`)
	http.ServeContent(w, r, "", content.CreatedAt, bytes.NewReader(content.Data))
}
