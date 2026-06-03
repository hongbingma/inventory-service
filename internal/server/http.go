package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"inventory-service/internal/biz"

	khttp "github.com/go-kratos/kratos/v2/transport/http"
)

type HTTPConfig struct{ Addr string }

type Handler struct{ uc *biz.InventoryUsecase }

func NewHTTPServer(cfg HTTPConfig, uc *biz.InventoryUsecase) *khttp.Server {
	if cfg.Addr == "" {
		cfg.Addr = ":8000"
	}
	srv := khttp.NewServer(khttp.Address(cfg.Addr), khttp.Timeout(10*time.Second))
	h := &Handler{uc: uc}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", h.health)
	mux.HandleFunc("/v1/inventories", h.collection)
	mux.HandleFunc("/v1/inventories/", h.item)
	srv.HandlePrefix("/", mux)
	return srv
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) collection(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v1/inventories" {
		writeError(w, biz.ErrNotFound)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		SkuID int64 `json:"sku_id"`
		Total int64 `json:"total"`
	}
	if !decode(w, r, &req) {
		return
	}
	inv, err := h.uc.CreateOrReplace(r.Context(), req.SkuID, req.Total)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"inventory": inv})
}

func (h *Handler) item(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/inventories/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 1 && parts[0] != "" {
		skuID, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			writeError(w, biz.ErrInvalidArgument)
			return
		}
		switch r.Method {
		case http.MethodGet:
			inv, err := h.uc.Get(r.Context(), skuID)
			if err != nil {
				writeError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"inventory": inv})
		case http.MethodPut:
			var req struct {
				Total           int64 `json:"total"`
				ExpectedVersion int64 `json:"expected_version"`
			}
			if !decode(w, r, &req) {
				return
			}
			inv, err := h.uc.Edit(r.Context(), skuID, req.Total, req.ExpectedVersion)
			if err != nil {
				writeError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"inventory": inv})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}
	if len(parts) == 1 && parts[0] == "deduct" && r.Method == http.MethodPost {
		h.deduct(w, r)
		return
	}
	if len(parts) == 1 && parts[0] == "increase" && r.Method == http.MethodPost {
		h.release(w, r)
		return
	}
	if len(parts) == 1 && parts[0] == "release" && r.Method == http.MethodPost {
		h.release(w, r)
		return
	}
	if len(parts) == 1 && parts[0] == "confirm" && r.Method == http.MethodPost {
		h.confirm(w, r)
		return
	}
	writeError(w, biz.ErrNotFound)
}

func (h *Handler) deduct(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RequestID string `json:"request_id"`
		SkuID     int64  `json:"sku_id"`
		Quantity  int64  `json:"quantity"`
	}
	if !decode(w, r, &req) {
		return
	}
	inv, ded, err := h.uc.Deduct(r.Context(), req.RequestID, req.SkuID, req.Quantity)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"inventory": inv, "deduction": ded})
}

func (h *Handler) release(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RequestID          string `json:"request_id"`
		DeductionRequestID string `json:"deduction_request_id"`
	}
	if !decode(w, r, &req) {
		return
	}
	inv, ded, err := h.uc.Release(r.Context(), req.RequestID, req.DeductionRequestID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"inventory": inv, "deduction": ded})
}

func (h *Handler) confirm(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RequestID          string `json:"request_id"`
		DeductionRequestID string `json:"deduction_request_id"`
	}
	if !decode(w, r, &req) {
		return
	}
	inv, ded, err := h.uc.Confirm(r.Context(), req.RequestID, req.DeductionRequestID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"inventory": inv, "deduction": ded})
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, biz.ErrInvalidArgument)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	switch {
	case errors.Is(err, biz.ErrInvalidArgument):
		code = http.StatusBadRequest
	case errors.Is(err, biz.ErrNotFound):
		code = http.StatusNotFound
	case errors.Is(err, biz.ErrInsufficientStock), errors.Is(err, biz.ErrVersionConflict), errors.Is(err, biz.ErrInvalidState):
		code = http.StatusConflict
	case errors.Is(err, context.DeadlineExceeded):
		code = http.StatusGatewayTimeout
	}
	writeJSON(w, code, map[string]string{"error": err.Error()})
}
