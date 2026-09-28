package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"shiori/internal/app"
	"shiori/internal/web"
)

// 終了コード: 0=成功、2=使い方エラー、1=実行時エラー。
const (
	ExitOK    = 0
	ExitError = 1
	ExitUsage = 2
)

// Run はCLIのエントリ。stdoutに通常出力、stderrに診断を書く。
// JSON要求時はstdoutにJSONのみを書く。
func Run(ctx context.Context, args []string, stdout, stderr io.Writer, svc *app.Service) int {
	if len(args) == 0 {
		printUsage(stderr)
		return ExitUsage
	}
	switch args[0] {
	case "add":
		return runAdd(ctx, args[1:], stdout, stderr, svc)
	case "list":
		return runList(ctx, args[1:], stdout, stderr, svc)
	case "show":
		return runShow(ctx, args[1:], stdout, stderr, svc)
	case "search":
		return runSearch(ctx, args[1:], stdout, stderr, svc)
	case "update":
		return runUpdate(ctx, args[1:], stdout, stderr, svc)
	case "delete":
		return runDelete(ctx, args[1:], stdout, stderr, svc)
	case "favorite":
		return runFavorite(ctx, true, args[1:], stdout, stderr, svc)
	case "unfavorite":
		return runFavorite(ctx, false, args[1:], stdout, stderr, svc)
	case "tag":
		return runTag(ctx, args[1:], stdout, stderr, svc)
	case "serve":
		return runServe(ctx, args[1:], stdout, stderr, svc)
	case "-h", "--help", "help":
		printUsage(stdout)
		return ExitOK
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		printUsage(stderr)
		return ExitUsage
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `usage:
  shiori add <URL> [--title T] [--comment C] [--tag T]... [--json]
  shiori list [--tag T] [--favorites] [--limit N] [--offset N] [--json]
  shiori show <ID> [--json]
  shiori search <QUERY> [--tag T] [--limit N] [--offset N] [--json]
  shiori update <ID> [--url U] [--title T] [--comment C] [--json]
  shiori delete <ID>
  shiori favorite <ID> [--json]
  shiori unfavorite <ID> [--json]
  shiori tag add <ID> <TAG> [--json]
  shiori tag remove <ID> <TAG> [--json]
  shiori tag list [--json]
  shiori serve [--addr localhost:8080]`)
}

type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ",") }
func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

// splitLeadingPositionals は `add <URL> --flag` のように位置引数が
// フラグより前に来る使い方を支える。先頭n引数がフラグらしくなければ
// 位置引数として切り出し、残りをflagパース対象にする。
func splitLeadingPositionals(args []string, n int) (pos []string, rest []string) {
	if len(args) >= n {
		ok := true
		for _, a := range args[:n] {
			if a == "--" || strings.HasPrefix(a, "-") {
				ok = false
				break
			}
		}
		if ok {
			return args[:n], args[n:]
		}
	}
	return nil, args
}

func writeJSON(stdout io.Writer, v any) error {
	enc := json.NewEncoder(stdout)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

func errf(stderr io.Writer, format string, a ...any) {
	fmt.Fprintf(stderr, "shiori: "+format+"\n", a...)
}

// 人間向け1行: "[★ ]<id> <url> [tag1,tag2] <title>"
func formatOneLine(b app.Bookmark) string {
	star := ""
	if b.Favorite {
		star = "★ "
	}
	tags := ""
	if len(b.Tags) > 0 {
		tags = " [" + strings.Join(b.Tags, ",") + "]"
	}
	title := ""
	if b.Title != "" {
		title = " " + b.Title
	}
	return fmt.Sprintf("%s%d %s%s%s", star, b.ID, b.URL, tags, title)
}

func formatDetail(b app.Bookmark) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "ID: %d\n", b.ID)
	fmt.Fprintf(&sb, "URL: %s\n", b.URL)
	fmt.Fprintf(&sb, "Title: %s\n", b.Title)
	fmt.Fprintf(&sb, "Comment: %s\n", b.Comment)
	fmt.Fprintf(&sb, "Tags: %s\n", strings.Join(b.Tags, ", "))
	fmt.Fprintf(&sb, "Favorite: %v\n", b.Favorite)
	fmt.Fprintf(&sb, "Created: %s\n", b.CreatedAt)
	fmt.Fprintf(&sb, "Updated: %s\n", b.UpdatedAt)
	return sb.String()
}

