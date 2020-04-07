# lanhttp

lanhttp wraps http.Client to automatically route internal traffic (denoted by
*.internal) URLs to internal endpoints, and route other traffic normally over
the internet. This is an alternative to consul and other DNS-level routing.

It distributes traffic randomly among the internal IPs.

## Usage

```
import "egt.run/lanhttp"
```
