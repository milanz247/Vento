package controllers

import (
	"net/http"

	"vento-app/vento"
)

// Health handles GET /api/health: a liveness check for load balancers,
// uptime monitors, and container orchestrators. Add real readiness checks
// here (e.g. c.DB().Exec("SELECT 1")) if a deployment needs more than a
// process-is-alive signal.
func Health(c *vento.Context) {
	c.JSON(http.StatusOK, vento.H{"status": "ok"})
}
