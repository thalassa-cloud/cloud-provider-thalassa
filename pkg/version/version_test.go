package version

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInit(t *testing.T) {
	version := "1.0.0"
	commit := "abcd1234"
	date := "2022-01-01"
	builtBy := "Goreleaser"

	Init(version, commit, date, builtBy)

	require.Equal(t, version, Version(), "Version mismatch")
	require.Equal(t, commit, Commit(), "Commit mismatch")
	require.Equal(t, date, BuildDate(), "Build date mismatch")
	require.Equal(t, builtBy, BuiltBy(), "Built by mismatch")
}
