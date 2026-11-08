// db-sandbox isolates integration runs from any existing installation schema.
package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/jackc/pgx/v5"
	"net/url"
	"os"
	"regexp"
	"time"
)

func main() {
	create := flag.String("create", "", "isolated test schema")
	drop := flag.String("drop", "", "remove only specified isolated test schema")
	flag.Parse()
	name := *create
	if name == "" {
		name = *drop
	}
	if !regexp.MustCompile("^v02_[a-f0-9]{16}$").MatchString(name) {
		panic("invalid isolated test schema")
	}
	u, e := url.Parse(os.Getenv("TEST_DATABASE_URL"))
	must(e)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, e := pgx.Connect(ctx, u.String())
	must(e)
	defer conn.Close(ctx)
	identifier := pgx.Identifier{name}.Sanitize()
	if *create != "" {
		_, e = conn.Exec(ctx, "CREATE SCHEMA "+identifier)
		must(e)
		q := u.Query()
		q.Set("search_path", name)
		u.RawQuery = q.Encode()
		fmt.Print(u.String())
	} else {
		_, e = conn.Exec(ctx, "DROP SCHEMA "+identifier+" CASCADE")
		must(e)
	}
}
func must(e error) {
	if e != nil {
		panic(e)
	}
}
