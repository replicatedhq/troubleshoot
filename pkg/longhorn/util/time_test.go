package util

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParsePeriod(t *testing.T) {
	assert := require.New(t)

	loc := time.UTC
	now := time.Now().UTC()
	p, err := ParsePeriod("", loc)
	assert.Nil(err)
	assert.True(p[1].UnixNano() >= now.UnixNano())

	s1 := "2007-03-01T13:00:00/2008-05-11T15:30:00"
	//s2 := "2007-03-01T13:00:00/P1Y2M10DT2H30M"
	s3 := "2008-05-11T15:30:00"

	p1, err := ParsePeriod(s1, loc)
	assert.Nil(err)
	assert.Equal("2007-03-01T13:00:00Z", FormatTimeZ(p1[0]))
	assert.Equal("2008-05-11T15:30:00Z", FormatTimeZ(p1[1]))

	//p2, err := ParsePeriod(s2)
	//assert.Nil(err)
	//assert.Equal("2007-03-01T13:00:00Z", FormatTimeZ(p2[0]))
	//assert.Equal("2008-05-11T15:30:00Z", FormatTimeZ(p2[1]))

	p3, err := ParsePeriod(s3, loc)
	assert.Nil(err)
	assert.Equal("2008-05-11T15:30:00Z", FormatTimeZ(p3[0]))
	assert.True(p3[1].UnixNano() >= now.UnixNano())
}

func TestParseTime(t *testing.T) {
	assert := require.New(t)

	s1 := "2016-07-19T01:21:09"
	ls := "Asia/Yekaterinburg"
	loc, err := time.LoadLocation(ls)
	assert.Nil(err)

	t1, err := ParseLocalTime(s1, loc)
	assert.Nil(err)
	assert.Equal("2016-07-18T20:21:09Z", FormatTimeZ(t1))

	t2, err := ParseLocalTime(s1+"+05:00", loc)
	assert.Nil(err)
	assert.Equal("2016-07-18T20:21:09Z", FormatTimeZ(t2))

	s3 := "2015-03-01T13:00:00Z"
	t3, err := ParseTimeZ(s3)
	assert.Nil(err)

	assert.Equal("2015-03-01T13:00:00Z", FormatTimeZ(t3))
}
