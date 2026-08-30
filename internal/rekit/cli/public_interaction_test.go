package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestReducePublicStatusInteractionUsesOnlyTypedInteractionInput(t *testing.T) {
	for _, test := range []struct {
		name  string
		input publicStatusInteractionInput
		want  []string
	}{
		{
			name: "fresh onboarding",
			input: publicStatusInteractionInput{
				Mode:        "case-onboarding-required",
				CaseMission: &publicStatusMissionInput{Summary: "等待首次接入"},
				Onboarding:  &publicStatusOnboardingInput{State: "absent"},
			},
			want: []string{"等待首次接入", "尚未完成首次接入", "选择一个 pack"},
		},
		{
			name: "ready",
			input: publicStatusInteractionInput{
				Summary: "工作线已读取",
				MissionControlRunbook: &publicStatusRunbookInput{CurrentDriverRequest: &publicStatusDriverInput{
					State:             "ready-to-continue",
					CommandExecutable: true,
				}},
			},
			want: []string{"工作线已读取", "可以继续推进", "本次状态中给出的步骤"},
		},
		{
			name: "blocked",
			input: publicStatusInteractionInput{
				MissionControlRunbook: &publicStatusRunbookInput{CurrentDriverRequest: &publicStatusDriverInput{
					State:   "ready-to-continue",
					Blocked: true,
				}},
			},
			want: []string{"状态已读取", "需要先处理当前阻塞", "不要自行拼接内部参数"},
		},
		{
			name: "details required",
			input: publicStatusInteractionInput{
				State:           "details-required",
				DetailsRequired: true,
				Reason:          "compact-output-budget-exceeded",
			},
			want: []string{"完整情况暂时无法", "当前下一步无法", "完整状态", "不要猜测"},
		},
		{
			name: "mission complete",
			input: publicStatusInteractionInput{
				Summary: "全部工作线已完成",
				CaseMission: &publicStatusMissionInput{
					MissionCompletion: &publicMissionCompletionInput{
						State:                 "mission-complete",
						OperationallyComplete: true,
					},
				},
			},
			want: []string{"全部工作线已完成", "当前任务已完成", "补充或纠正", "新的独立目标"},
		},
		{
			name:  "no current",
			input: publicStatusInteractionInput{Summary: "空闲"},
			want:  []string{"空闲", "没有可以直接执行的下一步", "告诉主 Agent 你的目标"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			interaction := reducePublicStatusInteraction(test.input)
			text := interaction.Now + " " + interaction.Reason + " " + interaction.Next
			for _, want := range test.want {
				if !strings.Contains(text, want) {
					t.Fatalf("interaction=%+v, want containing %q", interaction, want)
				}
			}
		})
	}
}

func TestRunPublicSupervisorSeparatesInteractionAndDiagnostics(t *testing.T) {
	const generation = "9007199254740993"
	raw := json.RawMessage(`{
		"mode":"case",
		"caseMission":{"summary":"ready=1 blocked=0"},
		"missionControlRunbook":{"currentDriverRequest":{
			"state":"ready-to-continue",
			"commandExecutable":true,
			"command":"/steamai continue -Lane internal -WhatIf -Format json"
		}},
		"currentDriverRequestSha256":"` + strings.Repeat("a", 64) + `",
		"generation":` + generation + `,
		"path":"C:\\private\\case"
	}`)
	var calls []publicOptions
	executor := func(options publicOptions, projectRoot string, recoveryOnly bool) (json.RawMessage, error) {
		if projectRoot != "project-root" || !recoveryOnly {
			t.Fatalf("executor binding drifted: root=%q recovery=%t", projectRoot, recoveryOnly)
		}
		calls = append(calls, options)
		return append(json.RawMessage(nil), raw...), nil
	}

	var interaction bytes.Buffer
	if err := runPublicWithExecutor(
		[]string{"status"},
		&interaction,
		"project-root",
		true,
		executor,
	); err != nil {
		t.Fatal(err)
	}
	text := interaction.String()
	for _, want := range []string{"现在：ready=1 blocked=0", "可以继续推进", "本次状态中给出的步骤"} {
		if !strings.Contains(text, want) {
			t.Fatalf("interaction output missing %q: %s", want, text)
		}
	}
	for _, forbidden := range []string{"-Lane", "currentDriverRequestSha256", strings.Repeat("a", 64), generation, `C:\private`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("interaction output leaked %q: %s", forbidden, text)
		}
	}

	var diagnostics bytes.Buffer
	if err := runPublicWithExecutor(
		[]string{"status", "--diagnostics"},
		&diagnostics,
		"project-root",
		true,
		executor,
	); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || calls[0].Mode != publicOutputInteraction || calls[1].Mode != publicOutputDiagnostics {
		t.Fatalf("supervisor mode routing drifted: %+v", calls)
	}
	diagnosticText := diagnostics.String()
	for _, want := range []string{generation, `"currentDriverRequestSha256"`, `"path": "C:\\private\\case"`} {
		if !strings.Contains(diagnosticText, want) {
			t.Fatalf("diagnostics output lost %q: %s", want, diagnosticText)
		}
	}
	if strings.Contains(diagnosticText, "9007199254740992") {
		t.Fatalf("diagnostics rounded typed generation: %s", diagnosticText)
	}
}

func TestWritePublicInteractionFailsClosedOnTypedFieldMismatch(t *testing.T) {
	var out bytes.Buffer
	err := writePublicInteraction(
		&out,
		"status",
		json.RawMessage(`{"missionControlRunbook":{"currentDriverRequest":{"blocked":"yes"}}}`),
	)
	if err == nil || !strings.Contains(err.Error(), "decode public status interaction") {
		t.Fatalf("typed field mismatch error=%v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("typed field mismatch published partial interaction: %q", out.String())
	}
}
