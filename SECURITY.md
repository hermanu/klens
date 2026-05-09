# Security Policy

## Supported versions

Only the latest released version of `klens` receives security fixes.

## Reporting a vulnerability

**Please do not file public issues for security vulnerabilities.**

Report security issues privately via GitHub's [private security advisories](https://github.com/hermanu/klens/security/advisories/new). I aim to acknowledge reports within 72 hours and will coordinate a fix and disclosure timeline with you.

When reporting, please include:

- A description of the vulnerability and its impact
- Steps to reproduce
- The version of `klens` affected
- Any suggested mitigation, if you have one

## Scope

`klens` is a local TUI that reads your kubeconfig and talks to your Kubernetes API server with the same credentials your `kubectl` would use. In-scope concerns include:

- Leakage of secret values, kubeconfig contents, or credentials beyond the local terminal
- Privilege escalation beyond what the kubeconfig user already has
- Code paths that could send cluster data to a remote endpoint

Out of scope:

- Issues caused by misconfigured kubeconfigs or excessive RBAC permissions on your cluster
- Vulnerabilities in upstream dependencies (please report those upstream; I will track and update)
