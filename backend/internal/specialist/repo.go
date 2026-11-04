package specialist

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AlikoYe/tutorhub/backend/internal/visibility"
)

// Repo is the PostgreSQL data access layer for specialists.
type Repo struct {
	db   *pgxpool.Pool
	rank string
	free string
}

// NewRepo needs the app time zone name for the "free this week" filter and ranking.
func NewRepo(db *pgxpool.Pool, tz string) *Repo {
	return &Repo{db: db, rank: visibility.RankSQL(tz), free: visibility.FreeSoonSQL(tz)}
}

// DB exposes the pool to sibling packages that extend profiles (stats, lessons).
func (r *Repo) DB() *pgxpool.Pool { return r.db }

const cardColumns = `s.id, s.kind, s.display_name, s.photo_url, s.verified, ` + visibility.FeaturedSQL + `,
	s.city, s.district, s.formats, s.languages, s.experience_years, s.trial_kind, s.trial_price,
	(SELECT min(ss.price_kzt) FROM specialist_subjects ss WHERE ss.specialist_id = s.id)`

const selectPrivate = `SELECT ` + cardColumns + `,
	s.bio, s.education, s.lesson_minutes, s.cancel_hours, s.address, ` + visibility.ListedSQL + `,
	s.status, s.rejection_reason, s.submitted_at, s.approved_at, s.lat, s.lng,
	u.telegram_id, u.name, u.username
	FROM specialists s JOIN users u ON u.telegram_id = s.user_id`

func cardDest(c *Card) []any {
	return []any{&c.ID, &c.Kind, &c.DisplayName, &c.PhotoURL, &c.Verified, &c.Featured,
		&c.City, &c.District, &c.Formats, &c.Languages, &c.ExperienceYears, &c.TrialKind, &c.TrialPrice, &c.MinPriceKZT}
}

func scanPrivate(row pgx.Row) (Private, error) {
	var p Private
	var address string
	dest := append(cardDest(&p.Card),
		&p.Bio, &p.Education, &p.LessonMinutes, &p.CancelHours, &address, &p.Listed,
		&p.Status, &p.RejectionReason, &p.SubmittedAt, &p.ApprovedAt, &p.Lat, &p.Lng,
		&p.Owner.TelegramID, &p.Owner.Name, &p.Owner.Username)
	if err := row.Scan(dest...); err != nil {
		return Private{}, err
	}
	p.Address = &address
	p.UserID = p.Owner.TelegramID
	return p, nil
}

