# indexify

Static directory index generator, based on [Caddy's implementation](https://github.com/caddyserver/caddy/tree/master/modules/caddyhttp/fileserver).

## Usage

```
Usage:
  indexify <dir> [flags]

Flags:
      --base-url string     base url to use for links (if the files are hosted elsewhere)
  -n, --dry-run             don't write anything to disk
  -h, --help                help for indexify
      --hidden              index hidden files
      --index-name string   name of index file to generate (default "index.html")
  -r, --recursive           process directories recursively
      --root string         path to root directory
      --stdout              output to stdout only
  -v, --version             version for indexify
```
