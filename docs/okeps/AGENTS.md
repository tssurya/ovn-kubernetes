# AGENTS.md — OVN-Kubernetes Enhancement Proposals (OKEPs)

This directory contains OKEPs — design documents for significant new
features in OVN-Kubernetes that aim to solve end user pain points or
new use cases.

## What is an OKEP

A structured design proposal with an associated GitHub enhancement
tracking issue. Template: `okep-4368-template.md`.
Naming convention: `okep-XXXX-title.md` where XXXX is the issue number.

## Required Sections

- Every OKEP MUST include all sections from the template
  (`okep-4368-template.md`) with substantive content. Flag as
  incomplete if any section contains placeholder text.
- The OKEP MUST start with the intent (the *why*), followed by
  high-level end-to-end use cases describing what the user is trying
  to achieve. Flag as incorrect if use cases describe implementation
  details (e.g. "Add New API CRD", "add IPs to address-set") instead
  of user-facing outcomes.
- PRs adding OKEPs MUST also update `mkdocs.yml` under Enhancement
  Proposals.

## Quality Expectations

- The proposal MUST address common use cases within the domain of
  ovn-kubernetes. Flag as insufficient if the use case is too specific
  to a narrow subset of users without justification for why it cannot
  be generalized.
- Complex datapath changes MUST include diagrams showing flows across
  nodes, gateways, and OVN logical constructs.
- If the feature touches gateway paths, both local gateway (lgw) and
  shared gateway (sgw) modes MUST be addressed.
- The testing plan MUST cover interaction with existing features
  (NetworkPolicy, EgressIP, Services, UDN, etc.).
- The proposed design MUST be concrete and complete — no hand-waving
  or deferred details. Flag as incomplete if any aspect is left
  unresolved. Prefer simpler designs when they achieve the same goal.
- If the design introduces or modifies CRDs, it MUST follow Kubernetes
  API conventions: clear field naming, proper use of status vs spec,
  CEL validation rules, kubebuilder markers, sensible defaults,
  backward-compatible evolution,
  and no leaking of internal implementation details into the API
  surface.
- The Alternatives section MUST list at least one rejected design with
  a clear rationale for why it was discarded.
- The design MUST address performance and scale implications, especially
  CPU/memory tradeoffs (e.g. caching vs recomputation, watch overhead,
  additional OVN DB objects at scale). Flag as incomplete if missing.
- The design MUST address networking impact at the edge — e.g. BUM
  traffic flooding, ARP storms, broadcast domains growing with scale.
  Flag as incomplete if the OKEP does not analyze behavior at hundreds
  of nodes with thousands of pods.
- Content MUST be self-contained; external links are supplementary only.

## Governance

- The linked GitHub issue MUST use the enhancement issue template.
- The issue MUST be discussed in a community meeting to get consensus
  before starting an OKEP.
- Discussion MUST live on GitHub issue and OKEP PR, not Slack or
  meeting notes.
