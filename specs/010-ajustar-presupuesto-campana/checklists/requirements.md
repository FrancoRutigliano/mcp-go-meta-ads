# Specification Quality Checklist: Ajustar presupuesto de campañas

**Purpose**: Validate specification completeness and quality before proceeding to planning
**Created**: 2026-07-28
**Feature**: [spec.md](../spec.md)

## Content Quality

- [x] No implementation details (languages, frameworks, APIs)
- [x] Focused on user value and business needs
- [x] Written for non-technical stakeholders
- [x] All mandatory sections completed

## Requirement Completeness

- [x] No [NEEDS CLARIFICATION] markers remain — los 3 se resolvieron con el usuario (ver "Decisiones tomadas")
- [x] Requirements are testable and unambiguous
- [x] Success criteria are measurable
- [x] Success criteria are technology-agnostic (no implementation details)
- [x] All acceptance scenarios are defined
- [x] Edge cases are identified
- [x] Scope is clearly bounded
- [x] Dependencies and assumptions identified

## Feature Readiness

- [x] All functional requirements have clear acceptance criteria
- [x] User scenarios cover primary flows
- [x] Feature meets measurable outcomes defined in Success Criteria
- [x] No implementation details leak into specification

## Constitution Alignment (proyecto)

- [x] Principio I — Confidencialidad del token: FR-035
- [x] Principio II — Propose/Confirm: FR-001, FR-020, FR-026, FR-027
- [x] Principio IV — Auditoría: FR-029, SC-003
- [x] Principio V — Errores semánticos: FR-028, FR-033
- [x] Principio VII — Mensajes legibles: FR-032, FR-034
- [x] Principio VIII — Umbrales de negocio (ROAS 2x): FR-024, SC-007
- [x] Principio IX — Honestidad ante datos insuficientes: FR-025, SC-007

## Notes

- Iteración 1: la spec pasó todos los criterios de calidad salvo los 3 marcadores de clarificación
  (alcance de niveles, forma de expresar el cambio, límites de seguridad).
- Iteración 2: los 3 marcadores se resolvieron con el usuario. Se amplió el alcance a nivel conjunto
  de anuncios (nueva User Story 2, FR-007 a FR-010), se agregó el ajuste relativo (FR-011 a FR-015) y
  el doble guardrail configurable (FR-016 a FR-018). Checklist completo.
- **Listo para `/plan`.**

## Cobertura de los 35 FRs (T053, verificado 2026-07-29)

| FR | Dónde vive | Test |
|---|---|---|
| FR-001, FR-002 | `app.ProposeBudget.Execute` / `resolveTarget` | `TestProposeBudget_AbsoluteAmount`, `_NeverWrites` |
| FR-003, FR-004 | `mcp.formatBudgetProposal` | `TestProposeBudgetHandler_ProposesWithoutWriting`, `_AdSetProposal` |
| FR-005, FR-006 | `domain.BudgetChange.Validate/Apply` | `TestBudgetChangeValidate*`, `TestProposeBudget_Rejections` |
| FR-007, FR-008 | `resolveTarget` + `domain.BudgetLevelError` | `TestProposeBudget_CampaignLevelListsAdSets`, `_AdSetRejectedWhenCampaignControlsBudget` |
| FR-009, FR-010 | `app.GetBudgets` | `TestGetBudgets_*` |
| FR-011 a FR-015 | `domain.BudgetChange` | `TestBudgetChangeApply*`, `TestProposeBudget_RelativeChange` |
| FR-016 a FR-018 | `domain.Guardrails.Check` + `config.loadGuardrails` | `TestGuardrailsCheck`, `TestLoad_Guardrail*` |
| FR-019 | `formatBudgetProposal` (proyección mensual) | `TestBudgetsHandler_CampaignLevel` |
| FR-020, FR-026, FR-027 | `app.ConfirmProposal` | `TestConfirmProposal_*` |
| FR-021 | `domain.Campaign.Editable` / `AdSet.Editable` | `TestProposeBudget_Rejections` (campaña archivada) |
| FR-022 | `newProposalID` + `Proposal.Expired` | `TestProposeBudget_ProposalExpires`, `TestConfirmProposal_ExpiredRejected` |
| FR-023 a FR-025 | `ProposeBudget.attachPerformance` + `mcp.perfBlock` | `TestProposeBudget_AttachesPerformance`, `_FlagsLowROAS`, `_InsufficientDataDoesNotBlock` |
| FR-028 | `ConfirmProposal.Execute` (no consume si falla) | `TestConfirmProposal_KeepsBudgetProposalOnWriteFailure` |
| FR-029 | auditoría con `nivel` y `entidad_id` | `TestConfirmProposal_AppliesBudgetAndAudits`, `_AppliesAdSetBudgetOnly` |
| FR-030 | `ConfirmProposal.applyBudget` (relectura) | `TestConfirmProposal_RejectsDriftedBudget`, `_AdSetDriftRejected` |
| FR-031 | `applyBudget` ramifica por `Level` | `TestConfirmProposal_AppliesAdSetBudgetOnly` |
| FR-032, FR-033 | `mcp.budgetMessage` | `TestMessageForError_*`, `TestProposeBudgetHandler_GuardrailMessageIsHuman` |
| FR-034 | `mcp.formatMoney` | `TestFormatMoney` |
| FR-035 | el token nunca sale del adaptador `meta` | `TestUpdateCampaignBudget_PermissionErrorMapsToUnauthorized` |

**Sin cobertura directa**: ninguno. El único punto con implementación degradada es la detección del
presupuesto mínimo de Meta (FR-033), que se resuelve por texto en vez de por código — documentado en
`research.md` (R5) con sus tests y su limitación.

## Revisión de seguridad (T052, verificado 2026-07-29)

- Las **3** invocaciones al `MetaWriter` están dentro de `ConfirmProposal.apply` / `applyBudget`.
  Ningún otro caso de uso recibe un writer (`ProposeBudget` ni siquiera lo tiene como campo).
- El token aparece sólo en `config` (redactado en `String()`), en la construcción de la query del
  cliente Meta y en el cableado de `main.go`. El adaptador `meta` no tiene ninguna llamada a `slog`,
  así que no se loguean URLs ni queries.
- Cobertura ≥ 80 % en los 6 paquetes; `go vet` y `go test -race` limpios.
