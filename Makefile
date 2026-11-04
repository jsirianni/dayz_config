PYTHON ?= python3
DZDEPLOY = .tools/bin/dzdeploy

.PHONY: tools build check test
tools:
	$(PYTHON) scripts/install-dzdeploy.py

# Compatibility alias: the tool is downloaded, never built in this repo.
build: tools

check: tools
	$(DZDEPLOY) tools install
	$(DZDEPLOY) validate all

test: check
	$(PYTHON) -m unittest discover -s scripts -p 'test_install_dzdeploy.py'
