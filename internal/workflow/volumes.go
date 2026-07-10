package workflow

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	"agent-flow/internal/prompts"
)

const defaultVolumeSize = 25
const defaultVolumeThreshold = 25

// VolumeRange describes one outline volume.
type VolumeRange struct {
	Num   int
	Start int
	End   int
}

// NovelSkeleton is the high-level multi-volume outline scaffold.
type NovelSkeleton struct {
	Title      string           `json:"title"`
	Synopsis   string           `json:"synopsis"`
	Characters []map[string]any `json:"characters"`
	Volumes    []VolumeMeta     `json:"volumes"`
}

// VolumeMeta describes one volume in the skeleton.
type VolumeMeta struct {
	Num          int    `json:"num"`
	Title        string `json:"title"`
	StartChapter int    `json:"startChapter"`
	EndChapter   int    `json:"endChapter"`
	Theme        string `json:"theme"`
	Summary      string `json:"summary"`
}

// VolumeOutline is detailed chapters for a single volume.
type VolumeOutline struct {
	Volume   int              `json:"volume"`
	Chapters []ChapterOutline `json:"chapters"`
}

// UseVolumeOutline returns true when chapter count warrants split outline generation.
func UseVolumeOutline(params map[string]string, chapterCount int) bool {
	if raw := strings.TrimSpace(params["useVolumes"]); raw != "" {
		return raw == "true" || raw == "1"
	}
	if raw := strings.TrimSpace(params["disableVolumes"]); raw == "true" || raw == "1" {
		return false
	}
	threshold := IntParam(params, "volumeThreshold", defaultVolumeThreshold)
	return chapterCount > threshold
}

// VolumeSize returns chapters per outline volume.
func VolumeSize(params map[string]string) int {
	return IntParam(params, "volumeSize", defaultVolumeSize)
}

// PlanVolumes splits total chapters into volume ranges.
func PlanVolumes(chapterCount, volumeSize int) []VolumeRange {
	if chapterCount <= 0 {
		return nil
	}
	if volumeSize <= 0 {
		volumeSize = defaultVolumeSize
	}
	var volumes []VolumeRange
	for start, num := 1, 1; start <= chapterCount; start, num = start+volumeSize, num+1 {
		end := start + volumeSize - 1
		if end > chapterCount {
			end = chapterCount
		}
		volumes = append(volumes, VolumeRange{Num: num, Start: start, End: end})
	}
	return volumes
}

// VolumeStepID returns workflow step id for a volume outline task.
func VolumeStepID(num int) string {
	return fmt.Sprintf("outline-vol-%02d", num)
}

// VolumeFileName returns workspace path for a volume outline artifact.
func VolumeFileName(num int) string {
	return fmt.Sprintf("volumes/volume-%02d.json", num)
}

// ParseSkeletonJSON extracts multi-volume scaffold from model output.
func ParseSkeletonJSON(raw string) (*NovelSkeleton, error) {
	raw = strings.TrimSpace(raw)
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end <= start {
		return nil, fmt.Errorf("skeleton json not found")
	}
	var skeleton NovelSkeleton
	if err := json.Unmarshal([]byte(raw[start:end+1]), &skeleton); err != nil {
		return nil, err
	}
	if len(skeleton.Volumes) == 0 {
		return nil, fmt.Errorf("skeleton has no volumes")
	}
	sort.Slice(skeleton.Volumes, func(i, j int) bool {
		return skeleton.Volumes[i].Num < skeleton.Volumes[j].Num
	})
	return &skeleton, nil
}

// ParseVolumeOutlineJSON extracts per-volume chapter outline.
func ParseVolumeOutlineJSON(raw string) (*VolumeOutline, error) {
	raw = strings.TrimSpace(raw)
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end <= start {
		return nil, fmt.Errorf("volume outline json not found")
	}
	var vol VolumeOutline
	if err := json.Unmarshal([]byte(raw[start:end+1]), &vol); err != nil {
		return nil, err
	}
	if len(vol.Chapters) == 0 {
		return nil, fmt.Errorf("volume outline has no chapters")
	}
	sort.Slice(vol.Chapters, func(i, j int) bool {
		return vol.Chapters[i].Num < vol.Chapters[j].Num
	})
	return &vol, nil
}

// MergeVolumeOutlines combines skeleton and volume files into outline.json.
func MergeVolumeOutlines(wf *agentflowiov1alpha1.Workflow) (string, error) {
	skeletonRaw, err := ReadArtifact(wf, "skeleton.json")
	if err != nil {
		return "", fmt.Errorf("read skeleton.json: %w", err)
	}
	skeleton, err := ParseSkeletonJSON(skeletonRaw)
	if err != nil {
		return "", err
	}

	var chapters []ChapterOutline
	for _, meta := range skeleton.Volumes {
		path := VolumeFileName(meta.Num)
		raw, err := ReadArtifact(wf, path)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", path, err)
		}
		if err := ValidateVolumeOutline(raw, meta.StartChapter, meta.EndChapter); err != nil {
			return "", fmt.Errorf("validate %s: %w", path, err)
		}
		vol, err := ParseVolumeOutlineJSON(raw)
		if err != nil {
			return "", fmt.Errorf("parse %s: %w", path, err)
		}
		chapters = append(chapters, vol.Chapters...)
	}
	if len(chapters) == 0 {
		return "", fmt.Errorf("no chapters merged from volume outlines")
	}
	sort.Slice(chapters, func(i, j int) bool {
		return chapters[i].Num < chapters[j].Num
	})

	outline := NovelOutline{
		Title:      skeleton.Title,
		Synopsis:   skeleton.Synopsis,
		Characters: skeleton.Characters,
		Chapters:   chapters,
	}
	data, err := json.MarshalIndent(outline, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// OutlineReady reports whether chapter generation can begin.
func OutlineReady(wf *agentflowiov1alpha1.Workflow) bool {
	for _, id := range wf.Status.CompletedSteps {
		if id == "outline" || id == "outline-merge" {
			return true
		}
	}
	return false
}

// BuildVolumeOutlineInstruction renders prompt for one volume's detailed outline.
func BuildVolumeOutlineInstruction(prompt string, skeleton *NovelSkeleton, vol VolumeMeta, prevVolumeSummary string) string {
	chars := ""
	if skeleton != nil {
		chars = FormatCharacters(&NovelOutline{Characters: skeleton.Characters})
	}
	title := ""
	syn := ""
	if skeleton != nil {
		title = skeleton.Title
		syn = skeleton.Synopsis
	}
	return prompts.BuildVolumeOutlineInstruction(
		prompt,
		title,
		syn,
		chars,
		vol.Num,
		vol.Title,
		vol.StartChapter,
		vol.EndChapter,
		vol.Theme,
		vol.Summary,
		prevVolumeSummary,
	)
}
