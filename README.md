# indexify

Static directory index generator, based on [Caddy's implementation](https://github.com/caddyserver/caddy/tree/master/modules/caddyhttp/fileserver).

## Usage

```
Usage:
  indexify <dir> [flags]

Flags:
  -n, --dry-run             don't write anything to disk
  -h, --help                help for indexify
      --hidden              index hidden files
      --index-name string   name of index file to generate (default "index.html")
      --root string         path to root directory
      --stdout              output to stdout only
  -v, --version             version for indexify
```

If you need to process directories recursively, just use `find`:

```bash
find /path/to/root -type d -exec indexify --root /path/to/root {} \;
```
