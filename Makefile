VERSION := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"
IMAGE   := ghcr.io/chrissnell/sparkyfish

.PHONY: all client server test vet clean \
        docker-build docker-push \
        bump-point bump-minor tag-and-push

all: client server

client:
	go build $(LDFLAGS) -o bin/sparkyfish ./cmd/sparkyfish

server:
	go build $(LDFLAGS) -o bin/sparkyfish-server ./cmd/sparkyfish-server

test:
	go test ./cmd/... ./pkg/...

vet:
	go vet ./cmd/... ./pkg/...

clean:
	rm -f bin/sparkyfish bin/sparkyfish-server

# --- Docker ---

docker-build:
	docker build --platform linux/amd64 --build-arg VERSION=$(VERSION) -t $(IMAGE):$(VERSION) -t $(IMAGE):latest .

docker-push:
	docker push $(IMAGE):$(VERSION)
	docker push $(IMAGE):latest

docker: docker-build docker-push

# --- Version bumping ---
# These targets increment the version, create a tag, and push it.

bump-point:
	@current=$(VERSION); \
	major=$$(echo $$current | sed 's/^v//' | cut -d. -f1); \
	minor=$$(echo $$current | sed 's/^v//' | cut -d. -f2); \
	point=$$(echo $$current | sed 's/^v//' | cut -d. -f3); \
	point=$${point:-0}; \
	new="v$$major.$$minor.$$((point + 1))"; \
	echo "$(VERSION) -> $$new"; \
	git tag -a "$$new" -m "Release $$new"; \
	git push origin "$$new"

bump-minor:
	@current=$(VERSION); \
	major=$$(echo $$current | sed 's/^v//' | cut -d. -f1); \
	minor=$$(echo $$current | sed 's/^v//' | cut -d. -f2); \
	new="v$$major.$$((minor + 1)).0"; \
	echo "$(VERSION) -> $$new"; \
	git tag -a "$$new" -m "Release $$new"; \
	git push origin "$$new"
