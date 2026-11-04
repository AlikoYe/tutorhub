// Package site renders the public website: server-side teacher pages and "subject + city"
// catalogue pages with SEO tags, a sitemap and robots.txt. It needs no Telegram auth.
package site

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AlikoYe/tutorhub/backend/internal/geo"
	"github.com/AlikoYe/tutorhub/backend/internal/review"
	"github.com/AlikoYe/tutorhub/backend/internal/specialist"
	"github.com/AlikoYe/tutorhub/backend/internal/subject"
	"github.com/AlikoYe/tutorhub/backend/internal/visibility"
)

//go:embed templates/*.html
var templatesFS embed.FS

// Events records anonymous profile views. Optional.
type Events interface {
	Viewed(ctx context.Context, specialistID, userID int64)
}

// Options configures the site.
type Options struct {
	BaseURL     string // e.g. https://tutorhub.kz (no trailing slash)
	WebAppURL   string
	BotUsername string
}

type Handler struct {
	db          *pgxpool.Pool
	specialists *specialist.Service
	reviews     *review.Service
	subjects    *subject.Repo
	events      Events
	opts        Options
	log         *slog.Logger
	teacherTmpl *template.Template
	listTmpl    *template.Template
}

func NewHandler(db *pgxpool.Pool, specialists *specialist.Service, reviews *review.Service, subjects *subject.Repo, events Events, opts Options, log *slog.Logger) *Handler {
	funcs := template.FuncMap{
		"price": formatPrice,
		"stars": func(n int) string { return strings.Repeat("★", n) + strings.Repeat("☆", 5-n) },
		"deref": func(f *float64) float64 { return *f },
	}
	parse := func(page string) *template.Template {
		return template.Must(template.New("").Funcs(funcs).ParseFS(templatesFS, "templates/layout.html", "templates/"+page))
	}
	return &Handler{db: db, specialists: specialists, reviews: reviews, subjects: subjects, events: events, opts: opts, log: log,
		teacherTmpl: parse("teacher.html"), listTmpl: parse("list.html")}
}

// Routes mounts the public pages at the root.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/t/{id}", h.teacher)
	r.Get("/c/{city}", h.city)
	r.Get("/c/{city}/{subject}", h.subjectCity)
	r.Get("/sitemap.xml", h.sitemap)
	r.Get("/robots.txt", h.robots)
}

type alternate struct {
	Lang, Label, URL string
}

type page struct {
	Lang, Title, Description, Canonical, Image, Home string
	Alternates                                       []alternate
	Noindex                                          bool
	JSONLD                                           template.JS
	Tx                                               map[string]string
	Data                                             any
}

func lang(r *http.Request) string {
	l := r.URL.Query().Get("lang")
	for _, x := range Langs {
		if l == x {
			return l
		}
	}
	return Langs[0]
}

// url builds an absolute site URL for path in lang (the default language has no parameter).
func (h *Handler) url(path, lang string) string {
	u := h.opts.BaseURL + path
	if lang != Langs[0] {
		u += "?lang=" + lang
	}
	return u
}

func (h *Handler) newPage(path, lng string) page {
	p := page{Lang: lng, Canonical: h.url(path, lng), Home: h.url("/c/"+geo.DefaultCity, lng), Tx: texts[lng]}
	labels := map[string]string{"ru": "Рус", "kk": "Қаз", "en": "Eng"}
	for _, l := range Langs {
		p.Alternates = append(p.Alternates, alternate{Lang: l, Label: labels[l], URL: h.url(path, l)})
	}
	return p
}

