package shorturl

import (
	"fmt"
	"net/http"
	"path"
	"time"
	"urlshortener/internal/configmanager"
	"urlshortener/internal/shorturl/database"
	"urlshortener/internal/shorturl/service"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type ShortURLHandler struct {
	s        *service.ShortURL
	basePath string
	limiter  *rate.Limiter
}

type UploadRequest struct {
	URL      string    `json:"url"`
	ExpireAt time.Time `json:"expireAt"`
}

type ShortUrlResponse struct {
	ID       string `json:"id"`
	ShortURL string `json:"shortUrl"`
}

func NewShortURLHandler() (*ShortURLHandler, error) {
	config, err := configmanager.Get()
	if err != nil {
		return nil, err
	}

	db, err := newDatabase(config.Database)
	if err != nil {
		return nil, err
	}

	cache, err := newCache(config.Redis)
	if err != nil {
		return nil, err
	}

	return &ShortURLHandler{
		s:        service.NewShortURL(db, cache),
		basePath: fmt.Sprintf("%s:%d", config.HTTPServer.Domain, config.HTTPServer.Port),
		limiter:  rate.NewLimiter(config.Service.LimitRate, config.Service.BurstSize),
	}, nil
}

func (h *ShortURLHandler) UploadURL(ctx *gin.Context) {
	if err := h.limiter.Wait(ctx); err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	var request UploadRequest
	if err := ctx.BindJSON(&request); err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	urlID, err := h.s.Shorter(ctx, request.URL, request.ExpireAt)
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	response := ShortUrlResponse{
		ID:       urlID,
		ShortURL: path.Join(h.basePath, urlID),
	}
	ctx.JSON(http.StatusOK, response)
}

func (h *ShortURLHandler) DeleteURL(ctx *gin.Context) {
	if err := h.limiter.Wait(ctx); err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	urlID, ok := ctx.Params.Get("urlID")
	if !ok {
		ctx.Status(http.StatusBadRequest)
		return
	}

	if err := h.s.DeleteURL(ctx, urlID); err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.Status(http.StatusOK)
}

func (h *ShortURLHandler) RedirectURL(ctx *gin.Context) {
	if err := h.limiter.Wait(ctx); err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	urlID, ok := ctx.Params.Get("urlID")
	if !ok {
		ctx.Status(http.StatusBadRequest)
		return
	}

	url, err := h.s.GetURL(ctx, urlID)
	if err != nil {
		_, ok := err.(*database.DatabaseError)
		if ok {
			ctx.Status(http.StatusInternalServerError)
		} else {
			ctx.Status(http.StatusNotFound)
		}
		return
	}

	ctx.Redirect(http.StatusFound, url)
}
