package collect

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_parsePostgresVersion(t *testing.T) {
	tests := []struct {
		postgresVersion string
		expect          string
	}{
		{
			//  docker run -d --name pgnine -e POSTGRES_PASSWORD=password postgres:9
			postgresVersion: "PostgreSQL 9.6.17 on x86_64-pc-linux-gnu (Debian 9.6.17-2.pgdg90+1), compiled by gcc (Debian 6.3.0-18+deb9u1) 6.3.0 20170516, 64-bit",
			expect:          "9.6.17",
		},
		{
			//  docker run -d --name pgten -e POSTGRES_PASSWORD=password postgres:10
			postgresVersion: "PostgreSQL 10.12 (Debian 10.12-2.pgdg90+1) on x86_64-pc-linux-gnu, compiled by gcc (Debian 6.3.0-18+deb9u1) 6.3.0 20170516, 64-bit",
			expect:          "10.12",
		},
		{
			//  docker run -d --name pgeleven -e POSTGRES_PASSWORD=password postgres:11
			postgresVersion: "PostgreSQL 11.7 (Debian 11.7-2.pgdg90+1) on x86_64-pc-linux-gnu, compiled by gcc (Debian 6.3.0-18+deb9u1) 6.3.0 20170516, 64-bit",
			expect:          "11.7",
		},
		{
			// docker run -d --name pgtwelve -e POSTGRES_PASSWORD=password postgres:12
			postgresVersion: "PostgreSQL 12.2 (Debian 12.2-2.pgdg100+1) on x86_64-pc-linux-gnu, compiled by gcc (Debian 8.3.0-6) 8.3.0, 64-bit",
			expect:          "12.2",
		},
	}
	for _, test := range tests {
		t.Run(test.postgresVersion, func(t *testing.T) {
			req := require.New(t)
			actual, err := parsePostgresVersion(test.postgresVersion)
			req.NoError(err)

			assert.Equal(t, test.expect, actual)

		})
	}
}