func (h *Handler) render(w http.ResponseWriter, r *http.Request, tmpl *template.Template, status int, p page) {
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "layout", p); err != nil {
		h.log.ErrorContext(r.Context(), "render page", slog.Any("err", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

func (h *Handler) notFound(w http.ResponseWriter, r *http.Request, lng string) {
	p := h.newPage(r.URL.Path, lng)
	p.Title, p.Noindex = "TutorHub", true
	p.Data = listData{Heading: "404", Intro: t(lng, "nobody")}
	h.render(w, r, h.listTmpl, http.StatusNotFound, p)
}

func (h *Handler) internal(w http.ResponseWriter, r *http.Request, err error) {
	h.log.ErrorContext(r.Context(), "site", slog.Any("err", err))
	http.Error(w, "internal error", http.StatusInternalServerError)
}

func formatPrice(n int) string {
	s := strconv.Itoa(n)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteRune(' ')
		}
		b.WriteRune(c)
	}
	return b.String() + " ₸"
}

type offerView struct {
	Name, Levels string
	Price        int
}

type teacherData struct {
	Profile            specialist.Public
	Place              string
	Offers             []offerView
	Formats, Languages string
	Reviews            []review.Review
	BookURL, ShareURL  string
}

func (h *Handler) teacher(w http.ResponseWriter, r *http.Request) {
	lng := lang(r)
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		h.notFound(w, r, lng)
		return
	}
	prof, err := h.specialists.Profile(r.Context(), specialist.Viewer{}, id)
	if errors.Is(err, specialist.ErrNotFound) {
		h.notFound(w, r, lng)
		return
	}
	if err != nil {
		h.internal(w, r, err)
		return
	}
	reviews, err := h.reviews.List(r.Context(), specialist.Viewer{}, id)
	if err != nil {
		h.internal(w, r, err)
		return
	}
	if len(reviews) > 10 {
		reviews = reviews[:10]
	}
	if h.events != nil {
		h.events.Viewed(r.Context(), id, 0)
	}

	city, _ := geo.FindCity(prof.City)
	place := placeName(city.Place, lng)
	if d := districtName(prof.City, prof.District, lng); d != "" {
		place += ", " + d
	}
	d := teacherData{Profile: prof, Place: place, Reviews: reviews}
	var names []string
	for _, o := range prof.Subjects {
		var lv []string
		for _, l := range o.Levels {
			lv = append(lv, levelNames[lng][l])
		}
		d.Offers = append(d.Offers, offerView{Name: offerName(o, lng), Levels: strings.Join(lv, ", "), Price: o.PriceKZT})
		names = append(names, offerName(o, lng))
	}
	var fm, lg []string
	for _, f := range prof.Formats {
		fm = append(fm, formatNames[lng][f])
	}
	for _, l := range prof.Languages {
		lg = append(lg, languageNames[l])
	}
	d.Formats, d.Languages = strings.Join(fm, ", "), strings.Join(lg, ", ")
	if h.opts.BotUsername != "" {
		d.BookURL = "https://t.me/" + h.opts.BotUsername + "?start=t_" + strconv.FormatInt(id, 10)
	} else if h.opts.WebAppURL != "" {
		d.BookURL = strings.TrimRight(h.opts.WebAppURL, "/") + "/specialists/" + strconv.FormatInt(id, 10)
	}

	path := "/t/" + strconv.FormatInt(id, 10)
	p := h.newPage(path, lng)
	d.ShareURL = "https://t.me/share/url?" + url.Values{"url": {p.Canonical}, "text": {prof.DisplayName + " — TutorHub"}}.Encode()
	minPrice := ""
	if prof.MinPriceKZT != nil {
		minPrice = formatPrice(*prof.MinPriceKZT)
	}
	p.Title = prof.DisplayName + " — " + strings.Join(names, ", ") + " | TutorHub"
	p.Description = t(lng, "teacherDesc", prof.DisplayName, strings.Join(names, ", "), place, minPrice)
	p.Noindex = !prof.Listed
	if prof.PhotoURL != nil {
		p.Image = h.opts.BaseURL + *prof.PhotoURL
	}
	p.JSONLD = jsonLD(map[string]any{
		"@context":    "https://schema.org",
		"@type":       "Person",
		"name":        prof.DisplayName,
		"jobTitle":    "Tutor",
		"description": prof.Bio,
		"knowsAbout":  names,
		"url":         p.Canonical,
		"address":     map[string]any{"@type": "PostalAddress", "addressLocality": placeName(city.Place, "en"), "addressCountry": "KZ"},
	})
	p.Data = d
	h.render(w, r, h.teacherTmpl, http.StatusOK, p)
}

