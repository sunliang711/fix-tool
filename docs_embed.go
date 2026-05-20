package fixtool

import _ "embed"

//go:embed docs/project/fix-tool/user-guide.md
var userGuideMarkdown string

//go:embed docs/project/fix-tool/faq.md
var faqMarkdown string

func UserGuideMarkdown() string {
	return userGuideMarkdown
}

func FAQMarkdown() string {
	return faqMarkdown
}
