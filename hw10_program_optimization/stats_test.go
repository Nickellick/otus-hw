//go:build !bench
// +build !bench

package hw10programoptimization

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetDomainStat(t *testing.T) {
	data := `{"Id":1,"Name":"Howard Mendoza","Username":"0Oliver","Email":"aliquid_qui_ea@Browsedrive.gov","Phone":"6-866-899-36-79","Password":"InAQJvsq","Address":"Blackbird Place 25"}
{"Id":2,"Name":"Jesse Vasquez","Username":"qRichardson","Email":"mLynch@broWsecat.com","Phone":"9-373-949-64-00","Password":"SiZLeNSGn","Address":"Fulton Hill 80"}
{"Id":3,"Name":"Clarence Olson","Username":"RachelAdams","Email":"RoseSmith@Browsecat.com","Phone":"988-48-97","Password":"71kuz3gA5w","Address":"Monterey Park 39"}
{"Id":4,"Name":"Gregory Reid","Username":"tButler","Email":"5Moore@Teklist.net","Phone":"520-04-16","Password":"r639qLNu","Address":"Sunfield Park 20"}
{"Id":5,"Name":"Janice Rose","Username":"KeithHart","Email":"nulla@Linktype.com","Phone":"146-91-01","Password":"acSBF5","Address":"Russell Trail 61"}`

	t.Run("find 'com'", func(t *testing.T) {
		result, err := GetDomainStat(bytes.NewBufferString(data), "com")
		require.NoError(t, err)
		require.Equal(t, DomainStat{
			"browsecat.com": 2,
			"linktype.com":  1,
		}, result)
	})

	t.Run("find 'gov'", func(t *testing.T) {
		result, err := GetDomainStat(bytes.NewBufferString(data), "gov")
		require.NoError(t, err)
		require.Equal(t, DomainStat{"browsedrive.gov": 1}, result)
	})

	t.Run("find 'unknown'", func(t *testing.T) {
		result, err := GetDomainStat(bytes.NewBufferString(data), "unknown")
		require.NoError(t, err)
		require.Equal(t, DomainStat{}, result)
	})
}

func TestGetDomainStat_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		domain      string
		expected    DomainStat
		expectedErr bool
	}{
		{
			name: "case insensitive domains",
			input: `{"Email":"alice@Google.COM"}
{"Email":"bob@google.com"}
{"Email":"charlie@GOOGLE.Com"}`,
			domain: "COM",
			expected: DomainStat{
				"google.com": 3,
			},
		},
		{
			name: "email without at sign",
			input: `{"Email":"invalid-email"}
{"Email":"valid@example.com"}`,
			domain: "com",
			expected: DomainStat{
				"example.com": 1,
			},
		},
		{
			name: "domain must match suffix",
			input: `{"Email":"alice@example.biz.com"}
{"Email":"bob@example.biz"}`,
			domain: "biz",
			expected: DomainStat{
				"example.biz": 1,
			},
		},
		{
			name:        "invalid json",
			input:       `{"Email":`,
			domain:      "com",
			expectedErr: true,
		},
		{
			name:     "empty input",
			input:    "",
			domain:   "com",
			expected: DomainStat{},
		},
		{
			name:   "last line without newline",
			input:  `{"Email":"alice@example.com"}`,
			domain: "com",
			expected: DomainStat{
				"example.com": 1,
			},
		},
		{
			name:     "different top-level domain",
			input:    `{"Email":"alice@example.org"}`,
			domain:   "com",
			expected: DomainStat{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetDomainStat(
				strings.NewReader(tt.input),
				tt.domain,
			)

			if tt.expectedErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}
