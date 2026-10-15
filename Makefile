install:
	uv sync --extra dev

dev:
	uv run media-report doctor

lint:
	uv run ruff check .

format:
	uv run ruff format .

test:
	uv run pytest

build:
	uv run python -m build

doctor:
	uv run media-report doctor

go-fmt:
	gofmt -w cmd internal

go-test:
	go test ./...

go-vet:
	go vet ./...

go-check:
	test -z "$$(gofmt -l cmd internal)"
	go vet ./...
	go test ./...
