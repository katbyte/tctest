package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/katbyte/tctest/lib/cout"
	"github.com/katbyte/tctest/lib/tc"
)

// QueueListCmd prints the builds in the TeamCity build queue, only those whose full build configuration name matches
// filter when it is not nil.
func (f *FlagData) QueueListCmd(filter *regexp.Regexp) error {
	matched, err := getQueuedBuilds(f.NewTCServer(), filter)
	if err != nil {
		return err
	}

	for _, b := range matched {
		cout.Quietf("%d@%s %s\n", b.ID, b.BuildTypeID, b.URL)
	}

	return nil
}

// QueueRemoveCmd removes every build in the TeamCity build queue whose full build configuration name matches filter,
// asking for confirmation first unless --force is set.
func (f *FlagData) QueueRemoveCmd(filter *regexp.Regexp, confirmIn io.Reader) error {
	server := f.NewTCServer()

	matched, err := getQueuedBuilds(server, filter)
	if err != nil {
		return err
	}
	if len(matched) == 0 {
		cout.Printf("no queued builds to remove\n")
		return nil
	}

	if f.DryRun {
		cout.Printf("<yellow>[DRY RUN]</> would remove <yellow>%d</> queued builds\n", len(matched))
		return nil
	}

	if !f.Force {
		ok, err := confirm(confirmIn, fmt.Sprintf("remove %d queued builds matching %q? [y/N]: ", len(matched), filter))
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("aborted, no queued builds were removed (use --force to skip confirmation)")
		}
	}

	comment := fmt.Sprintf("removed from queue by tctest (matched %q)", filter)
	removed := 0
	var failed []string
	for _, b := range matched {
		err := server.CancelQueuedBuild(b.ID, comment)
		switch {
		case err == nil:
			removed++
			cout.Printf("  removed <green>%d</> %s\n", b.ID, b.FullName())
			cout.Quietf("%d@%s %s\n", b.ID, b.BuildTypeID, b.URL)
		case errors.Is(err, tc.ErrNotQueued):
			cout.Errorf("  <yellow>WARNING:</> skipped %d %s: %v\n", b.ID, b.FullName(), err)
		default:
			failed = append(failed, strconv.Itoa(b.ID))
			cout.Errorf("  <red>ERROR:</> %v\n", err)
		}
	}

	cout.Printf("removed <green>%d</> of <yellow>%d</> matching queued builds\n", removed, len(matched))

	if len(failed) > 0 {
		return fmt.Errorf("failed to remove %d queued build(s): %s", len(failed), strings.Join(failed, ", "))
	}

	return nil
}

// getQueuedBuilds fetches the build queue, then prints and returns the builds whose full build configuration name
// matches filter (every build when filter is nil).
func getQueuedBuilds(server tc.Server, filter *regexp.Regexp) ([]tc.QueuedBuild, error) {
	cout.Printf("Retrieving build queue...")
	queue, err := server.GetBuildQueue()
	if err != nil {
		cout.Println()
		return nil, fmt.Errorf("error retrieving build queue: %w", err)
	}
	cout.Printf(" found <yellow>%d</> queued builds\n", len(queue))

	matched := queue
	if filter != nil {
		matched = slices.DeleteFunc(queue, func(b tc.QueuedBuild) bool {
			return !filter.MatchString(b.FullName())
		})
		cout.Printf("<yellow>%d</> match <cyan>%s</>\n", len(matched), filter)
	}

	for _, b := range matched {
		if b.Branch != "" {
			cout.Printf("  <green>%d</> %s <magenta>[%s]</>\n", b.ID, b.FullName(), b.Branch)
		} else {
			cout.Printf("  <green>%d</> %s\n", b.ID, b.FullName())
		}
		cout.Verbosef("    <darkGray>%s</>\n", b.URL)
	}

	return matched, nil
}

// confirm writes prompt to stderr, so it shows even when stdout output is quiet or silenced, and reports whether the
// line read from in is a yes. No answer at all (EOF, e.g. stdin is empty or /dev/null) counts as no.
func confirm(in io.Reader, prompt string) (bool, error) {
	fmt.Fprint(os.Stderr, prompt)

	answer, err := bufio.NewReader(in).ReadString('\n')
	if err != nil {
		if !errors.Is(err, io.EOF) {
			return false, fmt.Errorf("error reading confirmation: %w", err)
		}
		fmt.Fprintln(os.Stderr) // the prompt never got a newline from the user
	}

	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}