// get loads one full profile by a WHERE clause on s.
func (r *Repo) get(ctx context.Context, where string, args ...any) (Private, error) {
	p, err := scanPrivate(r.db.QueryRow(ctx, selectPrivate+` WHERE `+where, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return Private{}, ErrNotFound
	}
	if err != nil {
		return Private{}, fmt.Errorf("select specialist: %w", err)
	}
	offers, err := r.offers(ctx, []int64{p.ID})
	if err != nil {
		return Private{}, err
	}
	p.Subjects = offers[p.ID]
	if p.Subscription, err = r.currentSubscription(ctx, p.ID); err != nil {
		return Private{}, err
	}
	return p, nil
}

func (r *Repo) Get(ctx context.Context, id int64) (Private, error) {
	return r.get(ctx, `s.id = $1`, id)
}

func (r *Repo) GetByUser(ctx context.Context, userID int64) (Private, error) {
	p, err := r.get(ctx, `s.user_id = $1`, userID)
	if errors.Is(err, ErrNotFound) {
		return Private{}, ErrNoProfile
	}
	return p, err
}

// offers loads the subjects of the given specialists, keyed by specialist id.
func (r *Repo) offers(ctx context.Context, ids []int64) (map[int64][]Offer, error) {
	rows, err := r.db.Query(ctx, `
		SELECT ss.specialist_id, sj.id, sj.slug, sj.name_ru, sj.name_kk, sj.name_en, ss.levels, ss.price_kzt,
		       coalesce(ss.lesson_minutes, sp.lesson_minutes)
		FROM specialist_subjects ss JOIN subjects sj ON sj.id = ss.subject_id JOIN specialists sp ON sp.id = ss.specialist_id
		WHERE ss.specialist_id = ANY($1)
		ORDER BY sj.position, sj.id`, ids)
	if err != nil {
		return nil, fmt.Errorf("select offers: %w", err)
	}
	defer rows.Close()
	out := make(map[int64][]Offer, len(ids))
	for _, id := range ids {
		out[id] = []Offer{}
	}
	for rows.Next() {
		var sid int64
		var o Offer
		if err := rows.Scan(&sid, &o.SubjectID, &o.Slug, &o.NameRU, &o.NameKK, &o.NameEN, &o.Levels, &o.PriceKZT, &o.LessonMinutes); err != nil {
			return nil, fmt.Errorf("scan offer: %w", err)
		}
		out[sid] = append(out[sid], o)
	}
	return out, rows.Err()
}

// currentSubscription is the subscription running now, else the latest one, else nil.
func (r *Repo) currentSubscription(ctx context.Context, specialistID int64) (*Subscription, error) {
	var s Subscription
	err := r.db.QueryRow(ctx, `
		SELECT id, plan, starts_at, ends_at, note FROM subscriptions
		WHERE specialist_id = $1
		ORDER BY (starts_at <= now() AND ends_at > now()) DESC, ends_at DESC LIMIT 1`, specialistID).
		Scan(&s.ID, &s.Plan, &s.StartsAt, &s.EndsAt, &s.Note)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("select subscription: %w", err)
	}
	return &s, nil
}

// Summary returns the user's own profile summary, or nil.
func (r *Repo) Summary(ctx context.Context, userID int64) (*Summary, error) {
	var s Summary
	err := r.db.QueryRow(ctx, `SELECT s.id, s.status, s.display_name, `+visibility.ListedSQL+`
		FROM specialists s WHERE s.user_id = $1`, userID).Scan(&s.ID, &s.Status, &s.DisplayName, &s.Listed)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("select summary: %w", err)
	}
	return &s, nil
}

// Upsert creates the user's draft profile or replaces its editable fields and subjects.
func (r *Repo) Upsert(ctx context.Context, userID int64, in Input) (int64, error) {
	var id int64
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			INSERT INTO specialists (user_id, kind, display_name, bio, education, experience_years, city, district,
			                         address, lat, lng, formats, languages, trial_kind, trial_price, lesson_minutes, cancel_hours)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
			ON CONFLICT (user_id) DO UPDATE SET
				kind = EXCLUDED.kind, display_name = EXCLUDED.display_name, bio = EXCLUDED.bio,
				education = EXCLUDED.education, experience_years = EXCLUDED.experience_years,
				city = EXCLUDED.city, district = EXCLUDED.district, address = EXCLUDED.address,
				lat = EXCLUDED.lat, lng = EXCLUDED.lng, formats = EXCLUDED.formats, languages = EXCLUDED.languages,
				trial_kind = EXCLUDED.trial_kind, trial_price = EXCLUDED.trial_price,
				lesson_minutes = EXCLUDED.lesson_minutes, cancel_hours = EXCLUDED.cancel_hours, updated_at = now()
			WHERE specialists.status <> 'blocked'
			RETURNING id`,
			userID, in.Kind, in.DisplayName, in.Bio, in.Education, in.ExperienceYears, in.City, in.District,
			in.Address, in.Lat, in.Lng, in.Formats, in.Languages, in.TrialKind, in.TrialPrice, in.LessonMinutes, *in.CancelHours,
		).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrBlocked
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM specialist_subjects WHERE specialist_id = $1`, id); err != nil {
			return err
		}
		for _, o := range in.Subjects {
			_, err := tx.Exec(ctx, `INSERT INTO specialist_subjects (specialist_id, subject_id, levels, price_kzt, lesson_minutes)
				VALUES ($1, $2, $3, $4, $5)`, id, o.SubjectID, o.Levels, o.PriceKZT, o.LessonMinutes)
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23503" {
				return fmt.Errorf("%w %d", ErrUnknownSubj, o.SubjectID)
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrBlocked) || errors.Is(err, ErrUnknownSubj) {
			return 0, err
		}
		return 0, fmt.Errorf("upsert specialist: %w", err)
	}
	return id, nil
}

