package cmd

import (
  "bytes"
  "embed"
  "errors"
  "fmt"
  "io/fs"
  "net/url"
  "os"
  "path/filepath"
  "strings"
  "text/template"
  "time"

  "github.com/dustin/go-humanize"
  "github.com/spf13/cobra"
)

//go:embed template.html
var embedded embed.FS

var errTargetIsADirectory = errors.New("target is a directory")
var errTargetExistsAndIsNotGenerated = errors.New("target already exists and is not a generated file")

type RootCmdRunner struct {
  dryRun bool
  recursive bool
  includeHidden bool
  stdout bool
  indexName string
  baseUrl string

  dirRelative string
  dirAbsolute string

  rootRelative string
  rootAbsolute string

  dirRelativeToRoot string
  dirChrooted string
  templateData IndexTemplate
}

type IndexTemplate struct {
  Name string
  Breadcrumbs []Breadcrumb
  NumDirs int
  NumFiles int
  CanGoUp bool
  Items []DirectoryItem
}

type Breadcrumb struct {
  Text string
  Link string
}

type DirectoryItem struct {
  URL string
  IsDir bool
  IsSymlink bool
  Name string
  Size int64
  ModTime time.Time
}

var rootCmdRunner = RootCmdRunner{}
var rootCmd = &cobra.Command{
  Use: "indexify <dir>",
  Version: "1.2.1",
  RunE: rootCmdRunner.Run,
  Args: cobra.ExactArgs(1),
}

func Execute() {
  if err := rootCmd.Execute(); err != nil {
    fmt.Println(err)
    os.Exit(1)
  }
}

func init() {
  rootCmd.Flags().StringVarP(
    &rootCmdRunner.rootRelative,
    "root", "", "",
    "path to root directory",
  )

  rootCmd.Flags().BoolVarP(
    &rootCmdRunner.includeHidden,
    "hidden", "", false,
    "index hidden files",
  )

  rootCmd.Flags().BoolVarP(
    &rootCmdRunner.dryRun,
    "dry-run", "n", false,
    "don't write anything to disk",
  )

  rootCmd.Flags().BoolVarP(
    &rootCmdRunner.recursive,
    "recursive", "r", false,
    "process directories recursively",
  )

  rootCmd.Flags().BoolVarP(
    &rootCmdRunner.stdout,
    "stdout", "", false,
    "output to stdout only",
  )

  rootCmd.Flags().StringVarP(
    &rootCmdRunner.indexName,
    "index-name", "", "index.html",
    "name of index file to generate",
  )

  rootCmd.Flags().StringVarP(
    &rootCmdRunner.baseUrl,
    "base-url", "", "",
    "base url to use for links (if the files are hosted elsewhere)",
  )

  rootCmd.MarkFlagRequired("root")
}

func (runner *RootCmdRunner) Run(cmd *cobra.Command, args []string) error {
  err := runner.prepare(args[0])

  if err != nil {
    return err
  }

  if runner.recursive {
    return filepath.WalkDir(
      runner.dirRelative,
      func(path string, d fs.DirEntry, err error) error {
        if err != nil {
          return err
        }

        if !d.IsDir() {
          return nil
        }

        err = runner.prepare(path)

        if err != nil {
          return err
        }

        return runner.execute()
      })
  }

  return runner.execute()
}

func (runner *RootCmdRunner) prepare(dir string) error {
  runner.dirRelative = dir

  var err error

  runner.dirAbsolute, err = filepath.Abs(runner.dirRelative)

  if err != nil {
    return err
  }

  runner.rootAbsolute, err = filepath.Abs(runner.rootRelative)

  if err != nil {
    return err
  }

  runner.dirRelativeToRoot, err = filepath.Rel(
    runner.rootAbsolute,
    runner.dirAbsolute,
  )

  if strings.HasPrefix(runner.dirRelativeToRoot, "..") {
    return fmt.Errorf("directory is outside root")
  }

  runner.dirChrooted = filepath.Join("/", runner.dirRelativeToRoot)
  runner.templateData = IndexTemplate{
    Name: fmt.Sprintf("Index: %s", runner.dirChrooted),
    CanGoUp: runner.dirAbsolute != runner.rootAbsolute,
  }

  return err
}

