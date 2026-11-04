// Package geo lists the cities and districts the catalogue knows, plus the fixed
// vocabularies (levels, formats, languages) shared by the API and the database CHECKs.
package geo

import "slices"

type Place struct {
	Slug   string `json:"slug"`
	NameRU string `json:"name_ru"`
	NameKK string `json:"name_kk"`
	NameEN string `json:"name_en"`
}

type City struct {
	Place
	Districts []Place `json:"districts"`
}

// DefaultCity is the launch city.
const DefaultCity = "almaty"

// Cities at launch: Almaty only; the data model supports any city.
var Cities = []City{
	{
		Place: Place{"almaty", "Алматы", "Алматы", "Almaty"},
		Districts: []Place{
			{"alatau", "Алатауский", "Алатау", "Alatau"},
			{"almaly", "Алмалинский", "Алмалы", "Almaly"},
			{"auezov", "Ауэзовский", "Әуезов", "Auezov"},
			{"bostandyk", "Бостандыкский", "Бостандық", "Bostandyk"},
			{"zhetysu", "Жетысуский", "Жетісу", "Zhetysu"},
			{"medeu", "Медеуский", "Медеу", "Medeu"},
			{"nauryzbay", "Наурызбайский", "Наурызбай", "Nauryzbay"},
			{"turksib", "Турксибский", "Түрксіб", "Turksib"},
		},
	},
}

var (
	Levels    = []string{"primary", "middle", "high", "ent", "university", "adult"}
	Formats   = []string{"online", "teacher_place", "student_home"}
	Languages = []string{"kk", "ru", "en"}
	Kinds     = []string{"teacher", "consultant"}
)

// FindCity returns the city with slug, if known.
func FindCity(slug string) (City, bool) {
	for _, c := range Cities {
		if c.Slug == slug {
			return c, true
		}
	}
	return City{}, false
}

// ValidDistrict reports whether district belongs to city ("" is always valid).
func ValidDistrict(city, district string) bool {
	if district == "" {
		return true
	}
	c, ok := FindCity(city)
	if !ok {
		return false
	}
	return slices.ContainsFunc(c.Districts, func(p Place) bool { return p.Slug == district })
}

// AllIn reports whether every value is in allowed and there are no duplicates.
func AllIn(values, allowed []string) bool {
	seen := map[string]bool{}
	for _, v := range values {
		if !slices.Contains(allowed, v) || seen[v] {
			return false
		}
		seen[v] = true
	}
	return true
}

// Meta is the fixed vocabulary the client needs to build filters and forms.
type Meta struct {
	Cities    []City   `json:"cities"`
	Levels    []string `json:"levels"`
	Formats   []string `json:"formats"`
	Languages []string `json:"languages"`
	Kinds     []string `json:"kinds"`
}

// CurrentMeta returns the vocabulary.
func CurrentMeta() Meta {
	return Meta{Cities: Cities, Levels: Levels, Formats: Formats, Languages: Languages, Kinds: Kinds}
}
