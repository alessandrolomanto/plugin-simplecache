# Simple Cache

Simple cache plugin middleware caches responses on disk.

## Configuration

To configure this plugin you should add its configuration to the Traefik dynamic configuration as explained [here](https://docs.traefik.io/getting-started/configuration-overview/#the-dynamic-configuration).
The following snippet shows how to configure this plugin with the File provider in TOML and YAML: 

Static:

```toml
[experimental.plugins.cache]
  modulename = "github.com/alessandrolomanto/plugin-simplecache"
  version = "v0.0.1"
```

Dynamic:

```toml
[http.middlewares]
  [http.middlewares.my-cache.plugin.cache]
    path = "/some/path/to/cache/dir"
```

```yaml
http:
  middlewares:
   my-cache:
      plugin:
        cache:
          path: /some/path/to/cache/dir
```

### Options

#### Path (`path`)

The base path that files will be created under. This must be a valid existing
filesystem path.

#### Max Expiry (`maxExpiry`)

*Default: 300*

The maximum number of seconds a response can be cached for. The 
actual cache time will always be lower or equal to this.

#### Cleanup (`cleanup`)

*Default: 600*

The number of seconds to wait between cache cleanup runs.
	
#### Add Status Header (`addStatusHeader`)

*Default: true*

This determines if the cache status header `Cache-Status` will be added to the
response headers. This header can have the value `hit`, `miss` or `error`.

## Features

### Query Parameter Handling

The cache key includes all query parameters, ensuring that requests with different query parameters are cached separately. 
Some of the key features:

- Full URL query string support (path + query parameters)
- Proper handling of URL-encoded characters in query parameters
- Consistent caching regardless of parameter order (parameters are sorted)
- Support for multiple values for the same parameter

### Caching Behavior

Only responses that are cacheable according to HTTP standards are cached:

- Only cacheable status codes (200, 203, 204, etc.)
- Respects Cache-Control headers
- Automatic expiration based on max-age directives or plugin configuration

### Error Handling

Improved error handling throughout the plugin:
- Detailed error logging 
- Proper handling of cache misses and errors
- Failsafe mechanisms to prevent serving invalid cached content