func (runner *RootCmdRunner) execute() error {
  err := runner.fetchData()

  if err != nil {
    return err
  }

  runner.generateBreadcrumbs()
  err = runner.render()

  if err != nil && (
    errors.Is(err, errTargetIsADirectory) ||
    errors.Is(err, errTargetExistsAndIsNotGenerated)) {

    fmt.Printf("skipped: %s\n", err)
    return nil
  }

  return err
}

func (runner *RootCmdRunner) fetchData() error {
  var err error

  files, err := os.ReadDir(runner.dirAbsolute)

  if err != nil {
    return err
  }

  for _, dirEntry := range files {
    info, err := dirEntry.Info()

    if err != nil {
      return err
    }

    name := dirEntry.Name()

    if !runner.includeHidden && strings.HasPrefix(name, ".") {
      continue
    }

    if name == "index.html" {
      continue
    }

    item := DirectoryItem{
      URL: filepath.Join(runner.baseUrl, name),
      IsDir: dirEntry.IsDir(),
      IsSymlink: info.Mode() & fs.ModeSymlink > 0,
      Name: dirEntry.Name(),
      Size: info.Size(),
      ModTime: info.ModTime().UTC(),
    }

    runner.templateData.Items = append(runner.templateData.Items, item)

    if dirEntry.IsDir() {
      runner.templateData.NumDirs += 1
    } else {
      runner.templateData.NumFiles += 1
    }
  }

  return nil
}

func (runner *RootCmdRunner) generateBreadcrumbs() {
  if len(runner.dirChrooted) == 0 {
    return
  }

  // skip trailing slash
  lpath := runner.dirChrooted

  if lpath[len(lpath)-1] == '/' {
    lpath = lpath[:len(lpath)-1]
  }

  parts := strings.Split(lpath, "/")
  result := make([]Breadcrumb, len(parts))

  for i, p := range parts {
    if i == 0 && p == "" {
      p = "/"
    }

    // the directory name could include an encoded slash in its path, so the
    // item name should be unescaped in the loop rather than unescaping the
    // entire path outside the loop.

    p, _ = url.PathUnescape(p)
    lnk := strings.Repeat("../", len(parts)-i-1)
    result[i] = Breadcrumb{Link: lnk, Text: p}
  }

  runner.templateData.Breadcrumbs = result
}

func (runner *RootCmdRunner) render() error {
  var err error

  t, err := template.ParseFS(embedded, "template.html")

  if err != nil {
    return err
  }

  if runner.stdout {
    return t.Execute(os.Stdout, runner.templateData)
  }

  return runner.renderToFile(t)
}

func (runner *RootCmdRunner) renderToFile(t *template.Template) error {
  err := runner.checkRenderTarget()

  if err != nil {
    return err
  }

  if runner.dryRun {
    fmt.Println("[dry-run] write", runner.renderTargetPath())
    return nil
  }

  f, err := os.Create(runner.renderTargetPath())

  if err != nil {
    return err
  }

  defer f.Close()

  return t.Execute(f, runner.templateData)
}

func (runner *RootCmdRunner) checkRenderTarget() error {
  f, err := os.Open(runner.renderTargetPath())

  if err != nil {
    // failure to open probably meant the file was not found, which is ok
    return nil
  }

  defer f.Close()
  info, err := f.Stat()

  if err != nil {
    return err
  }

  if info.IsDir() {
    return fmt.Errorf(
      "%w: %s", errTargetIsADirectory, runner.renderTargetPath(),
    )
  }

  buf := new(bytes.Buffer)
  buf.ReadFrom(f)
  data := buf.String()

  if strings.Contains(data, "Index generated with") {
    return nil
  }

  return fmt.Errorf(
    "%w: %s", errTargetExistsAndIsNotGenerated, runner.renderTargetPath(),
  )
}

func (runner *RootCmdRunner) renderTargetPath() string {
  return filepath.Join(runner.dirRelative, runner.indexName)
}

func (di *DirectoryItem) HumanModTime(format string) string {
  return di.ModTime.Format(format)
}

func (di *DirectoryItem) HumanSize() string {
  return humanize.IBytes(uint64(di.Size))
}
