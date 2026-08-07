package web

import (
	"testing"

	"github.com/wallacepnts/vaultzap/internal/render"
	"github.com/wallacepnts/vaultzap/internal/store"
)

func TestBuildCalendar(t *testing.T) {
	chat := store.Chat{
		ID:             7,
		FirstMessageAt: "2022-08-14 10:00:00",
		LastMessageAt:  "2022-10-02 18:30:00",
	}
	days := []store.DayWithMessages{
		{Date: "2022-09-01", Total: 125, FirstID: 11},
		{Date: "2022-09-22", Total: 621, FirstID: 22},
		{Date: "2022-09-30", Total: 6, FirstID: 33},
	}

	c := buildCalendar(chat, "2022-09", days, render.LocalePTBR)

	if c.Title != "setembro de 2022" {
		t.Errorf("título = %q", c.Title)
	}
	if c.Previous != "2022-08" || c.Next != "2022-10" {
		t.Errorf("navegação = %q / %q, esperado 2022-08 / 2022-10", c.Previous, c.Next)
	}
	if len(c.Weeks) != 5 {
		t.Fatalf("%d semanas, esperado 5", len(c.Weeks))
	}
	for i, week := range c.Weeks {
		if len(week) != 7 {
			t.Errorf("semana %d tem %d células, toda semana precisa de 7", i, len(week))
		}
	}
	for i := 0; i < 4; i++ {
		if c.Weeks[0][i].Day != 0 {
			t.Errorf("célula %d da primeira semana devia ser vazia, veio dia %d", i, c.Weeks[0][i].Day)
		}
	}
	if d := c.Weeks[0][4]; d.Day != 1 || d.Total != 125 || d.FirstID != 11 {
		t.Errorf("dia 1 = %+v", d)
	}
	seen := map[int]CalendarDay{}
	for _, week := range c.Weeks {
		for _, d := range week {
			if d.Day == 0 {
				continue
			}
			if _, repeated := seen[d.Day]; repeated {
				t.Errorf("dia %d apareceu duas vezes", d.Day)
			}
			seen[d.Day] = d
		}
	}
	if len(seen) != 30 {
		t.Errorf("%d dias no grid, esperado 30", len(seen))
	}
	if p := seen[22].Weight; p != 4 {
		t.Errorf("peso do dia mais movimentado = %d, esperado 4", p)
	}
	if p := seen[30].Weight; p != 1 {
		t.Errorf("peso do dia mais fraco = %d, esperado 1", p)
	}
	if p := seen[15].Weight; p != 0 {
		t.Errorf("dia sem mensagem tem peso %d, esperado 0", p)
	}

	if c.CurrentYear != "2022" || c.CurrentMonth != "09" {
		t.Errorf("CurrentYear/CurrentMonth = %q/%q, esperado 2022/09", c.CurrentYear, c.CurrentMonth)
	}
	if len(c.Months) != 12 || c.Months[0].Number != "01" || c.Months[8].Name != "setembro" {
		t.Errorf("Months malformado: %+v", c.Months)
	}
	if got := c.Years; len(got) != 1 || got[0] != "2022" {
		t.Errorf("Years = %v, esperado [2022]", got)
	}
	if c.CurrentMonthName != "setembro" {
		t.Errorf("CurrentMonthName = %q, esperado setembro", c.CurrentMonthName)
	}

	if len(c.Days) != 3 {
		t.Fatalf("%d dias na lista, esperado 3", len(c.Days))
	}
	if got := c.Days[0]; got.Label != "quinta-feira, 1 de setembro de 2022" || got.Total != 125 || got.FirstID != 11 || got.Weight != 1 {
		t.Errorf("Days[0] = %+v", got)
	}
	if got := c.Days[1]; got.Weight != 4 {
		t.Errorf("Days[1].Weight = %d, esperado 4 (dia mais movimentado)", got.Weight)
	}
	if got := c.Days[2]; got.Weight != 1 {
		t.Errorf("Days[2].Weight = %d, esperado 1 (dia mais fraco)", got.Weight)
	}
}

func TestYearsBetween(t *testing.T) {
	if got := yearsBetween("2021-11", "2023-02"); len(got) != 3 || got[0] != "2021" || got[2] != "2023" {
		t.Errorf("yearsBetween(2021-11, 2023-02) = %v, esperado [2021 2022 2023]", got)
	}
	if got := yearsBetween("", "2023-02"); got != nil {
		t.Errorf("mês vazio devia devolver nil, veio %v", got)
	}
}

func TestBuildCalendar_stopsAtTheEnds(t *testing.T) {
	chat := store.Chat{
		FirstMessageAt: "2022-09-01 10:00:00",
		LastMessageAt:  "2022-09-30 10:00:00",
	}
	c := buildCalendar(chat, "2022-09", nil, render.LocalePTBR)
	if c.Previous != "" || c.Next != "" {
		t.Errorf("mês único devia travar a navegação, veio %q / %q", c.Previous, c.Next)
	}
}
