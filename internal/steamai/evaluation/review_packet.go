package evaluation

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

type reviewPacketSource struct {
	record RunRecord
	output []byte
}

func encodeBlindReviewPacket(runID string, arms []ArmRecord, sources map[string]reviewPacketSource) (BlindReviewPacket, []byte, error) {
	if !idPattern.MatchString(runID) || len(arms) != 2 || len(sources) != len(arms) {
		return BlindReviewPacket{}, nil, ErrInvalid
	}
	ordered := append([]ArmRecord(nil), arms...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Label < ordered[j].Label })
	packet := BlindReviewPacket{SchemaVersion: 1, RunID: runID}
	for index, arm := range ordered {
		source, present := sources[arm.Label]
		if !present || source.record.ArmLabel != arm.Label || source.record.Result.OutputSHA256 != arm.OutputSHA256 ||
			Hash(source.output) != arm.OutputSHA256 {
			return BlindReviewPacket{}, nil, ErrInvalid
		}
		entry, err := blindReviewEntry(fmt.Sprintf("entry-%d", index), arm, source.record, source.output)
		if err != nil {
			return BlindReviewPacket{}, nil, err
		}
		packet.Entries = append(packet.Entries, entry)
	}
	data, err := json.MarshalIndent(packet, "", "  ")
	if err != nil {
		return BlindReviewPacket{}, nil, errors.New("编码 blind review packet 失败")
	}
	return packet, append(data, '\n'), nil
}

func blindReviewEntry(entryID string, arm ArmRecord, record RunRecord, output []byte) (BlindReviewEntry, error) {
	entry := BlindReviewEntry{
		Entry:        entryID,
		ArmLabel:     arm.Label,
		OutputSHA256: arm.OutputSHA256,
		Status:       record.Result.Status,
		SafetyGate:   record.Result.SafetyGate,
	}
	if record.Result.Status != "completed" {
		return entry, nil
	}
	envelope, _, err := decodeModelOutput(output)
	if err != nil {
		return BlindReviewEntry{}, ErrInvalid
	}
	entry.Answer = &BlindReviewAnswer{
		Summary:     envelope.StructuredOutput.Summary,
		Evidence:    envelope.StructuredOutput.Evidence,
		Limitations: envelope.StructuredOutput.Limitations,
	}
	return entry, nil
}