// SetPhoto stores the new photo URL and returns the previous one.
func (r *Repo) SetPhoto(ctx context.Context, id int64, url string) (old *string, err error) {
	err = r.db.QueryRow(ctx, `
		UPDATE specialists s SET photo_url = $2, updated_at = now()
		FROM (SELECT photo_url FROM specialists WHERE id = $1 FOR UPDATE) prev
		WHERE s.id = $1 RETURNING prev.photo_url`, id, url).Scan(&old)
	if err != nil {
		return nil, fmt.Errorf("set photo: %w", err)
	}
	return old, nil
}

// transition moves specialist id from one of the from statuses to `to`, applying extra
// SET clauses (which may use $3...). ErrNotFound / ErrInvalidStatus when nothing changed.
func (r *Repo) transition(ctx context.Context, q querier, id int64, from []string, to, extra string, args ...any) error {
	sql := `UPDATE specialists SET status = $2, updated_at = now()`
	if extra != "" {
		sql += `, ` + extra
	}
	sql += ` WHERE id = $1 AND status = ANY($` + strconv.Itoa(len(args)+3) + `)`
	all := append([]any{id, to}, args...)
	all = append(all, from)
	tag, err := q.Exec(ctx, sql, all...)
	if err != nil {
		return fmt.Errorf("update specialist status: %w", err)
	}
	if tag.RowsAffected() == 1 {
		return nil
	}
	var exists bool
	if err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM specialists WHERE id = $1)`, id).Scan(&exists); err != nil {
		return fmt.Errorf("select specialist: %w", err)
	}
	if !exists {
		return ErrNotFound
	}
	return ErrInvalidStatus
}

type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (r *Repo) Submit(ctx context.Context, id int64) error {
	return r.transition(ctx, r.db, id, []string{"draft"}, "pending", "submitted_at = now(), rejection_reason = NULL")
}

func (r *Repo) Hide(ctx context.Context, id int64) error {
	return r.transition(ctx, r.db, id, []string{"live"}, "hidden", "")
}

func (r *Repo) Unhide(ctx context.Context, id int64) error {
	return r.transition(ctx, r.db, id, []string{"hidden"}, "live", "")
}

func (r *Repo) Reject(ctx context.Context, id int64, reason string) error {
	return r.transition(ctx, r.db, id, []string{"pending"}, "draft", "rejection_reason = $3", reason)
}

// Approve makes a pending profile live and, unless a subscription is already running,
// starts plan until endsAt. It returns whether a subscription was created.
func (r *Repo) Approve(ctx context.Context, id, adminID int64, plan string, endsAt time.Time) (bool, error) {
	created := false
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		if err := r.transition(ctx, tx, id, []string{"pending"}, "live", "approved_at = now(), rejection_reason = NULL"); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `
			INSERT INTO subscriptions (specialist_id, plan, starts_at, ends_at, created_by_admin, note)
			SELECT $1, $2, now(), $3, $4, 'on approval'
			WHERE NOT EXISTS (SELECT 1 FROM subscriptions sub
			                  WHERE sub.specialist_id = $1 AND sub.starts_at <= now() AND sub.ends_at > now())`,
			id, plan, endsAt, adminID)
		created = tag.RowsAffected() == 1
		return err
	})
	return created, err
}

// ListByStatus returns full profiles with the given status (admin queue), oldest submission first.
func (r *Repo) ListByStatus(ctx context.Context, status string, limit int) ([]Private, error) {
	rows, err := r.db.Query(ctx, selectPrivate+`
		WHERE s.status = $1 ORDER BY s.submitted_at NULLS LAST, s.id LIMIT $2`, status, limit)
	if err != nil {
		return nil, fmt.Errorf("select specialists: %w", err)
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Private, error) { return scanPrivate(row) })
	if err != nil {
		return nil, fmt.Errorf("scan specialists: %w", err)
	}
	ids := make([]int64, len(out))
	for i := range out {
		ids[i] = out[i].ID
	}
	offers, err := r.offers(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Subjects = offers[out[i].ID]
		if out[i].Subscription, err = r.currentSubscription(ctx, out[i].ID); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// SetFavourite adds or removes a favourite; adding needs a live profile (false otherwise).
func (r *Repo) SetFavourite(ctx context.Context, userID, specialistID int64, on bool) (bool, error) {
	if !on {
		_, err := r.db.Exec(ctx, `DELETE FROM favourites WHERE user_id = $1 AND specialist_id = $2`, userID, specialistID)
		return true, err
	}
	tag, err := r.db.Exec(ctx, `INSERT INTO favourites (user_id, specialist_id)
		SELECT $1, s.id FROM specialists s WHERE s.id = $2 AND `+visibility.LinkOpenSQL+`
		ON CONFLICT DO NOTHING`, userID, specialistID)
	if err != nil {
		return false, fmt.Errorf("add favourite: %w", err)
	}
	if tag.RowsAffected() == 1 {
		return true, nil
	}
	var exists bool
	err = r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM favourites WHERE user_id = $1 AND specialist_id = $2)`, userID, specialistID).Scan(&exists)
	return exists, err
}

