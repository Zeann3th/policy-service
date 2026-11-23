package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zeann3th/policy-service/internal/core/reservation"
)

type reservationHandler struct {
	service reservation.Service
}

func RegisterReservationRoutes(routes *gin.RouterGroup, service reservation.Service) {
	handler := &reservationHandler{service: service}
	routes.POST("/v1/reservations", handler.reserve)
}

// reserve godoc
//
//	@Summary		Reserve a transaction
//	@Description	Evaluates the transaction against the sender's (OUT) and receiver's (IN) effective policies. When no limit is violated the usage is added and the status is RESERVED; otherwise nothing is added and the status is REJECTED. requestId is unique per month.
//	@Tags			Reservations
//	@Accept			json
//	@Produce		json
//	@Param			request			body		reservation.ReserveRequest	true	"Reservation"
//	@Success		200				{object}	reservation.ReserveResponse
//	@Failure		400,404,409,500	{object}	core.AppError
//	@Router			/v1/reservations [post]
func (h *reservationHandler) reserve(c *gin.Context) {
	var request reservation.ReserveRequest
	if !bindJSON(c, &request) {
		return
	}
	response, err := h.service.Reserve(c.Request.Context(), request)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}
