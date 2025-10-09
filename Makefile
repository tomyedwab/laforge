PHONY: build

build:
	(cd agents && docker build -t opencode-agent -f Dockerfile.opencode .)

run:
	mkdir -p /tmp/opencode-agent/log
	docker run -it --rm \
		-v $(HOME)/.config/opencode:/home/opencode/.config/opencode \
		-v $(HOME)/.local/share/opencode/auth.json:/home/opencode/.local/share/opencode/auth.json \
		-v /tmp/opencode-agent/log:/home/opencode/.local/share/opencode/log \
		-v $(PWD):/src \
		opencode-agent -m moonshot/kimi-k2-0905-preview run "What can you do?"
