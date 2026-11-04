DZDEPLOY = .tools/bin/dzdeploy
ifeq ($(OS),Windows_NT)
INSTALL_DZDEPLOY = powershell.exe -NoProfile -File scripts/install-dzdeploy.ps1
VERIFY_DZDEPLOY = powershell.exe -NoProfile -File scripts/verify-dzdeploy-checkout.ps1
else
INSTALL_DZDEPLOY = bash scripts/install-dzdeploy.sh
VERIFY_DZDEPLOY = bash scripts/verify-dzdeploy-checkout.sh
endif

.PHONY: tools build check test
tools:
	$(INSTALL_DZDEPLOY)

# Compatibility alias: the tool is downloaded, never built in this repo.
build: tools

check: tools
	$(DZDEPLOY) tools install
	$(DZDEPLOY) validate all

test: check
	$(VERIFY_DZDEPLOY)
