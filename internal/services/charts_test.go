package services_test

import (
	"testing"

	"pay-dashboard/internal/services"
)

func TestToISO(t *testing.T) {
	cases := []struct{ in, want string }{
		{"1/5/2024", "2024-01-05"},
		{"12/31/2023", "2023-12-31"},
		{"2024-01-05", "2024-01-05"},
	}
	for _, c := range cases {
		got := services.ToISO(c.in)
		if got != c.want {
			t.Errorf("ToISO(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestGetChartData(t *testing.T) {
	database := newTestDB(t)
	database.MustExec("INSERT INTO people (name) VALUES (?)", "Chris")
	gross1, gross2 := 3500.0, 3600.0
	taxes1, taxes2 := 840.0, 864.0
	k1, k2 := 450.0, 450.0
	database.MustExec(`INSERT INTO pay_statements
		(person_id, source, pay_date, gross, total_taxes, total_401k)
		VALUES (1,'Safran','1/5/2024',?,?,?)`, gross1, taxes1, k1)
	database.MustExec(`INSERT INTO pay_statements
		(person_id, source, pay_date, gross, total_taxes, total_401k)
		VALUES (1,'Safran','1/19/2024',?,?,?)`, gross2, taxes2, k2)

	rows, ytd, err := services.GetChartData(database, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(rows))
	}
	if rows[0].Date != "2024-01-05" {
		t.Errorf("want ISO date 2024-01-05, got %s", rows[0].Date)
	}
	if _, ok := ytd["2024"]; !ok {
		t.Error("want ytd entry for 2024")
	}
}
