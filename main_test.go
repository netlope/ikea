package ikea

import (
	"testing"

	"netlope.de/billmate/backend/billmate"
)

type user struct {
	billmate.User
	Valid bool
}

func BenchmarkFilter(b *testing.B) {

	var filters []Filter
	for i := 0; i < 5; i++ {
		filters = append(filters, Filter{"_skip", true})
	}

	var fields []field
	for i := 0; i < 25; i++ {
		fields = append(fields, field{"field", "value"})
	}

	var row row
	row.fields = fields

	var rows rows
	for i := 0; i < 100; i++ {
		rows = append(rows, row)
	}

	b.SetBytes(2)
	for n := 0; n < b.N; n++ {
		rows.Filter(filters...)
	}

}

func BenchmarkToStruct(b *testing.B) {

	users := rows{}
	user1 := row{}
	user2 := row{}

	user1.fields = append(user1.fields,
		field{
			key:   "password",
			value: "Tim",
		}, field{
			key:   "email",
			value: "email@domain.tld",
		}, field{
			key:   "valid",
			value: true,
		})

	user2.fields = append(user2.fields,
		field{"password", "Bob"},
		field{"email", "email2@domain.tld"},
		field{"valid", false},
	)

	users = append(users, user1, user2)

	u := user{}
	b.SetBytes(2)
	for n := 0; n < b.N; n++ {
		users[0].ToStruct(&u)
	}

}
