package controllers

import "vento-app/vento"

// Health handles GET /api/health: a liveness check for load balancers,
// uptime monitors, and container orchestrators. Add real readiness checks
// here (e.g. c.DB().Exec("SELECT 1")) if a deployment needs more than a
// process-is-alive signal.
//
// c.OK writes JSON with a 200 status - one of Vento's response shorthands
// (c.OK/c.Created/c.NoContent, c.BadRequest/c.NotFound/...) that keep
// controllers from having to import net/http just to name a status code.
func Health(c *vento.Context) {
	c.OK(vento.H{"status": "ok"})
}
