// Package api 提供 HTTP API 处理器。
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/adortb/adortb-retail-media/internal/auction"
	"github.com/adortb/adortb-retail-media/internal/campaign"
	"github.com/adortb/adortb-retail-media/internal/catalog"
	"github.com/adortb/adortb-retail-media/internal/reporting"
)

// Handler 汇聚所有子服务的 HTTP 处理器。
type Handler struct {
	products      catalog.ProductStore
	categories    catalog.CategoryStore
	inventory     catalog.InventoryStore
	spStore       campaign.SPStore
	sbStore       campaign.SBStore
	searchAuction *auction.SearchAuction
	catAuction    *auction.CategoryAuction
	reporter      reporting.Reporter
}

// NewHandler 创建 API 处理器。
func NewHandler(
	products catalog.ProductStore,
	categories catalog.CategoryStore,
	inventory catalog.InventoryStore,
	spStore campaign.SPStore,
	sbStore campaign.SBStore,
	searchAuction *auction.SearchAuction,
	catAuction *auction.CategoryAuction,
	reporter reporting.Reporter,
) *Handler {
	return &Handler{
		products:      products,
		categories:    categories,
		inventory:     inventory,
		spStore:       spStore,
		sbStore:       sbStore,
		searchAuction: searchAuction,
		catAuction:    catAuction,
		reporter:      reporter,
	}
}

// RegisterRoutes 注册所有路由到 mux。
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Catalog
	mux.HandleFunc("GET /v1/products", h.listProducts)
	mux.HandleFunc("POST /v1/products", h.createProduct)
	mux.HandleFunc("GET /v1/products/{sku}", h.getProduct)
	mux.HandleFunc("PUT /v1/products/{sku}", h.updateProduct)
	mux.HandleFunc("DELETE /v1/products/{sku}", h.deleteProduct)
	mux.HandleFunc("POST /v1/products/import", h.bulkImportProducts)

	// SP Campaigns
	mux.HandleFunc("GET /v1/campaigns/sp", h.listSPCampaigns)
	mux.HandleFunc("POST /v1/campaigns/sp", h.createSPCampaign)
	mux.HandleFunc("GET /v1/campaigns/sp/{id}", h.getSPCampaign)
	mux.HandleFunc("PUT /v1/campaigns/sp/{id}", h.updateSPCampaign)
	mux.HandleFunc("DELETE /v1/campaigns/sp/{id}", h.deleteSPCampaign)
	mux.HandleFunc("GET /v1/campaigns/sp/{id}/performance", h.getCampaignPerformance)

	// SB Campaigns
	mux.HandleFunc("GET /v1/campaigns/sb", h.listSBCampaigns)
	mux.HandleFunc("POST /v1/campaigns/sb", h.createSBCampaign)

	// Ad Groups & Keywords
	mux.HandleFunc("POST /v1/ad-groups", h.createAdGroup)
	mux.HandleFunc("POST /v1/keywords", h.createKeyword)
	mux.HandleFunc("POST /v1/product-ads", h.createProductAd)

	// Auction
	mux.HandleFunc("POST /v1/ads/search", h.searchAds)
	mux.HandleFunc("POST /v1/ads/category", h.categoryAds)

	// Events
	mux.HandleFunc("POST /v1/events/click", h.recordClick)
	mux.HandleFunc("POST /v1/events/purchase", h.recordPurchase)
}

// --- Catalog Handlers ---

func (h *Handler) listProducts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	advertiserID, _ := strconv.ParseInt(q.Get("advertiser_id"), 10, 64)
	products, err := h.products.List(r.Context(), catalog.ProductFilter{
		AdvertiserID: advertiserID,
		CategoryID:   q.Get("category_id"),
		Status:       q.Get("status"),
		Limit:        50,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, products)
}

func (h *Handler) createProduct(w http.ResponseWriter, r *http.Request) {
	var p catalog.Product
	if err := decodeJSON(r, &p); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := h.products.Upsert(r.Context(), &p); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (h *Handler) getProduct(w http.ResponseWriter, r *http.Request) {
	sku := r.PathValue("sku")
	p, err := h.products.Get(r.Context(), sku)
	if err != nil {
		if errors.Is(err, catalog.ErrProductNotFound) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *Handler) updateProduct(w http.ResponseWriter, r *http.Request) {
	sku := r.PathValue("sku")
	var p catalog.Product
	if err := decodeJSON(r, &p); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	p.SKU = sku
	if err := h.products.Upsert(r.Context(), &p); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *Handler) deleteProduct(w http.ResponseWriter, r *http.Request) {
	sku := r.PathValue("sku")
	if err := h.products.Delete(r.Context(), sku); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) bulkImportProducts(w http.ResponseWriter, r *http.Request) {
	var products []*catalog.Product
	if err := decodeJSON(r, &products); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	count, err := h.products.BulkUpsert(r.Context(), products)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"imported": count})
}

