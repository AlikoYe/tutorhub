// Package lessonstatus is the single source of truth for which lessons hold a teacher's and a
// student's time and what status a reader sees. SQL fragments expect lessons aliased as "l".
package lessonstatus

// Values of lessons.status.
const (
	Pending   = "pending"
	Confirmed = "confirmed"
	Declined  = "declined"
	Cancelled = "cancelled"
	Expired   = "expired"
	Completed = "completed"
)

// BlockingSQL is true when lesson l holds its teacher's and student's time.
const BlockingSQL = `(l.status = 'confirmed' OR (l.status = 'pending' AND l.expires_at > now()))`

// EffectiveSQL is the status to report: an overdue request reads as expired and a confirmed
// lesson that has ended reads as completed, even before the worker updates the row.
const EffectiveSQL = `(CASE
	WHEN l.status = 'pending' AND l.expires_at <= now() THEN 'expired'
	WHEN l.status = 'confirmed' AND upper(l.during) <= now() THEN 'completed'
	ELSE l.status END)`

// ExpireOverdueSQL flips overdue requests of specialist $1 or student $2 to expired; run it before inserting.
const ExpireOverdueSQL = `
	UPDATE lessons SET status = 'expired', updated_at = now()
	WHERE status = 'pending' AND expires_at <= now() AND (specialist_id = $1 OR student_id = $2)`

// ExpireAllOverdueSQL flips every overdue request to expired and returns the ids (worker).
const ExpireAllOverdueSQL = `
	UPDATE lessons SET status = 'expired', updated_at = now()
	WHERE status = 'pending' AND expires_at <= now()
	RETURNING id`

// CompleteFinishedSQL marks confirmed lessons that have ended as completed and returns the ids (worker).
const CompleteFinishedSQL = `
	UPDATE lessons SET status = 'completed', updated_at = now()
	WHERE status = 'confirmed' AND upper(during) <= now()
	RETURNING id`

// DecideSQL sets $2 ('confirmed' or 'declined') on lesson $1 only while it is still a live request.
const DecideSQL = `
	UPDATE lessons SET status = $2, decided_at = now(), updated_at = now()
	WHERE id = $1 AND status = 'pending' AND expires_at > now()`

// ClaimRemindersSQL marks confirmed lessons starting within $1 seconds as reminded and returns the ids.
const ClaimRemindersSQL = `
	UPDATE lessons SET reminder_sent = true, updated_at = now()
	WHERE status = 'confirmed' AND NOT reminder_sent
	  AND lower(during) > now() AND lower(during) <= now() + make_interval(secs => $1)
	RETURNING id`