func jsonLD(v any) template.JS {
	b, _ := json.Marshal(v) // json.Marshal escapes <, > and & so the script cannot be closed early
	return template.JS(b)
}

type itemView struct {
	URL, Name, Subjects, Meta string
	Photo                     *string
	MinPrice                  int
	Rating                    float64
	Verified                  bool
}

type linkView struct {
	URL, Label string
	Count      int
}

type listData struct {
	Heading, Intro, LinksTitle string
	Items                      []itemView
	Total                      int
	Links                      []linkView
}

func (h *Handler) items(page specialist.Page, lng string) []itemView {
	out := []itemView{}
	for _, c := range page.Items {
		var names []string
		for _, o := range c.Subjects {
			names = append(names, offerName(o, lng))
		}
		var fm []string
		for _, f := range c.Formats {
			fm = append(fm, formatNames[lng][f])
		}
		meta := strings.Join(fm, ", ")
		if d := districtName(c.City, c.District, lng); d != "" {
			meta = d + " · " + meta
		}
		it := itemView{URL: h.url("/t/"+strconv.FormatInt(c.ID, 10), lng), Name: c.DisplayName, Subjects: strings.Join(names, " · "),
			Meta: meta, Photo: c.PhotoURL, Verified: c.Verified}
		if c.MinPriceKZT != nil {
			it.MinPrice = *c.MinPriceKZT
		}
		if c.Rating != nil {
			it.Rating = *c.Rating
		}
		out = append(out, it)
	}
	return out
}

