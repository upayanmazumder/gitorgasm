package administration

import (
	"net/http"
	"strconv"
	"time"

	"github.com/upayanmazumder/gitorgasm/core/internal/github/models"
	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/middleware/admission"
	"github.com/upayanmazumder/gitorgasm/core/internal/httpapi/response"
)

// GitHub allows unauthenticated search/users about 10 requests per minute per source address,
// which every tenant of this API shares. The per-actor and per-installation allowances keep one
// owner from spending all of it; the cache removes repeated identical lookups.
const (
	searchActorLimit        = 4
	searchInstallationLimit = 6
	searchWindow            = time.Minute
	searchCacheTTL          = 45 * time.Second
	searchCacheEntries      = 512
)

type searchResult struct {
	users      []models.DirectoryUser
	next       bool
	incomplete bool
}

// SearchGuard rate-limits and caches public user search for one API process.
type SearchGuard struct {
	cache        *admission.TTLCache[searchResult]
	actors       *admission.WindowLimiter
	installation *admission.WindowLimiter
}

func NewSearchGuard(now func() time.Time) *SearchGuard {
	return &SearchGuard{
		cache:        admission.NewTTLCache[searchResult](searchCacheTTL, searchCacheEntries, now),
		actors:       admission.NewWindowLimiter(searchActorLimit, searchWindow, now),
		installation: admission.NewWindowLimiter(searchInstallationLimit, searchWindow, now),
	}
}

// allow charges one upstream search and writes the 429 response when a limit is spent.
func (g *SearchGuard) allow(w http.ResponseWriter, r *http.Request, actorID, installationID int64) bool {
	actor, install := "actor:"+strconv.FormatInt(actorID, 10), "installation:"+strconv.FormatInt(installationID, 10)
	ok, wait := g.actors.Allow(actor)
	if ok {
		var iwait time.Duration
		if ok, iwait = g.installation.Allow(install); ok {
			return true
		}
		g.actors.Refund(actor)
		wait = iwait
	}
	seconds := max(1, int((wait+time.Second-1)/time.Second))
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	response.Error(w, r, http.StatusTooManyRequests, "search_rate_limited", "too many user searches; retry shortly")
	return false
}
