// SSE stream for notifications: GET /api/notifications/stream.
//
// Server-tick design: one goroutine per connection polls the SAME
// fetchUnread query every 10s and pushes {data:[...]} only when the payload
// changed (count + latest event id). No write-path hooks, no pub/sub —
// simplest thing that beats client polling (one upstream query per connected
// tab either way, but updates arrive without the 30s client lag).
// ponytail: pub/sub push (event writes + per-user channels) when >20 users.
package handlers

import (
	"bufio"
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func NotificationStream(gdb *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := currentUser(c)

		c.Set(fiber.HeaderContentType, "text/event-stream")
		c.Set(fiber.HeaderCacheControl, "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("X-Accel-Buffering", "no") // proxies (Caddy/nginx): do not buffer

		ctx := c.Context()
		ctx.SetBodyStreamWriter(func(w *bufio.Writer) {
			send := func(payload string) bool {
				_, err := w.WriteString("data: " + payload + "\n\n")
				if err != nil {
					return false
				}
				return w.Flush() == nil
			}

			first := true
			lastSig := ""
			tick := time.NewTicker(10 * time.Second)
			defer tick.Stop()

			// immediate first push, then on every tick (only on change)
			push := func() bool {
				items, err := fetchUnread(gdb, u)
				if err != nil {
					return true // transient DB error: retry next tick
				}
				sig := ""
				if len(items) > 0 {
					sig = items[0].ID + ":" + string(rune(len(items)))
				}
				if sig == lastSig && !first {
					return true // unchanged
				}
				first = false
				lastSig = sig
				buf, err := json.Marshal(items)
				if err != nil {
					return true
				}
				return send(string(buf))
			}

			if !push() {
				return
			}
			for {
				select {
				case <-ctx.Done():
					return
				case <-tick.C:
					if !push() {
						return
					}
				}
			}
		})
		return nil
	}
}
