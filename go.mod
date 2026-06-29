module docker-external-dns

go 1.22

require (
	github.com/cloudflare/cloudflare-go v0.93.0
	github.com/docker/docker v24.0.9+incompatible
	github.com/docker/go-connections v0.4.0
)

replace github.com/docker/distribution => github.com/docker/distribution v2.8.2+incompatible
