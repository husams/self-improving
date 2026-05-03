package skills

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillbench/internal/model"
)

func Load(path string) (model.SkillInfo, error) {
	if path == "" {
		return model.SkillInfo{}, fmt.Errorf("--skill is required")
	}
	info, err := os.Stat(path)
	if err != nil {
		return model.SkillInfo{}, err
	}
	skillPath := path
	root := path
	if info.IsDir() {
		skillPath = filepath.Join(path, "SKILL.md")
	} else {
		root = filepath.Dir(path)
	}
	b, err := os.ReadFile(skillPath)
	if err != nil {
		return model.SkillInfo{}, err
	}
	name := parseName(string(b))
	if name == "" {
		name = model.SkillNameFromPath(skillPath)
	}
	return model.SkillInfo{
		Name:        name,
		Path:        root,
		SKILLPath:   skillPath,
		Fingerprint: model.Fingerprint(string(b)),
		Content:     string(b),
	}, nil
}

func ExposureFor(agent model.Agent, info model.SkillInfo) model.Exposure {
	home, _ := os.UserHomeDir()
	clean := filepath.Clean(info.Path)
	switch agent {
	case model.AgentClaude:
		if strings.HasPrefix(clean, filepath.Join(home, ".claude", "skills")) || strings.Contains(clean, string(filepath.Separator)+".claude"+string(filepath.Separator)+"skills"+string(filepath.Separator)) {
			return model.ExposureNative
		}
	case model.AgentCodex:
		if strings.HasPrefix(clean, filepath.Join(home, ".codex", "skills")) || strings.Contains(clean, string(filepath.Separator)+".codex"+string(filepath.Separator)+"skills"+string(filepath.Separator)) {
			return model.ExposureNative
		}
	}
	return model.ExposureRenderedFallback
}

func RenderPrompt(info model.SkillInfo, prompt string) string {
	return fmt.Sprintf(`Use the following Agent Skill instructions for this test run.

<skill name="%s" fingerprint="%s">
%s
</skill>

User prompt:
%s`, info.Name, info.Fingerprint, info.Content, prompt)
}

func parseName(content string) string {
	sc := bufio.NewScanner(strings.NewReader(content))
	inFrontmatter := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			}
			break
		}
		if inFrontmatter && strings.HasPrefix(line, "name:") {
			return strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "name:")), `"'`)
		}
	}
	return ""
}
