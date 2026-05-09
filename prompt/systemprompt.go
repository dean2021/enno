package prompt

import "strings"

type Section struct {
	Name    string
	Content string
}

type RuntimeConfig struct {
	SkillsSummary string
}

func RuntimeSections(config RuntimeConfig) []Section {
	return filterEmptySections([]Section{
		SkillsSection(config.SkillsSummary),
	})
}

func Join(base string, sections []Section) string {
	var parts []string
	if strings.TrimSpace(base) != "" {
		parts = append(parts, strings.TrimSpace(base))
	}
	for _, section := range sections {
		text := section.String()
		if text == "" {
			continue
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, "\n\n")
}

func (s Section) String() string {
	content := strings.TrimSpace(s.Content)
	if content == "" {
		return ""
	}
	name := strings.TrimSpace(s.Name)
	if name == "" {
		return content
	}
	return "# " + name + "\n" + content
}

func SkillsSection(summary string) Section {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return Section{}
	}
	return Section{
		Name:    "Skills",
		Content: "Skills available:\n" + summary + "\nCall load_skill with a skill name when you need the full instructions for that workflow.",
	}
}

func filterEmptySections(sections []Section) []Section {
	filtered := make([]Section, 0, len(sections))
	for _, section := range sections {
		if strings.TrimSpace(section.Content) == "" {
			continue
		}
		filtered = append(filtered, section)
	}
	return filtered
}