// --- SP Campaign Handlers ---

func (h *Handler) listSPCampaigns(w http.ResponseWriter, r *http.Request) {
	advertiserID, err := parseAdvertiserID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	campaigns, err := h.spStore.ListCampaigns(r.Context(), advertiserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, campaigns)
}

func (h *Handler) createSPCampaign(w http.ResponseWriter, r *http.Request) {
	var c campaign.SPCampaign
	if err := decodeJSON(r, &c); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	id, err := h.spStore.CreateCampaign(r.Context(), &c)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	c.ID = id
	writeJSON(w, http.StatusCreated, c)
}

func (h *Handler) getSPCampaign(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	c, err := h.spStore.GetCampaign(r.Context(), id)
	if err != nil {
		if errors.Is(err, campaign.ErrCampaignNotFound) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (h *Handler) updateSPCampaign(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var c campaign.SPCampaign
	if err := decodeJSON(r, &c); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	c.ID = id
	if err := h.spStore.UpdateCampaign(r.Context(), &c); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (h *Handler) deleteSPCampaign(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := h.spStore.DeleteCampaign(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) getCampaignPerformance(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	q := r.URL.Query()
	from, _ := time.Parse("2006-01-02", q.Get("from"))
	to, _ := time.Parse("2006-01-02", q.Get("to"))
	if to.IsZero() {
		to = time.Now()
	}
	if from.IsZero() {
		from = to.AddDate(0, 0, -30)
	}
	metrics, err := h.reporter.QueryMetrics(r.Context(), reporting.QueryParams{
		CampaignID: id,
		From:       from,
		To:         to,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, metrics)
}

// --- SB Campaign Handlers ---

func (h *Handler) listSBCampaigns(w http.ResponseWriter, r *http.Request) {
	advertiserID, err := parseAdvertiserID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	campaigns, err := h.sbStore.List(r.Context(), advertiserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, campaigns)
}

func (h *Handler) createSBCampaign(w http.ResponseWriter, r *http.Request) {
	var c campaign.SBCampaign
	if err := decodeJSON(r, &c); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	id, err := h.sbStore.Create(r.Context(), &c)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	c.ID = id
	writeJSON(w, http.StatusCreated, c)
}

// --- Ad Groups / Keywords / ProductAds ---

func (h *Handler) createAdGroup(w http.ResponseWriter, r *http.Request) {
	var g campaign.SPAdGroup
	if err := decodeJSON(r, &g); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	id, err := h.spStore.CreateAdGroup(r.Context(), &g)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	g.ID = id
	writeJSON(w, http.StatusCreated, g)
}

func (h *Handler) createKeyword(w http.ResponseWriter, r *http.Request) {
	var kw campaign.SPKeyword
	if err := decodeJSON(r, &kw); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	id, err := h.spStore.CreateKeyword(r.Context(), &kw)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	kw.ID = id
	writeJSON(w, http.StatusCreated, kw)
}

func (h *Handler) createProductAd(w http.ResponseWriter, r *http.Request) {
	var ad campaign.SPProductAd
	if err := decodeJSON(r, &ad); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	id, err := h.spStore.CreateProductAd(r.Context(), &ad)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	ad.ID = id
	writeJSON(w, http.StatusCreated, ad)
}

// --- Auction Handlers ---

func (h *Handler) searchAds(w http.ResponseWriter, r *http.Request) {
	var req auction.SearchRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	resp, err := h.searchAuction.Run(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) categoryAds(w http.ResponseWriter, r *http.Request) {
	var req auction.CategoryRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	ads, err := h.catAuction.Run(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ads": ads})
}

// --- Event Handlers ---

func (h *Handler) recordClick(w http.ResponseWriter, r *http.Request) {
	var row reporting.PerformanceRow
	if err := decodeJSON(r, &row); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if row.Date.IsZero() {
		row.Date = time.Now().Truncate(24 * time.Hour)
	}
	if err := h.reporter.RecordClick(r.Context(), row); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) recordPurchase(w http.ResponseWriter, r *http.Request) {
	var row reporting.PerformanceRow
	if err := decodeJSON(r, &row); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if row.Date.IsZero() {
		row.Date = time.Now().Truncate(24 * time.Hour)
	}
	if err := h.reporter.RecordPurchase(r.Context(), row); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func decodeJSON(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("invalid request body: %w", err)
	}
	return nil
}

func parseID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

func parseAdvertiserID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.URL.Query().Get("advertiser_id"), 10, 64)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("advertiser_id required")
	}
	return id, nil
}
