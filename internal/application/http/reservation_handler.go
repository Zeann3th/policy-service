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
	routes.DELETE("/v1/reservations/:requestId", handler.release)
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

// release godoc
//
//	@Summary		Release a reservation
//	@Description	Reverses a RESERVED reservation by its original requestId: each RESERVE ledger entry is recorded again as a RELEASE entry with the opposite delta and subtracted from the same usage window. Only RESERVED can be released; anything else is 409.
//	@Tags			Reservations
//	@Produce		json
//	@Param			requestId		path		string	true	"Original requestId"
//	@Success		200				{object}	reservation.ReleaseResponse
//	@Failure		400,404,409,500	{object}	core.AppError
//	@Router			/v1/reservations/{requestId} [delete]
func (h *reservationHandler) release(c *gin.Context) {
	response, err := h.service.Release(c.Request.Context(), c.Param("requestId"))
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}