// subjectCounts counts listed specialists per subject in a city.
func (h *Handler) subjectCounts(ctx context.Context, city string) (map[int64]int, error) {
	rows, err := h.db.Query(ctx, `SELECT ss.subject_id, count(*) FROM specialist_subjects ss JOIN specialists s ON s.id = ss.specialist_id
		WHERE s.city = $1 AND `+visibility.ListedSQL+` GROUP BY ss.subject_id`, city)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]int{}
	for rows.Next() {
		var id int64
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

func (h *Handler) subjectLinks(ctx context.Context, city, lng string, skip int64) ([]linkView, error) {
	counts, err := h.subjectCounts(ctx, city)
	if err != nil {
		return nil, err
	}
	all, err := h.subjects.List(ctx)
	if err != nil {
		return nil, err
	}
	var out []linkView
	for _, s := range all {
		if counts[s.ID] > 0 && s.ID != skip {
			out = append(out, linkView{URL: h.url("/c/"+city+"/"+s.Slug, lng), Label: subjectName(s, lng), Count: counts[s.ID]})
		}
	}
	return out, nil
}

func (h *Handler) city(w http.ResponseWriter, r *http.Request) {
	lng := lang(r)
	c, ok := geo.FindCity(chi.URLParam(r, "city"))
	if !ok {
		h.notFound(w, r, lng)
		return
	}
	res, err := h.specialists.Search(r.Context(), specialist.Filter{City: c.Slug, Limit: 30})
	if err != nil {
		h.internal(w, r, err)
		return
	}
	links, err := h.subjectLinks(r.Context(), c.Slug, lng, 0)
	if err != nil {
		h.internal(w, r, err)
		return
	}
	name := placeName(c.Place, lng)
	p := h.newPage("/c/"+c.Slug, lng)
	p.Title = t(lng, "tutorsIn", name) + " | TutorHub"
	p.Description = t(lng, "cityDesc", name)
	p.Data = listData{Heading: t(lng, "tutorsIn", name), Intro: t(lng, "tagline"), Items: h.items(res, lng), Total: res.Total,
		Links: links, LinksTitle: t(lng, "allSubjects")}
	h.render(w, r, h.listTmpl, http.StatusOK, p)
}

func (h *Handler) subjectCity(w http.ResponseWriter, r *http.Request) {
	lng := lang(r)
	c, ok := geo.FindCity(chi.URLParam(r, "city"))
	if !ok {
		h.notFound(w, r, lng)
		return
	}
	var subj *subject.Subject
	all, err := h.subjects.List(r.Context())
	if err != nil {
		h.internal(w, r, err)
		return
	}
	for i := range all {
		if all[i].Slug == chi.URLParam(r, "subject") {
			subj = &all[i]
		}
	}
	if subj == nil {
		h.notFound(w, r, lng)
		return
	}
	res, err := h.specialists.Search(r.Context(), specialist.Filter{City: c.Slug, SubjectID: subj.ID, Limit: 50})
	if err != nil {
		h.internal(w, r, err)
		return
	}
	links, err := h.subjectLinks(r.Context(), c.Slug, lng, subj.ID)
	if err != nil {
		h.internal(w, r, err)
		return
	}
	city, sname := placeName(c.Place, lng), subjectName(*subj, lng)
	p := h.newPage("/c/"+c.Slug+"/"+subj.Slug, lng)
	p.Title = t(lng, "tutorsFor", sname, city) + " | TutorHub"
	minPrice := 0
	for _, it := range res.Items {
		for _, o := range it.Subjects {
			if o.SubjectID == subj.ID && (minPrice == 0 || o.PriceKZT < minPrice) {
				minPrice = o.PriceKZT
			}
		}
	}
	p.Description = t(lng, "tutorsForDesc", res.Total, sname, city, formatPrice(minPrice))
	// Empty pages are thin content: keep them out of the index.
	p.Noindex = res.Total == 0
	p.Data = listData{Heading: t(lng, "tutorsFor", sname, city), Items: h.items(res, lng), Total: res.Total,
		Links: links, LinksTitle: t(lng, "allSubjects")}
	h.render(w, r, h.listTmpl, http.StatusOK, p)
}

type sitemapURL struct {
	Loc string `xml:"loc"`
}

type urlset struct {
	XMLName xml.Name     `xml:"urlset"`
	XMLNS   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

func (h *Handler) sitemap(w http.ResponseWriter, r *http.Request) {
	set := urlset{XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9"}
	add := func(path string) {
		for _, l := range Langs {
			set.URLs = append(set.URLs, sitemapURL{Loc: h.url(path, l)})
		}
	}
	all, err := h.subjects.List(r.Context())
	if err != nil {
		h.internal(w, r, err)
		return
	}
	for _, c := range geo.Cities {
		add("/c/" + c.Slug)
		counts, err := h.subjectCounts(r.Context(), c.Slug)
		if err != nil {
			h.internal(w, r, err)
			return
		}
		for _, s := range all {
			if counts[s.ID] > 0 {
				add("/c/" + c.Slug + "/" + s.Slug)
			}
		}
	}
	rows, err := h.db.Query(r.Context(), `SELECT s.id FROM specialists s WHERE `+visibility.ListedSQL+` ORDER BY s.id`)
	if err != nil {
		h.internal(w, r, err)
		return
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		h.internal(w, r, err)
		return
	}
	for _, id := range ids {
		add("/t/" + strconv.FormatInt(id, 10))
	}
	out, err := xml.MarshalIndent(set, "", "  ")
	if err != nil {
		h.internal(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(out)
}

func (h *Handler) robots(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "User-agent: *\nAllow: /t/\nAllow: /c/\nDisallow: /api/\nDisallow: /admin\nDisallow: /teach\n\nSitemap: %s/sitemap.xml\n", h.opts.BaseURL)
}
