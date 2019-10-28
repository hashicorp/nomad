package sensitive

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
	"github.com/ugorji/go/codec"
)

func TestSensitive_Plaintext(t *testing.T) {
	s := Sensitive("super-secret-value")
	require.Equal(t, "super-secret-value", s.Plaintext())
	require.Equal(t, "[REDACTED]", s.String())
}

func TestSensitive_Sprintf(t *testing.T) {
	s := Sensitive("super-secret-value")

	v := fmt.Sprintf("VALUE IS %v %s", s, s)
	require.Equal(t, "VALUE IS [REDACTED] [REDACTED]", v)
	require.NotContains(t, v, s.Plaintext())
}

func TestSensitive_JSON_Marshal(t *testing.T) {
	s := Sensitive("super-secret-value")

	b, err := json.Marshal([]interface{}{s})
	require.NoError(t, err)
	require.Equal(t, `["[REDACTED]"]`, string(b))
	require.NotContains(t, string(b), s.Plaintext())
}

func TestSensitive_JSON_Unmarshal(t *testing.T) {
	var val struct {
		P string
		S Sensitive
	}

	err := json.Unmarshal([]byte(`{"P": "value", "S": "sensitive"}`), &val)
	require.NoError(t, err)
	require.Equal(t, "value", val.P)
	require.Equal(t, "sensitive", val.S.Plaintext())
}

func TestSensitive_HCLog_Text(t *testing.T) {
	s := Sensitive("super-secret-value")

	var buf bytes.Buffer
	logger := hclog.New(&hclog.LoggerOptions{
		Output: &buf,
	})

	logger.Info("my log line", "value", s)

	require.Contains(t, buf.String(), `my log line: value=[REDACTED]`)
	require.NotContains(t, buf.String(), s.Plaintext())
}

func TestSensitive_HCLog_Json(t *testing.T) {
	s := Sensitive("super-secret-value")

	var buf bytes.Buffer
	logger := hclog.New(&hclog.LoggerOptions{
		JSONFormat: true,
		Output:     &buf,
	})

	logger.Info("my log line", "value", s)

	require.Contains(t, buf.String(), `"value":"[REDACTED]"`)
	require.NotContains(t, buf.String(), s.Plaintext())
}

func TestSensitive_UgorjiGo_Json(t *testing.T) {
	s := Sensitive("super-secret-value")

	h := &codec.JsonHandle{HTMLCharsAsIs: true}

	var buf bytes.Buffer
	enc := codec.NewEncoder(&buf, h)
	require.NoError(t, enc.Encode(s))

	var out Sensitive
	dec := codec.NewDecoder(&buf, h)
	require.NoError(t, dec.Decode(&out))

	require.Equal(t, out, s)
	require.Equal(t, "super-secret-value", out.Plaintext())
}

func TestSensitive_UgorjiGo_MsgPack(t *testing.T) {
	s := Sensitive("super-secret-value")

	h := &codec.MsgpackHandle{}

	var buf bytes.Buffer
	enc := codec.NewEncoder(&buf, h)
	require.NoError(t, enc.Encode(s))

	var out Sensitive
	dec := codec.NewDecoder(&buf, h)
	require.NoError(t, dec.Decode(&out))

	require.Equal(t, out, s)
	require.Equal(t, "super-secret-value", out.Plaintext())
}
