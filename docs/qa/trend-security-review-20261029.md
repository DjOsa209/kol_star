# Security Baseline Scan Report

## Metadata
- schemaVersion: 1.1
- baseline: sec-baseline.md
- scope: Trend insights staged source, tests, migration, scripts and deployment examples
- generatedAt: 2026-10-29T18:47:05+08:00

## Summary
- result: incomplete
- critical: 0
- high: 0
- medium: 0
- low: 0
- manualReview: 2

## Findings
No findings.

## Manual Review
### REVIEW-01 Human code review before main integration
- rule: User-provided AGENTS.md SEC-AI-01
- reason: Automated checks and agent source review cannot establish that a human has reviewed the code.
- requiredEvidence: Human review of the prepared commit before integration into main.

### REVIEW-02 Deployment and organizational controls
- rule: Baseline sections 2.2, 2.3, 3.2 and 8
- reason: Production network configuration, organizational approval, retention policy, artifact signing and remote CI enforcement are outside this local code review. No dependency versions were changed.
- requiredEvidence: Deployment owner verifies HTTPS, private persistent storage, access scope, organizational permissions and existing CI controls.

## Coverage
- checked: Owner and menu authorization; region-bound upload grants; cryptographic grant generation; expiry, cancellation and replay handling; payload size and completeness; upload log redaction; image URL and public-IP checks; redirect rejection; output encoding; fixed browser request targets; runtime files excluded from staging; synthetic test credentials only.
- notChecked: Full Semgrep/Gitleaks or dependency vulnerability scan (tools unavailable); external CI policy; human review; target server configuration. Limited checks do not certify the entire repository or production deployment.
- tools: ESLint security rules (no-eval, no-implied-eval, no-new-func, no-script-url, vue/no-v-html), Go vet, staged-content credential-pattern scan (values suppressed), source data-flow review, Go race tests, Vue type check, git diff --check.

All backend race tests passed in 78.065 seconds. The snapshot test now creates synthetic fixtures and does not depend on harvested local data. Credential-pattern matches were confined to negative test fixtures. No raw browser session files or harvested datasets were inspected or staged for this review.
