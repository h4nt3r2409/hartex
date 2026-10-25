<!--
  This PR template is specific to hartex (the ARTEX global edition).
  Read CONTRIBUTING.md first for the contribution policy.
-->

## Summary

<!-- What changed, and why, in one or two sentences. -->

## Change type

<!-- Put an x in the matching box. -->

- [ ] Bug fix (`fix`)
- [ ] New feature (`feat`)
- [ ] Documentation (`docs`)
- [ ] Refactor / cleanup (`refactor` / `chore`)
- [ ] Other:

## Related issues

<!-- For example: Closes #123 -->

## How it was verified

<!-- Which commands did you run, and what did you observe? Logs are welcome. -->

- [ ] Go changes: `go build ./...` · `go vet ./...` · `go test ./...`
- [ ] Web changes: `cd web && npm run check && npm test -- --run && npm run build:static`
- [ ] UI changes: screenshots attached
- [ ] English-only invariant: `python3 -I scripts/check-source-language.py` passes

## Authorized-use confirmation

- [ ] This change does not encourage use outside the [authorized scope](../README.md#️-read-first--authorized-use-and-legal-notice)
      or applicable law. Verification was performed only against owned/authorized targets or in a
      locally isolated environment.
- [ ] Test or scan artifacts are not included in the commit.
