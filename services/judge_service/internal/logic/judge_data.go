package logic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"

	appErr "fuzoj/pkg/errors"
	"fuzoj/services/judge_service/internal/pmodel"
	"fuzoj/services/judge_service/internal/sandbox"
)

func (s *JudgeProcessor) downloadSource(ctx context.Context, payload pmodel.JudgeMessage) (string, error) {
	submissionDir := filepath.Join(s.workRoot, payload.SubmissionID, "source")
	if err := os.MkdirAll(submissionDir, 0755); err != nil {
		return "", appErr.Wrapf(err, appErr.JudgeSystemError, "create source dir failed")
	}
	filePath := filepath.Join(submissionDir, "source.code")
	ctxStorage := ctx
	if s.storageTimeout > 0 {
		var cancel context.CancelFunc
		ctxStorage, cancel = context.WithTimeout(ctx, s.storageTimeout)
		defer cancel()
	}
	reader, err := s.storage.GetObject(ctxStorage, s.sourceBucket, payload.SourceKey)
	if err != nil {
		return "", appErr.Wrapf(err, appErr.JudgeSystemError, "download source failed")
	}
	defer reader.Close()

	file, err := os.Create(filePath)
	if err != nil {
		return "", appErr.Wrapf(err, appErr.JudgeSystemError, "create source file failed")
	}
	defer file.Close()

	hasher := sha256.New()
	tee := io.TeeReader(reader, hasher)
	if _, err := io.Copy(file, tee); err != nil {
		return "", appErr.Wrapf(err, appErr.JudgeSystemError, "write source file failed")
	}
	if payload.SourceHash != "" {
		actual := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(actual, payload.SourceHash) {
			return "", appErr.New(appErr.InvalidParams).WithMessage("source hash mismatch")
		}
	}
	return filePath, nil
}

func resolveLanguageConfig(cfg pmodel.ProblemConfig, languageID string) ([]string, pmodel.ResourceLimit) {
	base := cfg.DefaultLimits
	var extra []string
	for _, lim := range cfg.LanguageLimits {
		if lim.LanguageID == languageID {
			if lim.Limits != nil {
				base = pmodel.MergeLimits(lim.Limits, base)
			}
			extra = append(extra, lim.ExtraCompileFlags...)
			break
		}
	}
	return extra, base
}

func buildTestcases(manifest pmodel.Manifest, basePath string, defaults pmodel.ResourceLimit) ([]sandbox.TestcaseSpec, []sandbox.SubtaskSpec, error) {
	ioCfg := sandbox.IOConfig{
		Mode:           manifest.IOConfig.Mode,
		InputFileName:  manifest.IOConfig.InputFileName,
		OutputFileName: manifest.IOConfig.OutputFileName,
	}

	tests := make([]sandbox.TestcaseSpec, 0, len(manifest.Tests))
	for _, tc := range manifest.Tests {
		inputPath, err := safeJoin(basePath, tc.InputPath)
		if err != nil {
			return nil, nil, err
		}
		answerPath := ""
		if tc.AnswerPath != "" {
			answerPath, err = safeJoin(basePath, tc.AnswerPath)
			if err != nil {
				return nil, nil, err
			}
		}
		limits := pmodel.MergeLimits(tc.Limits, defaults)
		checker := tc.Checker
		if checker == nil {
			checker = manifest.Checker
		}
		var checkerSpec *sandbox.CheckerSpec
		if checker != nil {
			checkerPath, err := safeJoin(basePath, checker.BinaryPath)
			if err != nil {
				return nil, nil, err
			}
			checkerSpec = &sandbox.CheckerSpec{
				BinaryPath: checkerPath,
				Args:       checker.Args,
				Env:        checker.Env,
				Limits:     pmodel.ToSandboxLimit(pmodel.MergeLimits(checker.Limits, defaults)),
			}
		}

		tests = append(tests, sandbox.TestcaseSpec{
			TestID:            tc.TestID,
			InputPath:         inputPath,
			AnswerPath:        answerPath,
			IOConfig:          ioCfg,
			Score:             tc.Score,
			SubtaskID:         tc.SubtaskID,
			Limits:            pmodel.ToSandboxLimit(limits),
			Checker:           checkerSpec,
			CheckerLanguageID: tc.CheckerLanguageID,
		})
	}

	subtasks := make([]sandbox.SubtaskSpec, 0, len(manifest.Subtasks))
	for _, st := range manifest.Subtasks {
		subtasks = append(subtasks, sandbox.SubtaskSpec{
			ID:         st.ID,
			Score:      st.Score,
			Strategy:   st.Strategy,
			StopOnFail: st.StopOnFail,
		})
	}
	return tests, subtasks, nil
}

func safeJoin(basePath, relPath string) (string, error) {
	if relPath == "" {
		return "", appErr.ValidationError("path", "required")
	}
	clean := filepath.Clean(relPath)
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return "", appErr.New(appErr.InvalidParams).WithMessage("invalid relative path")
	}
	full := filepath.Join(basePath, clean)
	if !strings.HasPrefix(full, filepath.Clean(basePath)+string(filepath.Separator)) {
		return "", appErr.New(appErr.InvalidParams).WithMessage("path traversal detected")
	}
	return full, nil
}
