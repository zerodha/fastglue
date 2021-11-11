package fastglue

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

type callLog struct {
	CallFrom             string         `json:"CallFrom"`
	CallTo               string         `json:"CallTo"`
	CallSid              string         `json:"CallSid"`
	FlowID               int            `json:"flow_id"`
	CallType             string         `json:"CallType"`
	Direction            string         `json:"call_direction"`
	TenantID             int            `json:"tenant_id"`
	RecordingURL         string         `json:"RecordingUrl"`
	ForwardedFrom        string         `json:"ForwardedFrom"`
	Legs                 map[string]leg `json:"Legs"`
	Insights             insight        `json:"Insights"`
	DialCallStatus       string         `json:"DialCallStatus"`
	DialWhomNumber       string         `json:"DialWhomNumber"`
	DialCallDuration     int            `json:"DialCallDuration"`
	RecordingAvailableBy string         `json:"RecordingAvailableBy"`
	CallStatus           string         `json:"CallStatus"`
	Tag                  string         `json:"tag"`
	Type                 string         `json:"type"`
	AgentEmail           string         `json:"agent_email"`
	rawURL               string
	Digits               string `json:"digits"`
	Ref                  string
}

type leg struct {
	Type           string  `json:"Type"`
	Cause          string  `json:"Cause"`
	Number         string  `json:"Number"`
	CauseCode      string  `json:"CauseCode"`
	OnCallDuration int     `json:"OnCallDuration"`
	DialedNumber   string  `json:"dialed_number"`
	DisconnectedBy string  `json:"DisconnectedBy"`
	Insights       insight `json:"Insights"`
}
type insight struct {
	DisconnectedBy  string  `json:"DisconnectedBy"`
	DetailedStatus  string  `json:"DetailedStatus"`
	RingingDuration float64 `json:"RingingDuration"`
	DialCallStatus  string  `json:"DialCallStatus"`
}

func TestUnmarshalArgs(t *testing.T) {
	args := fasthttp.AcquireArgs()
	args.Parse(`Legs[1][Insights][Status]=completed&Legs[1][Insights][DetailedStatus]=CALL_COMPLETED&Legs[1][Insights][RingingDuration]=4.39&Insights[DialCallStatus]=completed`)
	var o callLog
	err := UnmarshalArgs(args, &o)
	require.NoError(t, err)
}