func (r *Repo) IsFavourite(ctx context.Context, userID, specialistID int64) (bool, error) {
	var ok bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM favourites WHERE user_id = $1 AND specialist_id = $2)`, userID, specialistID).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("select favourite: %w", err)
	}
	return ok, nil
}

// Favourites returns the user's favourite live specialists, newest first.
func (r *Repo) Favourites(ctx context.Context, userID int64) ([]Card, error) {
	rows, err := r.db.Query(ctx, `SELECT `+cardColumns+` FROM specialists s JOIN favourites f ON f.specialist_id = s.id
		WHERE f.user_id = $1 AND `+visibility.LinkOpenSQL+` ORDER BY f.created_at DESC LIMIT 200`, userID)
	if err != nil {
		return nil, fmt.Errorf("select favourites: %w", err)
	}
	defer rows.Close()
	out := []Card{}
	for rows.Next() {
		var c Card
		if err := rows.Scan(cardDest(&c)...); err != nil {
			return nil, fmt.Errorf("scan favourite: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	ids := make([]int64, len(out))
	for i := range out {
		ids[i] = out[i].ID
	}
	offers, err := r.offers(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Subjects = offers[out[i].ID]
	}
	return out, nil
}

// IDByUser returns the id of the user's own profile.
func (r *Repo) IDByUser(ctx context.Context, userID int64) (int64, error) {
	var id int64
	err := r.db.QueryRow(ctx, `SELECT id FROM specialists WHERE user_id = $1`, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNoProfile
	}
	if err != nil {
		return 0, fmt.Errorf("select specialist id: %w", err)
	}
	return id, nil
}

// AdminIDs lists every admin's Telegram id.
func (r *Repo) AdminIDs(ctx context.Context) ([]int64, error) {
	rows, err := r.db.Query(ctx, `SELECT telegram_id FROM users WHERE role = 'admin' ORDER BY telegram_id`)
	if err != nil {
		return nil, fmt.Errorf("select admins: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowTo[int64])
}

// Filter is a catalogue search. Zero values mean "any".
type Filter struct {
	Kind, City, District, Language, Format, Level, Query string
	SubjectID                                            int64
	PriceMin, PriceMax                                   *int
	VerifiedOnly, FreeThisWeek                           bool
	Limit, Offset                                        int
}

// Page is one page of search results.
type Page struct {
	Items []Card `json:"items"`
	Total int    `json:"total"`
}

// Search returns listed specialists matching f, ranked by visibility.RankSQL.
func (r *Repo) Search(ctx context.Context, f Filter) (Page, error) {
	var where []string
	var args []any
	arg := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}
	where = append(where, visibility.ListedSQL)
	if f.Kind != "" {
		where = append(where, "s.kind = "+arg(f.Kind))
	}
	if f.City != "" {
		where = append(where, "s.city = "+arg(f.City))
	}
	if f.District != "" {
		where = append(where, "s.district = "+arg(f.District))
	}
	if f.Language != "" {
		where = append(where, arg(f.Language)+" = ANY(s.languages)")
	}
	if f.Format != "" {
		where = append(where, arg(f.Format)+" = ANY(s.formats)")
	}
	if f.VerifiedOnly {
		where = append(where, "s.verified")
	}
	if f.FreeThisWeek {
		where = append(where, r.free)
	}
	// Subject, level and price must all match the same offer.
	var offer []string
	if f.SubjectID != 0 {
		offer = append(offer, "ss.subject_id = "+arg(f.SubjectID))
	}
	if f.Level != "" {
		offer = append(offer, arg(f.Level)+" = ANY(ss.levels)")
	}
	if f.PriceMin != nil {
		offer = append(offer, "ss.price_kzt >= "+arg(*f.PriceMin))
	}
	if f.PriceMax != nil {
		offer = append(offer, "ss.price_kzt <= "+arg(*f.PriceMax))
	}
	if len(offer) > 0 {
		where = append(where, "EXISTS (SELECT 1 FROM specialist_subjects ss WHERE ss.specialist_id = s.id AND "+strings.Join(offer, " AND ")+")")
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		p := arg("%" + escapeLike(q) + "%")
		where = append(where, `(s.display_name ILIKE `+p+` OR s.bio ILIKE `+p+` OR EXISTS (
			SELECT 1 FROM specialist_subjects ss JOIN subjects sj ON sj.id = ss.subject_id
			WHERE ss.specialist_id = s.id AND (sj.name_ru ILIKE `+p+` OR sj.name_kk ILIKE `+p+` OR sj.name_en ILIKE `+p+` OR sj.slug ILIKE `+p+`)))`)
	}

	sql := `SELECT ` + cardColumns + `, count(*) OVER () FROM specialists s
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY ` + r.rank + `
		LIMIT ` + arg(f.Limit) + ` OFFSET ` + arg(f.Offset)
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return Page{}, fmt.Errorf("search specialists: %w", err)
	}
	defer rows.Close()
	page := Page{Items: []Card{}}
	for rows.Next() {
		var c Card
		if err := rows.Scan(append(cardDest(&c), &page.Total)...); err != nil {
			return Page{}, fmt.Errorf("scan card: %w", err)
		}
		page.Items = append(page.Items, c)
	}
	if err := rows.Err(); err != nil {
		return Page{}, err
	}
	if len(page.Items) == 0 && f.Offset > 0 {
		// count(*) OVER () is empty past the last page; count separately.
		if err := r.db.QueryRow(ctx, `SELECT count(*) FROM specialists s WHERE `+strings.Join(where, " AND "), args[:len(args)-2]...).Scan(&page.Total); err != nil {
			return Page{}, fmt.Errorf("count specialists: %w", err)
		}
	}
	ids := make([]int64, len(page.Items))
	for i, c := range page.Items {
		ids[i] = c.ID
	}
	offers, err := r.offers(ctx, ids)
	if err != nil {
		return Page{}, err
	}
	for i := range page.Items {
		page.Items[i].Subjects = offers[page.Items[i].ID]
	}
	return page, nil
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
