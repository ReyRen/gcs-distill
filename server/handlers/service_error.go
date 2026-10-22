package handlers

import (
	"errors"
	"net/http"

	"github.com/ReyRen/gcs-distill/service"
	"github.com/gin-gonic/gin"
)

func writeServiceError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	var validation *service.ValidationError
	if errors.As(err, &validation) {
		status = http.StatusBadRequest
	}
	c.JSON(status, gin.H{"code": status, "message": err.Error()})
}
