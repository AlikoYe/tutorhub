// Package visibility is the single source of truth for which specialists are listed in the
// catalogue and how they rank. SQL fragments expect specialists aliased as "s"; use them,
// never re-type the rule.
package visibility

import (
	"regexp"
	"strings"

	"github.com/AlikoYe/tutorhub/backend/internal/lessonstatus"
)

// Values of specialists.status.
const (
	Draft   = "draft"
	Pending = "pending"
	Live    = "live"
	Hidden  = "hidden"
	Blocked = "blocked"
)

// Values of subscriptions.plan.
const (
	PlanFounding = "founding"
	PlanTrial    = "trial"
	PlanBasic    = "basic"
	PlanFeatured = "featured"
)

// ActiveSubscriptionSQL is true when specialist s has a subscription running now.
const ActiveSubscriptionSQL = `EXISTS (SELECT 1 FROM subscriptions sub
	WHERE sub.specialist_id = s.id AND sub.starts_at <= now() AND sub.ends_at > now())`

// ListedSQL is the visibility rule: live AND a subscription running now.
const ListedSQL = `(s.status = 'live' AND ` + ActiveSubscriptionSQL + `)`

// FeaturedSQL is true when s has a featured subscription running now.
const FeaturedSQL = `EXISTS (SELECT 1 FROM subscriptions sub
	WHERE sub.specialist_id = s.id AND sub.plan = 'featured' AND sub.starts_at <= now() AND sub.ends_at > now())`

// LinkOpenSQL is true when anyone with the link may open the profile (listed or not):
// an expired subscription hides a teacher from search, but their link keeps working.
const LinkOpenSQL = `(s.status = 'live')`

var zoneName = regexp.MustCompile(`^[A-Za-z0-9_+\-/]+$`)

// FreeSoonSQL is true when specialist s has a working window in the next 7 local days (tz)
// that is not a day off, has not ended, and is not fully covered by blocking lessons.
// It is an approximation of "has free slots this week" that stays a single SQL query.
func FreeSoonSQL(tz string) string {
	if !zoneName.MatchString(tz) {
		panic("visibility: bad time zone name " + tz)
	}
	q := `EXISTS (
	SELECT 1
	FROM generate_series(0, 6) AS g(n)
	CROSS JOIN LATERAL (SELECT (now() AT TIME ZONE 'TZ')::date + g.n AS d) day
	JOIN availability_rules ar ON ar.specialist_id = s.id AND ar.weekday = extract(dow FROM day.d)
	CROSS JOIN LATERAL (SELECT greatest((day.d + ar.starts_at) AT TIME ZONE 'TZ', now() + interval '1 hour') AS ws,
	                           (day.d + ar.ends_at) AT TIME ZONE 'TZ' AS we) w
	WHERE NOT EXISTS (SELECT 1 FROM availability_exceptions ae WHERE ae.specialist_id = s.id AND ae.day = day.d AND ae.closed)
	  AND w.we >= w.ws + make_interval(mins => s.lesson_minutes)
	  AND coalesce((SELECT sum(extract(epoch FROM upper(l.during * tstzrange(w.ws, w.we)) - lower(l.during * tstzrange(w.ws, w.we))))
	                FROM lessons l
	                WHERE l.specialist_id = s.id AND ` + lessonstatus.BlockingSQL + ` AND l.during && tstzrange(w.ws, w.we)), 0)
	      + s.lesson_minutes * 60 <= extract(epoch FROM w.we - w.ws))`
	return strings.ReplaceAll(q, "TZ", tz)
}

// RatingSQL is the average published review rating of s (NULL without reviews).
const RatingSQL = `(SELECT avg(r.rating) FROM reviews r WHERE r.specialist_id = s.id AND r.status = 'published')`

// ReviewsCountSQL counts the published reviews of s.
const ReviewsCountSQL = `(SELECT count(*) FROM reviews r WHERE r.specialist_id = s.id AND r.status = 'published')`

// RankSQL orders listed specialists: featured first, then those with free time this week,
// then by rating, then the most recently approved.
func RankSQL(tz string) string {
	return FeaturedSQL + ` DESC, ` + FreeSoonSQL(tz) + ` DESC, ` + RatingSQL + ` DESC NULLS LAST, s.approved_at DESC NULLS LAST, s.id DESC`
}
