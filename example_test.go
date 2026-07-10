package ognl_test

import (
	"fmt"

	ognl "github.com/golang-infrastructure/go-ognl"
)

func Example() {
	type User struct {
		Name  string
		Roles []string
	}

	user := User{Name: "alice", Roles: []string{"admin", "maintainer"}}
	fmt.Println(ognl.Get(user, "Name").Value())

	parsed := ognl.Parse(user)
	fmt.Println(parsed.Get("Roles.1").Value())

	users := []User{{Name: "alice"}, {Name: "bob"}}
	fmt.Println(ognl.Get(users, "#.Name").Values())

	// Output:
	// alice
	// maintainer
	// [alice bob]
}
