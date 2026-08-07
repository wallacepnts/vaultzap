BINARY  := vaultzap
IMAGEM  := ghcr.io/wallacepnts/vaultzap:latest
PLATAFORMAS := linux/amd64,linux/arm64
CONTAINERS_DIR := $(HOME)/.config/containers
QUADLET_DIR    := $(CONTAINERS_DIR)/systemd
ENV_DIR        := $(CONTAINERS_DIR)/env
VOLUMES_DIR    := $(CONTAINERS_DIR)/volumes/vaultzap

.PHONY: dev test lint build image image-multiarch quadlet-install

dev:
	go run -tags dev ./cmd/vaultzap

test:
	go test ./...

lint:
	go vet ./...
	@saida="$$(gofmt -l .)"; \
	if [ -n "$$saida" ]; then \
		echo "arquivos fora do padrão gofmt:"; \
		echo "$$saida"; \
		exit 1; \
	fi

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BINARY) ./cmd/vaultzap

image:
	podman build -t $(IMAGEM) -f deploy/Dockerfile .

# Builda e publica um manifest multi-arch (amd64+arm64); exige "podman login ghcr.io" antes.
# Builds and pushes a multi-arch manifest (amd64+arm64); requires "podman login ghcr.io" first.
image-multiarch:
	podman build --platform $(PLATAFORMAS) --manifest $(IMAGEM) -f deploy/Dockerfile .
	podman manifest push $(IMAGEM) docker://$(IMAGEM)

# Um único arquivo quadlet, então ele fica solto em systemd/ (subpasta só a
# partir de dois). O .env é copiado com -n: nunca sobrescreve o que você editou.
# A single quadlet file, so it sits loose in systemd/ (a subfolder only from
# two up). The .env is copied with -n: it never overwrites what you edited.
quadlet-install:
	mkdir -p $(QUADLET_DIR) $(ENV_DIR) $(VOLUMES_DIR)/data $(VOLUMES_DIR)/inbox
	cp deploy/vaultzap.container $(QUADLET_DIR)/
	cp -n deploy/vaultzap.env.example $(ENV_DIR)/vaultzap.env || true
	systemctl --user daemon-reload