func outputBookmark(stdout io.Writer, b app.Bookmark, asJSON bool) error {
	if asJSON {
		return writeJSON(stdout, b)
	}
	_, err := fmt.Fprint(stdout, formatDetail(b))
	return err
}

func outputBookmarks(stdout io.Writer, items []app.Bookmark, asJSON bool) error {
	if asJSON {
		if items == nil {
			items = []app.Bookmark{}
		}
		return writeJSON(stdout, items)
	}
	for _, b := range items {
		if _, err := fmt.Fprintln(stdout, formatOneLine(b)); err != nil {
			return err
		}
	}
	return nil
}

func parseID(s string) (int64, error) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid ID %q: positive integer required", s)
	}
	return id, nil
}

func runAdd(ctx context.Context, args []string, stdout, stderr io.Writer, svc *app.Service) int {
	pos, flagArgs := splitLeadingPositionals(args, 1)
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var title, comment string
	var tags stringSlice
	var asJSON bool
	fs.StringVar(&title, "title", "", "title")
	fs.StringVar(&comment, "comment", "", "comment")
	fs.Var(&tags, "tag", "tag (repeatable)")
	fs.BoolVar(&asJSON, "json", false, "JSON output")
	if err := fs.Parse(flagArgs); err != nil {
		return ExitUsage
	}
	rest := fs.Args()
	if pos != nil {
		rest = append(pos, rest...)
	}
	if len(rest) != 1 {
		errf(stderr, "add requires exactly 1 URL argument")
		return ExitUsage
	}
	b, err := svc.Add(ctx, rest[0], title, comment, []string(tags))
	if err != nil {
		errf(stderr, "add: %v", err)
		return ExitError
	}
	if err := outputBookmark(stdout, b, asJSON); err != nil {
		errf(stderr, "output: %v", err)
		return ExitError
	}
	return ExitOK
}

func runList(ctx context.Context, args []string, stdout, stderr io.Writer, svc *app.Service) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var tag string
	var limit, offset int
	var favorites bool
	var asJSON bool
	fs.StringVar(&tag, "tag", "", "filter by tag")
	fs.BoolVar(&favorites, "favorites", false, "favorites only")
	fs.IntVar(&limit, "limit", 50, "max rows")
	fs.IntVar(&offset, "offset", 0, "offset")
	fs.BoolVar(&asJSON, "json", false, "JSON output")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	if len(fs.Args()) != 0 {
		errf(stderr, "list takes no positional arguments")
		return ExitUsage
	}
	var items []app.Bookmark
	var err error
	switch {
	case favorites && tag != "":
		errf(stderr, "list: --favorites cannot be combined with --tag")
		return ExitUsage
	case favorites:
		items, err = svc.ListFavorites(ctx, limit, offset)
	default:
		items, err = svc.List(ctx, tag, limit, offset)
	}
	if err != nil {
		errf(stderr, "list: %v", err)
		return ExitError
	}
	if err := outputBookmarks(stdout, items, asJSON); err != nil {
		errf(stderr, "output: %v", err)
		return ExitError
	}
	return ExitOK
}

