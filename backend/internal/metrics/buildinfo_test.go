package metrics

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

func TestNewBuildInfo(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	NewBuildInfo(reg, "1.0.0")

	expected := `
# HELP urlshort_build_info Build version of the running binary; the value is always 1.
# TYPE urlshort_build_info gauge
urlshort_build_info{version="1.0.0"} 1
`
	require.NoError(t, testutil.GatherAndCompare(reg, strings.NewReader(expected), "urlshort_build_info"))
}
