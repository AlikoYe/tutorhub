// Package visibility is the single source of truth for which specialists are listed in the
// catalogue and how they rank. SQL fragments expect specialists aliased as "s"; use them,
// never re-type the rule.
package visibility

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

// RankSQL orders listed specialists: featured first, then newest approval.
const RankSQL = FeaturedSQL + ` DESC, s.approved_at DESC NULLS LAST, s.id DESC`
