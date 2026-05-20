package cli

import (
	"fmt"
	"io"
	"strings"

	fixtool "fix-tool"

	"github.com/spf13/cobra"
)

type docsTopic struct {
	name        string
	description string
	content     func() string
}

var docsTopics = []docsTopic{
	{
		name:        "user-guide",
		description: "User guide with installation, configuration, order commands, shell, scenario, and raw message usage",
		content:     fixtool.UserGuideMarkdown,
	},
	{
		name:        "faq",
		description: "FAQ and troubleshooting guide",
		content:     fixtool.FAQMarkdown,
	},
}

func newDocsCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "docs [topic]",
		Short: "Show bundled documentation",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return writeDocsIndex(cmd.OutOrStdout())
			}
			topic, ok := findDocsTopic(args[0])
			if !ok {
				return fmt.Errorf("unknown docs topic %q, available topics: %s", args[0], docsTopicNames())
			}
			_, err := fmt.Fprint(cmd.OutOrStdout(), topic.content())
			return err
		},
	}
	return command
}

func writeDocsIndex(out io.Writer) error {
	if _, err := fmt.Fprintln(out, "Bundled documentation topics:"); err != nil {
		return err
	}
	for _, topic := range docsTopics {
		if _, err := fmt.Fprintf(out, "  %s\t%s\n", topic.name, topic.description); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(out, "\nExamples:\n  fix-tool docs user-guide\n  fix-tool docs faq")
	return err
}

func findDocsTopic(name string) (docsTopic, bool) {
	for _, topic := range docsTopics {
		if topic.name == name {
			return topic, true
		}
	}
	return docsTopic{}, false
}

func docsTopicNames() string {
	names := make([]string, 0, len(docsTopics))
	for _, topic := range docsTopics {
		names = append(names, topic.name)
	}
	return strings.Join(names, ", ")
}