func runShow(ctx context.Context, args []string, stdout, stderr io.Writer, svc *app.Service) int {
	pos, flagArgs := splitLeadingPositionals(args, 1)
	fs := flag.NewFlagSet("show", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var asJSON bool
	fs.BoolVar(&asJSON, "json", false, "JSON output")
	if err := fs.Parse(flagArgs); err != nil {
		return ExitUsage
	}
	rest := fs.Args()
	if pos != nil {
		rest = append(pos, rest...)
	}
	if len(rest) != 1 {
		errf(stderr, "show requires exactly 1 ID argument")
		return ExitUsage
	}
	id, err := parseID(rest[0])
	if err != nil {
		errf(stderr, "show: %v", err)
		return ExitUsage
	}
	b, err := svc.Get(ctx, id)
	if err != nil {
		errf(stderr, "show: %v", err)
		return ExitError
	}
	if err := outputBookmark(stdout, b, asJSON); err != nil {
		errf(stderr, "output: %v", err)
		return ExitError
	}
	return ExitOK
}

func runSearch(ctx context.Context, args []string, stdout, stderr io.Writer, svc *app.Service) int {
	pos, flagArgs := splitLeadingPositionals(args, 1)
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var tag string
	var limit, offset int
	var asJSON bool
	fs.StringVar(&tag, "tag", "", "filter by tag")
	fs.IntVar(&limit, "limit", 50, "max rows")
	fs.IntVar(&offset, "offset", 0, "offset")
	fs.BoolVar(&asJSON, "json", false, "JSON output")
	if err := fs.Parse(flagArgs); err != nil {
		return ExitUsage
	}
	rest := fs.Args()
	if pos != nil {
		rest = append(pos, rest...)
	}
	if len(rest) != 1 {
		errf(stderr, "search requires exactly 1 QUERY argument")
		return ExitUsage
	}
	items, err := svc.Search(ctx, rest[0], tag, limit, offset)
	if err != nil {
		errf(stderr, "search: %v", err)
		return ExitError
	}
	if err := outputBookmarks(stdout, items, asJSON); err != nil {
		errf(stderr, "output: %v", err)
		return ExitError
	}
	return ExitOK
}

func runUpdate(ctx context.Context, args []string, stdout, stderr io.Writer, svc *app.Service) int {
	pos, flagArgs := splitLeadingPositionals(args, 1)
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var urlFlag, titleFlag, commentFlag string
	var hasURL, hasTitle, hasComment bool
	fs.Func("url", "new URL", func(v string) error { urlFlag, hasURL = v, true; return nil })
	fs.Func("title", "new title", func(v string) error { titleFlag, hasTitle = v, true; return nil })
	fs.Func("comment", "new comment", func(v string) error { commentFlag, hasComment = v, true; return nil })
	var asJSON bool
	fs.BoolVar(&asJSON, "json", false, "JSON output")
	if err := fs.Parse(flagArgs); err != nil {
		return ExitUsage
	}
	rest := fs.Args()
	if pos != nil {
		rest = append(pos, rest...)
	}
	if len(rest) != 1 {
		errf(stderr, "update requires exactly 1 ID argument")
		return ExitUsage
	}
	if !hasURL && !hasTitle && !hasComment {
		errf(stderr, "update requires at least one of --url, --title, --comment")
		return ExitUsage
	}
	id, err := parseID(rest[0])
	if err != nil {
		errf(stderr, "update: %v", err)
		return ExitUsage
	}
	var in app.UpdateInput
	if hasURL {
		in.URL = &urlFlag
	}
	if hasTitle {
		in.Title = &titleFlag
	}
	if hasComment {
		in.Comment = &commentFlag
	}
	b, err := svc.Update(ctx, id, in)
	if err != nil {
		errf(stderr, "update: %v", err)
		return ExitError
	}
	if err := outputBookmark(stdout, b, asJSON); err != nil {
		errf(stderr, "output: %v", err)
		return ExitError
	}
	return ExitOK
}

func runDelete(ctx context.Context, args []string, stdout, stderr io.Writer, svc *app.Service) int {
	pos, flagArgs := splitLeadingPositionals(args, 1)
	fs := flag.NewFlagSet("delete", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(flagArgs); err != nil {
		return ExitUsage
	}
	rest := fs.Args()
	if pos != nil {
		rest = append(pos, rest...)
	}
	if len(rest) != 1 {
		errf(stderr, "delete requires exactly 1 ID argument")
		return ExitUsage
	}
	id, err := parseID(rest[0])
	if err != nil {
		errf(stderr, "delete: %v", err)
		return ExitUsage
	}
	if err := svc.Delete(ctx, id); err != nil {
		errf(stderr, "delete: %v", err)
		return ExitError
	}
	fmt.Fprintf(stdout, "deleted %d\n", id)
	return ExitOK
}

func runTag(ctx context.Context, args []string, stdout, stderr io.Writer, svc *app.Service) int {
	if len(args) == 0 {
		errf(stderr, "tag requires subcommand: add|remove|list")
		return ExitUsage
	}
	switch args[0] {
	case "add", "remove":
		return runTagAddRemove(ctx, args[0] == "add", args[1:], stdout, stderr, svc)
	case "list":
		return runTagList(ctx, args[1:], stdout, stderr, svc)
	default:
		errf(stderr, "unknown tag subcommand: %s", args[0])
		return ExitUsage
	}
}

func runTagList(ctx context.Context, args []string, stdout, stderr io.Writer, svc *app.Service) int {
	fs := flag.NewFlagSet("tag list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var asJSON bool
	fs.BoolVar(&asJSON, "json", false, "JSON output")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	if len(fs.Args()) != 0 {
		errf(stderr, "tag list takes no positional arguments")
		return ExitUsage
	}
	tags, err := svc.ListTags(ctx)
	if err != nil {
		errf(stderr, "tag list: %v", err)
		return ExitError
	}
	if asJSON {
		if tags == nil {
			tags = []app.TagCount{}
		}
		if err := writeJSON(stdout, tags); err != nil {
			errf(stderr, "output: %v", err)
			return ExitError
		}
		return ExitOK
	}
	for _, t := range tags {
		if _, err := fmt.Fprintf(stdout, "%s\t%d\n", t.Name, t.Count); err != nil {
			errf(stderr, "output: %v", err)
			return ExitError
		}
	}
	return ExitOK
}

func runTagAddRemove(ctx context.Context, isAdd bool, args []string, stdout, stderr io.Writer, svc *app.Service) int {
	name := "tag add"
	if !isAdd {
		name = "tag remove"
	}
	pos, flagArgs := splitLeadingPositionals(args, 2)
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	var asJSON bool
	fs.BoolVar(&asJSON, "json", false, "JSON output")
	if err := fs.Parse(flagArgs); err != nil {
		return ExitUsage
	}
	rest := fs.Args()
	if pos != nil {
		rest = append(pos, rest...)
	}
	if len(rest) != 2 {
		errf(stderr, "%s requires <ID> <TAG>", name)
		return ExitUsage
	}
	id, err := parseID(rest[0])
	if err != nil {
		errf(stderr, "%s: %v", name, err)
		return ExitUsage
	}
	var b app.Bookmark
	if isAdd {
		b, err = svc.TagAdd(ctx, id, rest[1])
	} else {
		b, err = svc.TagRemove(ctx, id, rest[1])
	}
	if err != nil {
		errf(stderr, "%s: %v", name, err)
		return ExitError
	}
	if err := outputBookmark(stdout, b, asJSON); err != nil {
		errf(stderr, "output: %v", err)
		return ExitError
	}
	return ExitOK
}

func defaultAddr() string {
	if v := os.Getenv("SHIORI_ADDR"); v != "" {
		return v
	}
	return "0.0.0.0:8080"
}

func runServe(ctx context.Context, args []string, stdout, stderr io.Writer, svc *app.Service) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := defaultAddr()
	fs.StringVar(&addr, "addr", addr, "listen address")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	if len(fs.Args()) != 0 {
		errf(stderr, "serve takes no positional arguments")
		return ExitUsage
	}
	fmt.Fprintf(stderr, "shiori: listening on http://%s\n", addr)
	if err := web.Serve(ctx, addr, svc); err != nil {
		errf(stderr, "serve: %v", err)
		return ExitError
	}
	return ExitOK
}

func runFavorite(ctx context.Context, fav bool, args []string, stdout, stderr io.Writer, svc *app.Service) int {
	name := "favorite"
	if !fav {
		name = "unfavorite"
	}
	pos, flagArgs := splitLeadingPositionals(args, 1)
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	var asJSON bool
	fs.BoolVar(&asJSON, "json", false, "JSON output")
	if err := fs.Parse(flagArgs); err != nil {
		return ExitUsage
	}
	rest := fs.Args()
	if pos != nil {
		rest = append(pos, rest...)
	}
	if len(rest) != 1 {
		errf(stderr, "%s requires exactly 1 ID argument", name)
		return ExitUsage
	}
	id, err := parseID(rest[0])
	if err != nil {
		errf(stderr, "%s: %v", name, err)
		return ExitUsage
	}
	b, err := svc.SetFavorite(ctx, id, fav)
	if err != nil {
		errf(stderr, "%s: %v", name, err)
		return ExitError
	}
	if err := outputBookmark(stdout, b, asJSON); err != nil {
		errf(stderr, "output: %v", err)
		return ExitError
	}
	return ExitOK
}
