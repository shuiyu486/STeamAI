package steamai

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// 两档模型偏好只保存 Claude Code 模型别名；不记录厂商模型 ID、凭据或其它字段。
// 语义由用户指定：high/standard 允许相同别名，Go 层只做机械校验与映射。
type modelTiers struct {
	High     string `json:"high"`
	Standard string `json:"standard"`
}

var defaultModelTiers = modelTiers{High: "opus", Standard: "sonnet"}

func validModelAlias(alias string) bool {
	switch alias {
	case "opus", "sonnet", "haiku":
		return true
	default:
		return false
	}
}

// configuredModelTiers 读取固定位置的 models.json；文件缺失时返回默认档位且不写入，
// 文件或目录畸形时返回错误，绝不悄悄回退默认值。
func (a *app) configuredModelTiers() (modelTiers, error) {
	path, err := modelsConfigPath()
	if err != nil {
		return modelTiers{}, err
	}
	return readModelTiers(path)
}

// modelsCommand 支持两种形态：
//
//	steamai models                                        只读输出当前档位
//	steamai models --high A --standard B                  两个参数必须一次完整提供
func (a *app) modelsCommand(args []string) error {
	var desired modelTiers
	if len(args) != 0 {
		var err error
		desired, err = parseModelsArgs(args)
		if err != nil {
			return err
		}
	}
	path, err := modelsConfigPath()
	if err != nil {
		return err
	}
	current, err := readModelTiers(path)
	if err != nil {
		return err
	}
	if len(args) != 0 && desired != current {
		if err := writeModelTiers(path, desired); err != nil {
			return err
		}
		current = desired
	}
	_, err = fmt.Fprintf(a.stdout, "high=%s\nstandard=%s\nconfig=%s\n", current.High, current.Standard, path)
	return err
}

const modelsUsage = "用法：steamai models [--high opus|sonnet|haiku --standard opus|sonnet|haiku]（设置时两个参数必须同时提供）"

// parseModelsArgs 接受任意顺序的完整 --high/--standard 对；拒绝重复、缺值、
// 未知参数、多余位置参数和非法别名，任何失败都不触发写入。
func parseModelsArgs(args []string) (modelTiers, error) {
	var tiers modelTiers
	seenHigh, seenStandard := false, false
	for i := 0; i < len(args); i++ {
		label := args[i]
		var target *string
		var seen *bool
		switch label {
		case "--high":
			target, seen = &tiers.High, &seenHigh
		case "--standard":
			target, seen = &tiers.Standard, &seenStandard
		default:
			return modelTiers{}, fmt.Errorf("未知 models 参数 %q；%s", label, modelsUsage)
		}
		if *seen {
			return modelTiers{}, fmt.Errorf("%s 只能出现一次；%s", label, modelsUsage)
		}
		if i+1 == len(args) {
			return modelTiers{}, fmt.Errorf("%s 需要模型别名；%s", label, modelsUsage)
		}
		i++
		alias := args[i]
		if !validModelAlias(alias) {
			return modelTiers{}, fmt.Errorf("%s 的模型别名 %q 无效，只允许 opus、sonnet、haiku；%s", label, alias, modelsUsage)
		}
		*target = alias
		*seen = true
	}
	if !seenHigh || !seenStandard {
		return modelTiers{}, fmt.Errorf("--high 与 --standard 必须同时提供；%s", modelsUsage)
	}
	return tiers, nil
}

// modelsConfigPath 固定解析 %LOCALAPPDATA%\STeamAI\models.json；环境目录必须存在且为绝对路径。
func modelsConfigPath() (string, error) {
	root := os.Getenv("LOCALAPPDATA")
	if root == "" {
		return "", errors.New("LOCALAPPDATA 不可用")
	}
	if !filepath.IsAbs(root) {
		return "", errors.New("LOCALAPPDATA 必须是绝对目录")
	}
	if err := requirePlainDirectory(root); err != nil {
		return "", fmt.Errorf("LOCALAPPDATA 不是可用的普通目录: %w", err)
	}
	return filepath.Join(root, "STeamAI", "models.json"), nil
}

func readModelTiers(path string) (modelTiers, error) {
	if err := requirePlainDirectory(filepath.Dir(path)); err != nil {
		if os.IsNotExist(err) {
			return defaultModelTiers, nil
		}
		return modelTiers{}, fmt.Errorf("模型配置目录无效: %w", err)
	}
	if err := requirePlainFile(path); err != nil {
		if os.IsNotExist(err) {
			return defaultModelTiers, nil
		}
		return modelTiers{}, fmt.Errorf("模型配置文件无效: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return modelTiers{}, err
	}
	tiers, err := parseModelTiers(data)
	if err != nil {
		return modelTiers{}, fmt.Errorf("配置 %s 无效（请修复为只含 high/standard 的 JSON 或移走该文件恢复默认）: %w", path, err)
	}
	return tiers, nil
}

func parseModelTiers(data []byte) (modelTiers, error) {
	if !json.Valid(data) {
		return modelTiers{}, errors.New("不是单个有效 JSON 对象")
	}
	fields, err := decodeStrictObject(data)
	if err != nil {
		return modelTiers{}, err
	}
	var tiers modelTiers
	if len(fields) != 2 || json.Unmarshal(fields["high"], &tiers.High) != nil || json.Unmarshal(fields["standard"], &tiers.Standard) != nil ||
		!validModelAlias(tiers.High) || !validModelAlias(tiers.Standard) {
		return modelTiers{}, errors.New("应包含 high 和 standard 两个字段，值各选 opus、sonnet 或 haiku")
	}
	return tiers, nil
}

// writeModelTiers 在同目录写临时文件并同步后用 rename 原子替换：
// models.json 任意时刻要么是完整旧内容、要么是完整新内容，失败时原件不动。
func writeModelTiers(path string, tiers modelTiers) error {
	data, err := json.Marshal(tiers)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := requirePlainDirectory(dir); err != nil {
		return fmt.Errorf("模型配置目录无效 %s: %w", dir, err)
	}
	if exists, err := pathExists(path); err != nil {
		return err
	} else if exists {
		if err := requirePlainFile(path); err != nil {
			return fmt.Errorf("模型配置文件无效 %s: %w", path, err)
		}
	}
	tmp, err := os.CreateTemp(dir, ".models.json.tmp-*")
	if err != nil {
		return fmt.Errorf("创建临时文件: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("写入临时文件: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("同步临时文件: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("原子更新 %s: %w", path, err)
	}
	return nil
}
