package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zeann3th/policy-service/internal/core/dryrun"
)

type dryRunHandler struct {
	service dryrun.Service
}

func RegisterDryRunRoutes(routes *gin.RouterGroup, service dryrun.Service) {
	handler := &dryRunHandler{service: service}
	routes.POST("/v1/reservations/dry-run", handler.run)
}

// run godoc
//
//	@Summary		Dry-run a reservation
//	@Description	Evaluates the transaction exactly as POST /v1/reservations would, against the effective policies and the wallets' live usage (optionally overridden per policy id), and saves nothing: no reservation, usage or ledger row, no lock.
//	@Tags			Reservations
//	@Accept			json
//	@Produce		json
//	@Param			request		body		dryrun.Request	true	"Transaction"
//	@Success		200			{object}	dryrun.Response
//	@Failure		400,500		{object}	core.AppError
//	@Router			/v1/reservations/dry-run [post]
func (h *dryRunHandler) run(c *gin.Context) {
	var request dryrun.Request
	if !bindJSON(c, &request) {
		return
	}
	response, err := h.service.Run(c.Request.Context(), request)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}
