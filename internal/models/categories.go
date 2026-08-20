package models

import "strings"

const (
	GroupFashion = "Мода"
	GroupTech    = "Техника"
	GroupHome    = "Дом"
	GroupBeauty  = "Красота"
	GroupKids    = "Дети и животные"
	GroupSport   = "Спорт и хобби"
	GroupFood    = "Продукты"
)

type Category struct {
	Slug  string `json:"slug"`
	Name  string `json:"name"`
	Icon  string `json:"icon,omitempty"`
	Group string `json:"group"`
}

// Categories — единственный источник правды витрины WOAson (только физические товары).
var Categories = []Category{
	{Slug: "odezhda", Name: "Одежда", Icon: "👗", Group: GroupFashion},
	{Slug: "obuv", Name: "Обувь", Icon: "👟", Group: GroupFashion},
	{Slug: "aksessuary", Name: "Аксессуары", Icon: "👜", Group: GroupFashion},
	{Slug: "ukrasheniya", Name: "Украшения", Icon: "💍", Group: GroupFashion},

	{Slug: "elektronika", Name: "Электроника", Icon: "📱", Group: GroupTech},
	{Slug: "bytovaya", Name: "Бытовая техника", Icon: "🧺", Group: GroupTech},
	{Slug: "kompjutery", Name: "Компьютеры", Icon: "💻", Group: GroupTech},
	{Slug: "igry", Name: "Игры", Icon: "🎮", Group: GroupTech},

	{Slug: "dom", Name: "Дом", Icon: "🏠", Group: GroupHome},
	{Slug: "mebel", Name: "Мебель", Icon: "🛋️", Group: GroupHome},
	{Slug: "kuhnya", Name: "Кухня", Icon: "🍳", Group: GroupHome},
	{Slug: "sad", Name: "Сад и дача", Icon: "🌱", Group: GroupHome},
	{Slug: "remont", Name: "Ремонт", Icon: "🔧", Group: GroupHome},

	{Slug: "krasota", Name: "Красота", Icon: "✨", Group: GroupBeauty},
	{Slug: "zdorovie", Name: "Здоровье", Icon: "💊", Group: GroupBeauty},

	{Slug: "detiam", Name: "Детям", Icon: "🧸", Group: GroupKids},
	{Slug: "zootovary", Name: "Зоотовары", Icon: "🐾", Group: GroupKids},

	{Slug: "sport", Name: "Спорт", Icon: "🏃", Group: GroupSport},
	{Slug: "hobbi", Name: "Хобби", Icon: "🎨", Group: GroupSport},
	{Slug: "knigi", Name: "Книги", Icon: "📚", Group: GroupSport},
	{Slug: "kantselyariya", Name: "Канцтовары", Icon: "✏️", Group: GroupSport},

	{Slug: "produkty", Name: "Продукты", Icon: "🍏", Group: GroupFood},
}

var BannedCategories = []string{
	"avto", "nedvizhimost", "transport", "uslugi", "rabota", "travel",
}

var goodsIndex = map[string]Category{}
var bannedIndex = map[string]struct{}{}

func init() {
	for _, c := range Categories {
		goodsIndex[c.Slug] = c
	}
	for _, s := range BannedCategories {
		bannedIndex[s] = struct{}{}
	}
}

func NormalizeCategory(slug string) string {
	return strings.ToLower(strings.TrimSpace(slug))
}

func IsGoodsCategory(slug string) bool {
	_, ok := goodsIndex[NormalizeCategory(slug)]
	return ok
}

func IsBannedCategory(slug string) bool {
	_, ok := bannedIndex[NormalizeCategory(slug)]
	return ok
}

func GoodsSlugs() []string {
	out := make([]string, len(Categories))
	for i, c := range Categories {
		out[i] = c.Slug
	}
	return out
}

func ValidDeliveryMethods(in []string) bool {
	for _, m := range in {
		switch m {
		case "cdek", "pochta", "pickup":
		default:
			return false
		}
	}
	return true
}
