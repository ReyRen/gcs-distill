package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func requireUIDQuery(c *gin.Context) (int, bool) {
	return requireUIDValue(c, c.Query("uid"))
}

func requireUIDForm(c *gin.Context) (int, bool) {
	return requireUIDValue(c, c.PostForm("uid"))
}

func requireUIDValue(c *gin.Context, raw string) (int, bool) {
	uid, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || uid <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": "uid 必须为大于0的整数",
		})
		return 0, false
	}
	return uid, true
}
