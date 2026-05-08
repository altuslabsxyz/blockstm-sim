package compare_test

import (
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/compare"
)

func TestFormatEvents_Empty(t *testing.T) {
	require.Equal(t, "(none)", compare.FormatEvents(nil))
	require.Equal(t, "(none)", compare.FormatEvents([]abci.Event{}))
}

func TestFormatEvents_SingleEvent(t *testing.T) {
	events := []abci.Event{
		{
			Type: "transfer",
			Attributes: []abci.EventAttribute{
				{Key: "recipient", Value: "addr1"},
				{Key: "amount", Value: "100uatom"},
			},
		},
	}
	require.Equal(t, "transfer:[recipient=addr1,amount=100uatom]", compare.FormatEvents(events))
}

func TestFormatEvents_MultipleEvents(t *testing.T) {
	events := []abci.Event{
		{
			Type: "transfer",
			Attributes: []abci.EventAttribute{
				{Key: "recipient", Value: "addr1"},
				{Key: "amount", Value: "100uatom"},
			},
		},
		{
			Type:       "message",
			Attributes: []abci.EventAttribute{{Key: "action", Value: "/cosmos.bank.v1beta1.MsgSend"}},
		},
	}
	require.Equal(t,
		"transfer:[recipient=addr1,amount=100uatom];message:[action=/cosmos.bank.v1beta1.MsgSend]",
		compare.FormatEvents(events),
	)
}

// TestFormatEvents_PreservesAttributeOrder verifies that attributes are NOT sorted.
// If "z" comes before "a" in the input, the output must reflect that.
func TestFormatEvents_PreservesAttributeOrder(t *testing.T) {
	events := []abci.Event{
		{
			Type: "test",
			Attributes: []abci.EventAttribute{
				{Key: "z", Value: "last"},
				{Key: "a", Value: "first"},
			},
		},
	}
	require.Equal(t, "test:[z=last,a=first]", compare.FormatEvents(events))
}

// TestFormatEvents_PreservesEventOrder verifies that events are NOT reordered.
func TestFormatEvents_PreservesEventOrder(t *testing.T) {
	events := []abci.Event{
		{Type: "second", Attributes: []abci.EventAttribute{{Key: "k", Value: "v2"}}},
		{Type: "first", Attributes: []abci.EventAttribute{{Key: "k", Value: "v1"}}},
	}
	result := compare.FormatEvents(events)
	require.Equal(t, "second:[k=v2];first:[k=v1]", result)
}
