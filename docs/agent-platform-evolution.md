# 云雀 Agent Platform Evolution

Updated: 2026-06-24

## External Signals

WorkBuddy emphasizes a scenario-based workbench: domain experts, a skills marketplace, remote control through IM channels, unified model/token management, object storage, deployment, monitoring, and common business skill calls.

OpenClaw emphasizes a self-hosted gateway: multi-channel chat surfaces, a browser control UI, multi-agent routing, per-agent workspaces and session stores, model-provider abstraction, skills/plugins/workflows, browser/exec/sandbox tools, and explicit security boundaries such as pairing, allowlists, session isolation, prompt-injection hardening, and sandbox profiles.

Sources checked during these slices:

- WorkBuddy official site: https://copilot.tencent.com/work/
- OpenClaw ecosystem hub: https://openclaw-ai.dev/

## Yunque Product Principles

1. Start with task entry points, not configuration walls.
2. Keep advanced power visible but folded behind progressive disclosure.
3. Treat every capability as a trust-managed asset: source, permissions, runtime, logs, and rollback.
4. Make model providers and channel connectors first-class, testable, and recoverable.
5. Move toward multi-agent operation through scoped workspaces, memory boundaries, approval policies, and AgentOps visibility.

## Current Mapping

| Pattern | Yunque Surface | Next Iteration |
| --- | --- | --- |
| Scenario workbench | `apps/web/src/app/settings/page.tsx`, `apps/web/src/app/packs/page.tsx` | Keep common entry points first; reduce dense setting copy. |
| Skills marketplace | `apps/web/src/app/packs/page.tsx`, `internal/agentcore/skillmarket` | Show install source, trust, and runtime status as actions, not paragraphs. |
| Model/provider abstraction | `apps/web/src/app/settings/providers/page.tsx`, `packages/yunque-client/src/*provider*` | Keep provider page routable, testable, and connection-test focused. |
| Multi-channel gateway | `apps/web/src/app/settings/connectors/page.tsx`, connector SDK modules | Add channel status, pairing, and allowlist states. |
| Multi-agent routing | `internal/agentcore/session`, `internal/agentcore/planner`, `packages/yunque-client/src/agent-kit.ts` | Introduce per-agent workspace/session summaries before exposing raw config. |
| AgentOps/security | approvals, audit, sandbox, trust modules | Surface pending approvals, risky tools, sandbox mode, and audit trails together. |

## First Applied Slice

This iteration prioritized reliability and UX trust:

- Restored `/settings` and `/settings/providers` to routable, type-safe HeroUI v3 surfaces.
- Reintroduced a low-density `常用入口` area before advanced configuration.
- Fixed settings navigation semantics with a labeled nav and current-page state.
- Fixed chat starter chips so screen readers hear the short action name while descriptions remain available.
- Migrated v2-style HeroUI usages in touched settings code to v3 compound patterns.

## Second Applied Slice

This iteration added the first AgentOps control tower:

- Added `/agentops` as a low-density overview for backend health, providers, approvals, tasks, audit integrity, desktop sandbox posture, packs, trust, memory, goroutines, and monthly cost.
- Kept every data source independently recoverable: a failed provider, pack, audit, or health request degrades only its chip/card instead of crashing the page.
- Linked the overview into the existing deep surfaces: providers, approvals, tasks, audit, trust, packs, desktop sandbox, and workers.
- Added the `nav-agentops` control-plane entry and localized label so the route is discoverable when the control-plane pack is enabled.
- Covered the happy path and partial API failure path with focused tests.

## Third Applied Slice

This iteration reduced Pack card reading load:

- Added a default-visible Pack summary strip with four facts: source, runtime state, trust/risk, and delivery readiness.
- Kept long permission explanations, install checklists, troubleshooting, and Xiaoyu polishing details inside maintenance/expanded surfaces.
- Made official/private/installable sources visible before expansion so users can judge origin before installing.
- Updated Packs tests to verify installed and installable cards expose the new low-density trust summary.

## Fourth Applied Slice

This iteration made model-provider settings more recoverable:

- Added a first-screen ProviderOps overview that summarizes access mode, enabled chat models, execution provider, and Tori status before detailed configuration tabs.
- Turned missing model/Tori/execution readiness into direct recovery actions that open the relevant existing tab.
- Kept API keys, breaker state, provider details, presets, routing, and Tori account management in progressive detail surfaces instead of expanding the settings wall.
- Added focused provider-page tests for a ready provider setup and an empty setup recovery path.
- Added a jsdom `getAnimations()` test polyfill for HeroUI/React Aria shared transitions.

## Fifth Applied Slice

This iteration made connector safety visible before configuration:

- Added a first-screen connector safety overview for browser pairing, third-party app authorization, broken credentials, and the currently open action surface.
- Linked the browser recovery action directly to the Browser Pack instead of leaving users to infer where pairing happens.
- Turned failed connector credentials into a direct "view attention" path that selects the affected connector for repair.
- Replaced the hand-written monitor SVG with the shared Lucide icon set.
- Added focused connector-page tests for healthy safety state and browser-pairing recovery.

## Sixth Applied Slice

This iteration turned AgentOps status into a recovery path:

- Added a Recovery Queue that orders pending approvals, failed tasks, audit-chain breaks, and partially unavailable AgentOps data sources into one repair surface.
- Kept each recovery item linked to the existing deep page: approvals, missions, audit, or AgentOps refresh context.
- Preserved the low-density metrics while removing the old approval-only "attention" card that hid task and audit recovery.
- Added focused AgentOps tests for the combined recovery queue, including failed tasks and audit break positions.

## Seventh Applied Slice

This iteration made per-agent boundaries visible before deeper routing work:

- Added an Agent Boundary panel to `/agentops` that combines workspace/project, worker, external session, and desktop sandbox state in one low-density control surface.
- Reused the existing `listWorkers`, `listProjects`, `orchestratorSessions`, task, and desktop sandbox APIs instead of adding a new backend protocol.
- Kept each boundary row action-oriented and linked to the existing deep surface: projects, workers, or the computer-use pack.
- Covered the healthy boundary state in AgentOps tests, including project, worker, session, and sandbox visibility.

## Eighth Applied Slice

This iteration made failed-task recovery more specific:

- Added inferred recovery hints for failed tasks in `/agentops` based on the task error/title/description.
- Routed common failure classes to the most useful repair surface: approvals, model providers, connectors, desktop sandbox, or task detail.
- Kept the Recovery Queue compact while changing failed-task rows from passive status into direct repair actions.
- Covered connector-related task failures in tests so a browser/connector error now links to connector settings instead of the generic missions page.

## Ninth Applied Slice

This iteration moved recovery hints into the backend task platform:

- Added a structured `recovery_hint` contract to tasks with category, severity, summary, detail, primary action, optional secondary actions, and source.
- Added deterministic backend inference for common failure classes: approval, provider, connector, sandbox, and generic task recovery.
- Made the work pack attach computed recovery hints to `/v1/tasks` create/list/detail responses without rewriting old task JSON files.
- Updated AgentOps to prefer backend-provided recovery actions while keeping the frontend fallback for older backends.
- Added Go tests for recovery classification and work-pack task-list output, plus AgentOps coverage for backend recovery hints.

## Tenth Applied Slice

This iteration preserved recovery context at the task failure site:

- Added `InferRecoveryHint(task, source)` so runners can store a fresh hint with an explicit source instead of relying only on API-time inference.
- Made task runner failures write source-aware hints for planning failures, single-step failures, parallel-group failures, and dependency-blocked interruptions.
- Cleared stale recovery hints on resume, restart, pause, and cancellation so new execution attempts do not inherit old repair guidance.
- Added interrupted-task recovery hints during store startup recovery for tasks left running/planning across process restarts.
- Covered runner-stored hints, startup recovery hints, and stale-hint clearing in task lifecycle tests.

## Eleventh Applied Slice

This iteration made capabilities visible as a low-density graph:

- Added a lightweight Capability Graph to `/agentops` that links model/provider, connector, tool/pack, skill, memory, approval/trust, audit, and runtime worker surfaces.
- Reused existing provider, connector, skill, pack, memory, worker, approval, trust, and audit APIs instead of adding a separate topology protocol.
- Kept each node as a native link with a short status line and one status chip, so AgentOps stays scan-first rather than becoming another settings wall.
- Covered the graph state and deep links in AgentOps tests, including connected/error connectors, registered skills, memory tiers, and runtime worker readiness.

## Twelfth Applied Slice

This iteration made planner recovery more step-specific:

- Added structured `failure_summary.failed_steps` to `/v1/planner/execution-state`, including step id, action, skill, friendly error, and per-step recovery recommendation.
- Kept raw planner/provider diagnostics sanitized while preserving enough context for a user to see which step/tool failed.
- Updated the Planner Recovery detail UI to show at most three failed steps as a compact semantic list under the unified execution scene.
- Covered the backend JSON contract and frontend rendering in planner recovery tests.

## Thirteenth Applied Slice

This iteration made connector sources functional and trust-readable:

- Added UI-safe connector `last_event` tracking in the connector registry for connect, OAuth connect, restore, disconnect, token refresh, and action execution outcomes.
- Exposed connector action allowlists through `/api/connectors` and `/api/connectors/detail` as `allowed_actions` and `allowlist_count`.
- Updated connector settings so the safety overview reports allowlist size and the selected connector shows a compact "能力边界 / 最近事件" block.
- Kept credentials and request parameters out of event records, preserving operational evidence without leaking secrets.
- Covered registry event tracking, connector-pack API output, and settings-page rendering in tests.

## Fourteenth Applied Slice

This iteration made Pack route/capability problems visible before users hit broken paths:

- Added a reusable `packManifestAudit` model for static Pack manifest checks: unknown `/packs/*` entry routes, missing `routeSpecs`, capability/permission mismatch, and iframe bundle whitelist gaps.
- Promoted audit-blocking Packs into the Xiaoyu polishing queue with P0 priority, so 404-prone entries and route whitelist gaps are fixed before wording polish.
- Added a low-density Manifest Audit summary to maintenance mode and Pack detail pages without increasing default Pack-card noise.
- Included audit findings in the copied Pack usability report and Studio/Chat handoff context.
- Covered the audit model, Packs center queue, and Pack detail rendering in focused tests.

## Fifteenth Applied Slice

This iteration merged runtime Pack route audit into the low-density Pack maintenance flow:

- Extended `packManifestAudit` to accept backend route audit entries and surface missing mounted routes, method mismatches, undeclared mounted routes, registry unavailability, and unknown runtime issues.
- Loaded `/v1/packs/backend-route-audit` only when Pack maintenance/advanced mode is visible, keeping the default Pack center lightweight.
- Promoted runtime route audit failures into the Xiaoyu polishing queue as P0 audit blockers and included them in copied reports and batch Studio/Chat handoff payloads.
- Added maintenance UI state for runtime audit loading, refresh, failure, and issue count.
- Covered runtime route audit merging in presentation tests and Packs center queue tests.

## Sixteenth Applied Slice

This iteration restored the local web shell route surface:

- Traced app-level 404s on `/chat`, `/packs`, `/settings/*`, `/login`, and `/setup` to the `next dev --turbopack` path under the current Next 16.2.4 desktop workspace.
- Switched the web dev script to `next dev --webpack --port 3001`, keeping local App Router pages reachable while retaining the existing Next config for future Turbopack re-evaluation.
- Verified the critical UX routes return 200 in dev: `/`, `/chat`, `/dashboard`, `/settings`, `/settings/providers`, `/settings/connectors`, `/packs`, `/packs?maintenance=1`, `/packs/detail?id=yunque.pack.backup`, `/login`, and `/setup`.

## Seventeenth Applied Slice

This iteration reduced Pack filter density and made source filtering meaningful:

- Reworked the Pack filter modal around HeroUI v3 Modal/Disclosure composition: search, current result summary, recommended next action, and common filters stay first-screen; risk, stability, readiness, sort, and detailed stats stay behind "More filters".
- Renamed the source filter surface to "source trust" and made installed, official source, and private source each explain what the user should do next: enable/check, install with rollback awareness, or inspect before enabling.
- Kept active filters as dismissible action chips and preserved the existing detailed source/delivery/current-view stats inside the advanced disclosure.
- Updated focused Packs tests to cover the clearer official-source button label while preserving source/install filtering behavior.

## Eighteenth Applied Slice

This iteration pulled Pack audit blockers into AgentOps recovery:

- Loaded `/v1/packs/backend-route-audit` alongside installed Packs in the AgentOps snapshot, with independent failure handling so the control tower still renders when the audit source is unavailable.
- Merged runtime route audit entries through the existing `packManifestAudit` model and promoted blocked Packs into the Recovery Queue.
- Linked each Pack audit recovery item directly to the Pack detail page so users can inspect the broken route, missing method, undeclared mount, or static entry problem without first discovering maintenance mode.
- Added a compact Pack audit chip and Capability Graph signal only when blockers exist, preserving the low-density default state.
- Covered the runtime route-audit recovery path in AgentOps tests.

## Nineteenth Applied Slice

This iteration made connector incidents visible from AgentOps:

- Promoted connector `error` states and failed `last_event` crumbs into the AgentOps Recovery Queue.
- Linked connector recovery rows to `/settings/connectors?focus=<id>` so the settings page opens the affected connector instead of leaving the user to hunt through the list.
- Updated the Capability Graph connector node to count recoverable connector incidents, including action execution failures where the connector still reports `connected`.
- Kept successful connector events out of AgentOps, preserving the scan-first control tower while still making broken channels actionable.
- Covered status-error and last-event-error paths in AgentOps tests, plus focused connector deep-link behavior in settings tests.

## Twentieth Applied Slice

This iteration turned Pack audit recovery into a repair action:

- Changed AgentOps Pack audit recovery rows from detail-page inspection links into Xiaoyu Studio repair links.
- Passed `packId` and a scoped repair `goal` into `/packs/studio`, preserving the existing Studio workflow instead of adding another repair surface.
- Included artifact `packagePath` and `sha256` when present so Studio can start closer to a safe yqpack inspect/workspace flow.
- Kept the Recovery Queue compact: the row still shows the first blocker, but the primary action now starts the repair plan directly.
- Covered the direct Studio handoff in AgentOps tests.

## Twenty-first Applied Slice

This iteration made multi-agent policy boundaries visible before richer routing metadata exists:

- Added a compact Strategy Boundary panel to `/agentops` that shows workspace default capabilities, worker execution capacity, session adapters, and tenant/task scope.
- Reused existing project, worker, session, and task fields instead of inventing a new policy API before the backend is ready.
- Linked each policy row to the existing deep surface: projects, workers, or missions, keeping AgentOps as a scan-and-jump control tower.
- Kept the copy short and status-chip based so AgentOps does not become another dense settings page.
- Covered the healthy policy state in AgentOps tests, including default caps, worker capacity, session adapter, tenant scope, and deep links.

## Twenty-second Applied Slice

This iteration reduced the default settings-page density:

- Added first-screen entries for Connectors and AgentOps so channel health, authorization, recovery queues, boundaries, and audit are reachable without hunting through configuration tabs.
- Changed the advanced config browser from default-visible to opt-in with an explicit "open parameter table" control.
- Kept the tier selector visible but folded the schema sidebar and field list until the user asks for low-level parameters.
- Added accessible expanded/collapsed state through `aria-expanded` and `aria-controls` on the HeroUI button.
- Covered the new default state in settings tests, including quick-entry links and the hidden-until-open config browser.

## Twenty-third Applied Slice

This iteration made Pack source information functional instead of decorative:

- Renamed the default Pack summary tile from passive source display to source verification.
- Kept the original source value visible while adding the next action directly in the tile: installed Packs can be disabled/rolled back, official-source Packs should be read-only inspected before install, and private-source Packs should have SHA/permissions checked.
- Avoided adding another modal or default-expanded detail area, keeping the Pack card scan-first.
- Covered installed, official-source, and private-source cards in Packs tests so the source tile now proves an action hint exists.

## Twenty-fourth Applied Slice

This iteration promoted Pack source failures into AgentOps:

- Loaded the Pack catalog report in the AgentOps snapshot as an independently recoverable data source.
- Turned failed `source_reports` and catalog-level errors into Recovery Queue items linked to `/packs?maintenance=1`.
- Updated the Capability Graph Tool/Pack node to show source issues when route audit is otherwise clean.
- Kept AgentOps resilient: catalog failure degrades like other data sources instead of blocking the page.
- Covered source-report failures in AgentOps tests, including the recovery link and graph status line.

## Twenty-fifth Applied Slice

This iteration made the AgentOps Recovery Queue easier to scan before clicking into details:

- Added lane summaries that group recoverable incidents by action class, such as approvals, connectors, Pack sources, Pack audits, and audit-chain breaks.
- Preserved item-level recovery links while moving repeated same-class incidents into compact HeroUI status chips at the top of the queue.
- Normalized connector-related failed tasks and connector health incidents into the same lane, matching the action users need to take.
- Kept the summary non-interactive so every visible control still has a real destination.
- Covered the grouped queue summaries in AgentOps tests for healthy, failed-task, and Pack-source failure states.

## Twenty-sixth Applied Slice

This iteration made model-provider settings more action-first:

- Added a compact ProviderOps recovery path that appears only when model access, execution routing, or Tori mode has blocking work.
- Deduplicated repeated recovery actions by destination, so an empty setup shows one "add provider" action instead of multiple repeated prompts.
- Kept the controls as real HeroUI buttons that switch to the relevant tab: presets, routing, or Tori.
- Preserved the healthy first screen by hiding the recovery path when there is nothing to fix.
- Covered Tori recovery and empty-provider recovery paths in provider settings tests.

## Twenty-seventh Applied Slice

This iteration connected actual model-provider readiness back into AgentOps:

- Loaded the real provider registry in AgentOps instead of relying only on provider presets.
- Promoted missing or unusable chat providers into Recovery Queue items linked directly to `/settings/providers`.
- Updated the Capability Graph model node to show ready chat providers, total provider count, preset count, and model recovery state.
- Added a top-level model incident chip when model access blocks Chat, Planner, or execution agents.
- Covered healthy provider readiness and empty-provider recovery in AgentOps tests.

## Twenty-eighth Applied Slice

This iteration made backend task recovery classification more precise:

- Added provider-specific recovery patterns for API authentication failures, quota/balance exhaustion, and rate limits.
- Ensured provider-context `401/403 unauthorized` errors route to `/settings/providers` instead of being misclassified as approval failures.
- Marked quota and authentication failures as danger severity while keeping rate-limit recovery warning-level.
- Preserved the existing structured `recovery_hint` contract so AgentOps and clients keep consuming the same API shape.
- Covered provider auth, quota, and rate-limit classification in task recovery tests.

## Twenty-ninth Applied Slice

This iteration aligned AgentOps fallback recovery with the backend classifier:

- Added provider-specific auth, quota, and rate-limit checks before the generic approval matcher in the frontend failed-task fallback.
- Prevented old tasks without stored `recovery_hint` from routing provider `401/403 unauthorized` failures to the approvals page.
- Kept all model recovery links pointed at `/settings/providers`, matching the backend `recovery_hint` contract.
- Covered provider-auth fallback behavior in AgentOps tests, including the recovery lane summary and destination link.

## Thirtieth Applied Slice

This iteration made connector recovery more precise and less text-heavy:

- Added a shared connector recovery classifier for browser pairing, credential/OAuth expiry, allowlist boundary failures, upstream/network issues, and rate limits.
- Updated AgentOps connector recovery rows to use specific titles, summaries, severity, and destinations instead of a generic "connector needs attention" message.
- Added a compact recovery callout to the connector settings detail panel, shown only for the selected broken connector.
- Kept Allowlist issues as explanation-only when there is no real one-click repair, avoiding decorative controls that imply unsupported functionality.
- Covered the classifier, AgentOps recovery row, and connector settings recovery hint with focused tests.

## Thirty-first Applied Slice

This iteration aligned failed-task recovery with connector incident precision:

- Added backend task recovery patterns for connector Allowlist failures, expired credentials/OAuth authorization, and connector rate limits before the generic approval matcher.
- Tightened provider quota/rate-limit patterns so connector `429` and similar external-app errors do not get misclassified as model-provider incidents.
- Updated AgentOps frontend fallback recovery for older tasks without stored `recovery_hint`, keeping connector auth failures out of the approval lane.
- Preserved the existing `recovery_hint` API contract while improving category, severity, summary, and repair destination.
- Covered backend connector-specific recovery and frontend fallback routing with focused tests.

## Thirty-second Applied Slice

This iteration turned connector task recovery into deep repair links:

- Added backend recovery action refinement for connector failures so Browser/extension/pairing errors open `/packs/browser`.
- Added connector-id focus inference for common app connectors such as GitHub, Gmail, Slack, Notion, Linear, and Jira, producing `/settings/connectors?focus=<id>` from task failure text.
- Updated AgentOps fallback recovery to use the same focused connector and browser-pack destinations for old tasks without stored backend hints.
- Kept browser-pack recovery grouped under the connector lane so the Recovery Queue remains scan-first instead of fragmenting action classes.
- Covered browser-pack and focused connector recovery destinations in backend and AgentOps tests.

## Thirty-third Applied Slice

This iteration made Planner failed-step recovery directly routable:

- Added structured `recovery_target` metadata to `/v1/planner/execution-state` failed steps.
- Routed provider failures to `/settings/providers`, browser pairing failures to `/packs/browser`, focused connector failures to `/settings/connectors?focus=<id>`, and dependency failures to the checkpoint dependency view.
- Added a summary-level `primary_target` so AgentOps/detail UI can surface the first concrete repair path without re-parsing error strings.
- Updated AgentOps to load recent recoverable Planner checkpoints, read execution-state, and promote `primary_target` into the Recovery Queue.
- Updated Planner checkpoint detail failed-step rows to show compact direct repair links from `recovery_target`.
- Preserved raw diagnostic sanitization while keeping the friendly failed-step recommendation and target action separate.
- Covered provider, connector, browser, and dependency recovery targets in gateway tests, plus AgentOps recovery queue, Planner detail links, and frontend type coverage.

## Thirty-fourth Applied Slice

This iteration reduced Pack maintenance-card density and removed a fake affordance:

- Converted installed Pack maintenance details from always-expanded copy into a real HeroUI disclosure-style button.
- Kept default Pack cards focused on status, trust, and real actions while moving long usage, validation, entry, limitation, and polish guidance behind "展开详情".
- Added `aria-expanded`/`aria-controls` state so assistive technology gets the same expanded/collapsed state as sighted users.
- Preserved the existing installable-card detail pattern so installed and installable Pack cards now behave consistently in maintenance mode.
- Covered collapsed and expanded installed-card details in Packs tests, including experimental Pack recovery copy and the visible "收起详情" state.

## Thirty-fifth Applied Slice

This iteration removed another default Pack-card debug detail without hiding the real action:

- Hid installed Pack `入口 /path` chips from the default Pack center card so users see the real open action instead of raw route text.
- Kept the same route chip visible in advanced/maintenance mode, where it is useful for diagnosis, QA, and Pack polish work.
- Preserved the existing `打开`/primary action button and detail links, so the default view loses text density but not capability.
- Covered the default-hidden and maintenance-visible route chip behavior in Packs tests.

## Thirty-sixth Applied Slice

This iteration made the Settings landing page more control-tower-like:

- Removed repeated explanatory descriptions from the default "常用入口" cards.
- Kept each card as a real navigation link with a visible title and concrete action label, preserving the recovery/control destinations.
- Left first-run setup guidance and the advanced parameter browser intact, so onboarding and diagnostics remain available when needed.
- Covered the scan-first Settings entry cards in Settings page tests.

## Thirty-seventh Applied Slice

This iteration kept the AgentOps control tower aligned with completed recovery work:

- Replaced the stale "下一步" roadmap card that still mentioned Planner recovery links already delivered.
- Reframed the sidebar card as "平台焦点", limited to current work that changes recovery routing or default-view density.
- Kept it non-interactive because no direct backend write action exists yet for policy guardrails or grouped incident history.
- Covered the refreshed AgentOps focus copy and the absence of the stale Planner item in AgentOps tests.

## Thirty-eighth Applied Slice

This iteration made AgentOps recovery lane summaries match structured Planner targets more closely:

- Added lane labels for `/skills`, `/tools`, and `/planner-checkpoint` recovery destinations instead of falling back to generic labels.
- Mapped Planner `skill` and `tool` recovery target categories to "技能" and "工具" in the Recovery Queue.
- Preserved the existing recovery deep links while making the control-tower summary tell operators what surface actually needs repair.
- Covered Planner tool recovery in AgentOps tests, including the `/tools` destination and "工具" lane count.

## Thirty-ninth Applied Slice

This iteration extended backend failed-task recovery to capability surfaces:

- Added backend task recovery patterns for missing/disabled skills and unavailable tools.
- Routed skill failures to `/skills` and tool failures to `/tools` instead of falling back to generic task-detail recovery.
- Kept provider, connector, approval, and sandbox patterns ahead of generic fallback so existing recovery routes remain stable.
- Covered skill and tool recovery hints in `internal/agentcore/task` tests.

## Fortieth Applied Slice

This iteration aligned AgentOps fallback recovery for older failed tasks:

- Added frontend fallback classification for legacy failed tasks that mention missing/disabled skills or unavailable tools but do not have stored backend `recovery_hint`.
- Routed skill failures to `/skills` and tool failures to `/tools`, matching the backend task recovery hints.
- Kept connector/provider-specific checks ahead of these fallback patterns so OAuth, allowlist, and model incidents keep their more precise destinations.
- Covered both fallback paths in AgentOps tests, including the "技能" and "工具" recovery lanes.

## Forty-first Applied Slice

This iteration tightened Planner failed-step recovery routing for capability failures:

- Added structured failed-step summaries to `/v1/planner/execution-state`, including per-step recommendations and recovery targets.
- Routed Planner provider, connector, browser, dependency, skill, and tool failures to their concrete repair surfaces.
- Kept missing/unknown skills and tools ahead of browser-name matching so `browser_extract`-style tool names still open `/tools`.
- Covered Chinese unknown-tool failures and browser-named missing tools in gateway tests.

## Forty-second Applied Slice

This iteration reduced Planner recovery detail density:

- Hid legacy `ruled_out` diagnostic text when structured failed steps are available.
- Kept failed steps focused on step identity, capability surface, and the direct repair link when `recovery_target.href` exists.
- Preserved diagnostic error/recommendation text only for failed steps without a direct recovery target.
- Covered the compact direct-repair state in Planner checkpoint detail tests.

## Forty-third Applied Slice

This iteration made the model-provider recovery landing more action-first:

- Replaced the multi-label provider recovery sentence with a single primary "下一步" target.
- Kept only one real recovery button visible in the recovery callout, with a compact count for remaining blockers.
- Removed the duplicate footer action when the callout already exposes the same repair path.
- Covered Tori-binding and empty-provider recovery states in provider settings tests.

## Forty-fourth Applied Slice

This iteration reduced dynamic Pack route recovery density:

- Shortened the "current entry" copy so it names the path, the permission check, and the Xiaoyu handoff without long diagnostic prose.
- Replaced "source/filter" language with concrete destinations: permission detail, Pack center location, and Pack Studio polish.
- Removed duplicate detail and center links from the lower recovery card, leaving the top actions and one real "Xiaoyu polish" action.
- Tightened the acceptance note from a verbose exit path into a compact verification line for status, permissions, and entry retest.
- Covered the shorter copy, removed duplicate links, and absence of stale "source/filter" wording in the dynamic Pack route test.

## Forty-fifth Applied Slice

This iteration aligned Chat Pack handoff cards with the shorter Pack recovery language:

- Replaced the batch Pack card's verbose "acceptance exit" sentence with the same compact status-permission-entry verification path used on Pack routes.
- Kept the real detail, center, and open-entry links intact so the Chat card still hands operators to concrete recovery surfaces.
- Removed the stale "验收出口" wording from the Chat rendering test and covered the shorter no-entry fallback.

## Forty-sixth Applied Slice

This iteration aligned Pack detail verification with the same compact recovery wording:

- Replaced the detail page's verbose "验收出口" phrase with a short "验收" label.
- Kept the recovery path focused on concrete surfaces: Pack center status, detail permissions, and entry retest when available.
- Shortened the no-entry fallback so backend-only Packs point to Chat/task/memory/knowledge triggering without extra observation prose.
- Covered the shorter detail-page verification copy and absence of the stale label in Pack detail tests.

## Forty-seventh Applied Slice

This iteration reduced Pack center maintenance-queue recovery wording:

- Replaced the readiness queue's "验收出口" summary with a short "验收" card that names entry retest or Chat/task/memory/knowledge triggering.
- Renamed queue detail actions from "先看权限与来源" to "权限与详情" while keeping the same concrete Pack detail destination.
- Shortened the Studio-return fallback copy from "回中心确认状态" to "中心看状态" and kept the real detail/open/studio actions intact.
- Covered the shorter queue summary, return-card copy, and new detail-link label in Packs page tests.

## Forty-eighth Applied Slice

This iteration aligned Pack Studio with the compact Pack recovery language:

- Replaced Studio handoff labels from "验收出口" / "最终验收出口" with shorter "验收" / "最终验收" labels.
- Shortened Studio verification copy to "中心看状态，详情复查权限" while preserving the real center, detail, and open-entry links.
- Renamed generated delivery-summary and post-install actions from "权限与来源详情" / "查看权限与来源" to "权限与详情".
- Covered the shorter Studio wording and absence of stale long labels in Pack Studio tests.

## Forty-ninth Applied Slice

This iteration removed the last Pack empty-state "source filter" wording:

- Replaced Pack Studio's empty candidate state from "来源筛选" to "来源信任", matching the real filter dimension.
- Replaced Pack Center's installed-list empty state with the same "来源信任" label.
- Covered both empty states so the UI no longer drifts back to generic source-filter language.

## Fiftieth Applied Slice

This iteration reduced connector-settings fallback copy:

- Shortened unsupported connector guidance from a long explanation into two facts: no server handler yet, and coming-soon connectors cannot connect.
- Kept the existing "即将开放" state and avoided adding any fake connection action.
- Covered the unsupported connector detail panel in settings tests.

## Fifty-first Applied Slice

This iteration tightened AgentOps card descriptions:

- Replaced default-visible explanatory Card descriptions with short object lists for recovery, boundaries, policy, capability graph, run control, resources, and platform focus.
- Kept all recovery links, counts, lane summaries, and capability graph destinations unchanged.
- Covered the compact AgentOps descriptions and absence of stale explanatory copy in AgentOps tests.

## Fifty-second Applied Slice

This iteration polished `/tools` as a real recovery target for tool failures:

- Added a compact PageHeader description that frames the page as command, background-session, and output verification.
- Localized the CWD placeholder, background toggle, and running-session tooltips while keeping all controls tied to real execution actions.
- Exposed the background mode as a stable toggle button with `aria-pressed` and visible "后台" labeling.
- Added visible labels for the working-directory and command inputs so the terminal surface is easier to scan and operate with assistive tech.
- Covered the compact recovery surface and background execution path in a focused Tools page test.

## Fifty-third Applied Slice

This iteration polished `/skills` as a real recovery target for missing or disabled skills:

- Added a compact PageHeader description that names the real recovery paths: scan local skills or install community skills.
- Kept the existing installed, market, and dynamic-skill surfaces intact while shortening search placeholders.
- Added visible labels for installed-skill search, market search, and GitHub `owner/repo` installation.
- Added current-state semantics to sort, category, and market-source filter buttons without turning them into fake settings.
- Covered the compact skill recovery surface, market-source state, and GitHub install path in a focused Skills page test.

## Fifty-fourth Applied Slice

This iteration aligned AgentOps capability navigation with the new recovery surfaces:

- Split the previous "Tool / Pack" capability row into a real `/tools` Tool row and a separate `/packs` Pack row.
- Kept Pack route/source audit status on the Pack row instead of overloading it as the generic tool recovery destination.
- Added compact Tool copy for command execution, background sessions, and output verification.
- Updated the capability graph description and readiness chip language so the API-backed readiness count stays honest.
- Covered the separate Tool and Pack links in AgentOps tests.

## Fifty-fifth Applied Slice

This iteration reduced the default settings landing copy:

- Shortened the page subtitle to focus on the real recovery order: model, channel, and runtime entry first.
- Compressed the common-entry helper into a scan-first object list instead of an explanatory sentence.
- Shortened the advanced-config helper so the parameter table remains clearly secondary.
- Kept all existing links and advanced controls intact; no fake preferences or settings were added.
- Covered the shorter wording and absence of the previous long helper copy in Settings page tests.

## Fifty-sixth Applied Slice

This iteration tightened the Pack Center Studio-return recovery copy:

- Shortened the return-state sentence to focus on permission/status review, entry retest, and rollback or Studio follow-up.
- Kept the existing real Pack detail, open-entry, and Studio actions unchanged.
- Added a regression check so the older long source/permission/delivery sentence does not return to the default view.

## Fifty-seventh Applied Slice

This iteration trimmed the Pack Center installable-source notice:

- Replaced the longer filter explanation with a short source-trust prompt for official, private, or local sources.
- Kept the existing source expansion control and installable-source diagnostics unchanged.
- Added a regression check so the older "current filter" phrasing does not return to the default view.

## Fifty-eighth Applied Slice

This iteration reduced Pack maintenance queue copy:

- Shortened the readiness overview to the blocking rule: P0 blocks acceptance, P1/P2 need purpose, entry, or boundary polish.
- Compressed the polishing queue summary to ordering, P0 status, and current batch counts.
- Kept pagination, Pack detail links, Studio handoff, Chat handoff, and JSON report actions unchanged.

## Fifty-ninth Applied Slice

This iteration tightened Pack maintenance diagnostics:

- Shortened Manifest Audit guidance to the concrete checks: entry routes, routeSpecs, capabilities, permissions, and runtime route blockers.
- Compressed delivery-distribution helper copy so it reads as an acceptance signal instead of a paragraph.
- Kept all audit refresh, source diagnostics, Studio/Chat handoff, and report-copy controls unchanged.

## Sixtieth Applied Slice

This iteration reduced provider-settings copy:

- Shortened the model operations overview details for Chat/Planner readiness, execution models, Tori state, and pending recovery.
- Tightened first-run, Tori mode, model-routing, and empty-preset helper text without changing any tab, bind, refresh, or add-provider actions.
- Covered the shorter overview states in provider settings tests so the old explanatory copy does not return.

## Sixty-first Applied Slice

This iteration reduced connector-settings copy:

- Shortened connector safety overview details for browser pairing, app authorization, credential incidents, allowlist surface, and footer recovery state.
- Tightened unsupported connector guidance while keeping the coming-soon state honest and without adding fake connection actions.
- Covered the shorter safety and unsupported states in connector settings tests.

## Sixty-second Applied Slice

This iteration tightened connector detail copy:

- Shortened the selected connector allowlist summary while keeping the action chips and backend handler distinction.
- Compressed the usage hint into a direct chat invocation example.
- Kept token entry, reconnect, recovery callout, recent event, and unsupported handler behavior unchanged.

## Sixty-third Applied Slice

This iteration tightened AgentOps control-tower copy:

- Shortened the AgentOps page subtitle into a scan-and-recover statement.
- Compressed the platform focus card to three current priorities: grouped incidents, editable guardrails, and lower default-view density.
- Kept recovery queue generation, lane summaries, capability graph links, and deep recovery destinations unchanged.

## Sixty-fourth Applied Slice

This iteration tightened AgentOps recovery queue copy:

- Shortened the recovery queue description from failed-task prose to a compact lane list.
- Compressed the empty and overflow helper text so the queue stays scan-first.
- Kept item ordering, lane chips, severity labels, and deep recovery links unchanged.

## Sixty-fifth Applied Slice

This iteration added AgentOps recovery queue guardrails:

- Added focused tests for the compact empty recovery state.
- Added focused tests for overflow messaging when more than six recovery items are present.
- Kept the existing recovery item generation and deep links unchanged while making the short copy harder to regress.

## Sixty-sixth Applied Slice

This iteration tightened Planner and AgentOps recovery guardrails:

- Adjusted the AgentOps overflow regression to use a mixed recovery queue: approvals, failed tasks, connector recovery, and audit breakage.
- Extended Planner failed-step recovery targeting so allowlist failures route to the connector settings recovery surface.
- Added a focused backend regression for allowlist-to-connector targeting while preserving provider, connector, browser, dependency, skill, and tool targets.

## Sixty-seventh Applied Slice

This iteration tightened the Planner-to-AgentOps recovery bridge:

- Added a focused AgentOps regression for Planner connector allowlist failures.
- Verified connector-category Planner recovery items land in the connector lane instead of the generic Planner lane.
- Verified the recovery link keeps the focused connector destination, for example `/settings/connectors?focus=github`.

## Sixty-eighth Applied Slice

This iteration fixed Planner dependency recovery grouping in AgentOps:

- Added a regression where `failed_steps[].recovery_target` is used when `primary_target` is absent.
- Fixed dependency recovery links such as `/planner-checkpoint?...#dependency-view` so the recovery queue groups them under the dependency lane instead of generic Planner.
- Kept the deep link pointed at the checkpoint dependency view, preserving the real recovery target.

## Sixty-ninth Applied Slice

This iteration tightened desktop sandbox recovery guardrails:

- Added backend coverage that desktop sandbox / Computer Use failures produce a `sandbox` recovery hint.
- Verified the backend primary action points to `/packs/computer-use`.
- Extended AgentOps legacy failed-task coverage so sandbox failures show in the sandbox lane and link to the Computer Use pack instead of browser or connector recovery.

## Seventieth Applied Slice

This iteration unified sandbox recovery grouping in AgentOps:

- Fixed `/packs/computer-use` recovery items so they always group under the sandbox lane.
- Added coverage for backend-provided sandbox recovery hints whose action label is `检查桌面沙箱`.
- Prevented sandbox incidents from splitting into an action-label lane such as `检查桌面沙箱`.

## Seventy-first Applied Slice

This iteration unified approval recovery grouping:

- Added backend coverage that approval-required task failures produce an approval recovery hint.
- Fixed `/approvals` recovery items so backend action labels such as `处理审批` still group under the approval lane.
- Added AgentOps coverage to prevent approval task recoveries from splitting into an action-label lane.

## Seventy-second Applied Slice

This iteration tightened AgentOps scan labels:

- Renamed the model-provider operation chip from the generic `连接` label to `模型` for consistency with the capability graph.
- Kept the running-control model entry aligned with the recovery queue and capability graph labels.
- Re-ran the AgentOps, task recovery, and typecheck gates after the label cleanup.

## Seventy-third Applied Slice

This iteration corrected the AgentOps recovery queue description:

- Replaced the outdated `审批、任务、审计、数据源。` helper with `审批、任务、通道、能力。`.
- Kept the description short while matching the real recovery lanes now produced by Planner, connectors, providers, Packs, sandbox, and approvals.
- Added a regression so the older, narrower lane description does not return.

## Seventy-fourth Applied Slice

This iteration tightened the AgentOps capability graph description:

- Replaced the long capability enumeration with `模型、通道、能力、证据、Worker。`.
- Kept the visible graph rows and deep links intact while making the header easier to scan.
- Added a regression so the older nine-item enumeration does not return to the default view.

## Seventy-fifth Applied Slice

This iteration tightened the Packs maintenance view:

- Shortened the pack kind explanation, polish-queue summary, boundary reminder, trusted-source helper, and local-install helper.
- Kept the real filters, Studio handoffs, Chat batch handoffs, source diagnostics, and recovery actions unchanged.
- Added focused regressions so the longer category and boundary reminders do not return.

## Seventy-sixth Applied Slice

This iteration refreshed the AgentOps platform-focus card:

- Replaced the completed connector/Pack source aggregation focus with current work: Planner failed-step recovery, Pack maintenance density, and backend-to-AgentOps recovery alignment.
- Converted the hand-numbered focus rows into a semantic ordered list while keeping the card compact.
- Added regressions so the outdated connector/Pack focus and older broad description do not return.

## Seventy-seventh Applied Slice

This iteration added provider recovery deep-link support:

- Let `/settings/providers?tab=...` open the requested provider tab, including model services, add-provider presets, access mode, model routing, and Tori.
- Preserved a safe fallback to the model-services tab for unknown tab values.
- Pointed AgentOps missing-provider recoveries at the add-provider tab and provider-auth/quota recoveries at the model-services tab.
- Added focused tests for direct routing-tab recovery links and invalid-tab fallback.

## Seventy-eighth Applied Slice

This iteration aligned backend provider recovery targets:

- Updated task recovery hints for provider auth, quota, rate-limit, and generic provider failures to open `/settings/providers?tab=providers`.
- Updated Planner failed-step provider recovery targets to use the same focused provider tab.
- Refreshed AgentOps Planner-provider mock coverage so the control tower mirrors the backend recovery href.

## Seventy-ninth Applied Slice

This iteration connected task-detail recovery hints:

- Added a compact task-detail recovery card that displays backend `recovery_hint` summary and detail.
- Renders a real navigation link when `primary_action.href` is present, so task failures can jump directly to provider, connector, browser, approval, skill, or tool recovery surfaces.
- Avoids fake actions when the backend only provides an endpoint action without a safe href.

## Eightieth Applied Slice

This iteration connected task-list recovery hints:

- Added compact Missions task-card recovery links when backend `primary_action.href` is present.
- Kept endpoint-only recovery actions non-clickable so the task list does not show fake recovery buttons.
- Covered provider recovery hrefs in Missions tests.

## Eighty-first Applied Slice

This iteration promoted Planner primary recovery targets:

- Added a compact primary recovery link to Planner checkpoint details when `failure_summary.primary_target.href` is present.
- Kept the link as real navigation, so provider, connector, browser, skill, tool, or dependency targets can open their focused recovery surface.
- Added Planner detail coverage for provider-focused recovery links.

## Eighty-second Applied Slice

This iteration linked chat traces back to Planner recovery:

- Added a real Planner checkpoint detail link to long-horizon checkpoint cards in the execution trace when `plan_id` is present.
- Kept prompt-based recovery actions intact while giving users a direct route to dependency, failed-step, and primary recovery targets.
- Covered the trace-to-detail href in execution trace tests.

## Eighty-third Applied Slice

This iteration connected generic failure recovery traces:

- Added support for `primary_target` and `recovery_target` hrefs on repeated-failure trace cards.
- Kept strategy-switch prompt buttons intact while exposing focused provider, connector, browser, skill, tool, or dependency recovery links when present.
- Covered provider-focused recovery links in execution trace tests.

## Eighty-fourth Applied Slice

This iteration made repeated-failure recovery targets real at the source:

- Added backend `primary_target` support to `PlannerFailureSummary`.
- Mapped repeated model/timeout failures to `/settings/providers?tab=providers`, tool/runtime failures to `/tools`, and trust-gate failures to `/approvals`.
- Covered provider target emission in planner repeated-failure tests.

## Eighty-fifth Applied Slice

This iteration preserved repeated-failure recovery targets through streaming:

- Added a traceview sanitizer branch for `PlannerFailureSummary` so friendly stream events keep `primary_target` while still cleaning raw fallback diagnostics.
- Preserved provider-focused recovery hrefs through `friendlyAgentEventForStream`.
- Covered target preservation and raw-diagnostic cleanup in stream tests.

## Eighty-sixth Applied Slice

This iteration reduced provider demo-check density:

- Shortened the provider settings demo gate from explanatory copy to three quick readiness states: Chat, docs, and image generation.
- Kept the existing refresh and image-model configuration actions intact while removing long status sentences.
- Added regression coverage so the old long demo-check copy does not return.

## Eighty-seventh Applied Slice

This iteration reduced Pack center source-copy density:

- Kept Pack cards focused on the source value, runtime, trust, and delivery state in the default view.
- Moved the extra source explanation line, such as read-only install checks or rollback hints, behind the advanced/maintenance view.
- Updated Pack center tests so source diagnostics stay available for maintainers without crowding the normal recovery surface.

## Eighty-eighth Applied Slice

This iteration refreshed the AgentOps platform-focus card:

- Replaced focus items that pointed at already-delivered Planner recovery and Pack copy work.
- Kept the card short and action-oriented around recovery regression, primary-action defaults, and Workspace/Session policy metadata.
- Added AgentOps regression coverage so the focus card does not drift back to completed roadmap items.

## Eighty-ninth Applied Slice

This iteration split Planner repeated-failure skill recovery from tool recovery:

- Added a `skill_unavailable` bucket for repeated `unknown skill`, missing skill, disabled skill-growth, or missing skill-provider failures.
- Routed that repeated-failure primary target to `/skills` instead of the generic `/tools` surface.
- Kept `unknown tool` and allowed-tool-surface failures on `/tools`, with focused planner tests for both paths.

## Ninetieth Applied Slice

This iteration carried Planner skill recovery through the frontend:

- Added AgentOps coverage for Planner `primary_target.category=skill`, ensuring the recovery queue labels it as a skill incident and links to `/skills`.
- Added chat execution-trace coverage for repeated-failure skill targets so the user can open the skill recovery surface directly from trace details.
- Kept provider and tool recovery tests intact, preserving the distinction between `/settings/providers`, `/tools`, and `/skills`.

## Ninety-first Applied Slice

This iteration aligned task recovery skill/tool splitting:

- Kept normal `unknown skill document_writer` task failures pointed at `/skills`.
- Added a legacy `_tool` guard so `unknown skill missing_tool` style failures remain tool-surface incidents linked to `/tools`.
- Covered both task recovery branches so backend hints match the Planner repeated-failure split.

## Ninety-second Applied Slice

This iteration made `/skills` a clearer recovery landing:

- Added a compact recovery strip for missing-skill incidents with real actions only: scan local skills, open the market tab, or review dynamic skills.
- Kept the page copy short so Planner and task recovery links land on a scan-first surface instead of another dense management page.
- Covered the recovery strip in Skills page tests, including the scan action and tab switches.

## Ninety-third Applied Slice

This iteration made `/tools` a clearer recovery landing:

- Added a compact recovery strip for tool-surface failures with real actions only: start a clean session or focus the command input.
- Kept the terminal as the primary surface while giving Planner and task recovery links a direct reproduction path.
- Covered the strip in Tools page tests, including command focus and real session creation.

## Ninety-fourth Applied Slice

This iteration tightened Planner dependency recovery targets:

- Added a structured dependency primary target for repeated Planner dependency failures without inventing a href when no `plan_id` is available.
- Taught AgentOps to anchor dependency primary targets without href to the current checkpoint dependency view.
- Covered both the backend target shape and the AgentOps dependency-anchor fallback.

## Ninety-fifth Applied Slice

This iteration aligned task dependency recovery with the Planner recovery model:

- Classified task dependency blocks as `dependency` recovery hints instead of generic task failures.
- Pointed the primary action to the task execution chain tab with a real `?tab=execution` route.
- Let task detail initialize from the `tab` query parameter so recovery links land on the relevant execution evidence.

## Ninety-sixth Applied Slice

This iteration carried task dependency recovery into AgentOps:

- Kept backend task `category=dependency` hints grouped under the `依赖` recovery lane.
- Preserved the real task execution-chain link, such as `/task-detail?id=<id>&tab=execution`.
- Added regression coverage so action labels like `查看执行链` do not become their own recovery lane.

## Ninety-seventh Applied Slice

This iteration tightened the AgentOps platform-focus card:

- Replaced broad roadmap copy with three short current control-tower priorities: recovery regression, Workspace/Session policy persistence, and default-state density.
- Kept the card informational only, with no fake controls or placeholder links.
- Added regression coverage so completed or wordier focus items do not drift back into the default view.

## Ninety-eighth Applied Slice

This iteration carried dependency recovery into chat execution traces:

- When a repeated-failure trace includes `primary_target.category=dependency` and a real `plan_id`, the recovery chip now opens `/planner-checkpoint?plan_id=<id>#dependency-view`.
- Trace recovery still hides the link when no plan id exists, avoiding fake dependency actions.
- Added regression coverage so Planner dependency targets behave consistently across AgentOps and trace detail cards.

## Ninety-ninth Applied Slice

This iteration aligned Planner detail recovery targets with the same dependency fallback:

- Planner checkpoint detail now resolves `category=dependency` targets without an href to the current plan's `#dependency-view`.
- The fallback is shared by the primary recovery entry and per-step recovery chips.
- Added coverage to ensure dependency failures show real links instead of raw dependency error text.

## Hundredth Applied Slice

This iteration made task-list recovery more resilient:

- Missions now resolves task `category=dependency` hints without an explicit href to `/task-detail?id=<id>&tab=execution`.
- Explicit backend hrefs still win, and endpoint-only actions remain hidden to avoid fake links.
- Added coverage for provider links, dependency execution-chain links, and non-link endpoint actions.

## Hundred-first Applied Slice

This iteration tightened Pack center default density:

- Kept readiness gap explanations behind the maintenance/details path instead of the default Pack card surface.
- Converted detail-only usage, verification, and entry guidance into semantic lists so the dense maintenance view is easier to scan and read with assistive tech.
- Added a regression check so unclear Packs still show compact status chips by default without rendering the long usability-check sentence.

## Hundred-second Applied Slice

This iteration aligned live Planner recovery events with checkpoint recovery:

- Repeated Planner failures now classify connector and browser recovery targets before they reach chat trace or AgentOps.
- Connector targets preserve real focused links such as `/settings/connectors?focus=github`; browser pairing issues go to `/packs/browser`.
- The classifier avoids treating `auth_type=token` configuration text as an auth failure, keeping recovery incidents tied to actual failure evidence.

## Hundred-third Applied Slice

This iteration started the session-policy metadata lane for Planner execution:

- Added `SessionID` to `planner.PlanRequest` and populated it from chat, agentic SSE, stream chat, and websocket entry points that already carry a real conversation session.
- Planner tool, partial, recovery, retry, handoff, Cogni trace, federation, ReAct, model fallback, and long-horizon events now write `meta.session_id`, so audit replay and AgentOps grouping can link execution failures back to the originating session.
- Long-horizon checkpoints also persist `session_id`, preserving the conversation link across dropped streams and resume flows.
- Added regression coverage for event metadata and checkpoint persistence without inventing agent or workspace policy fields.

## Hundred-fourth Applied Slice

This iteration made Planner session metadata visible through recovery APIs:

- Planner checkpoint summaries now expose `session_id`, so list, recover, resume, and execution-state responses can show the conversation link already persisted by the backend.
- Async resume jobs and their event history now carry `session_id`, including terminal events and JSONL persistence across gateway restarts.
- Execution-state now returns session metadata on the checkpoint, latest resume job, and compact event list, giving AgentOps and trace surfaces a real grouping key without exposing tenant IDs.

## Hundred-fifth Applied Slice

This iteration carried Planner session metadata into the frontend API contract:

- Added `session_id` to Planner checkpoint summaries, resume jobs, and resume job events in the shared runtime TypeScript types.
- Kept the UI unchanged for now, avoiding extra visible metadata until AgentOps and checkpoint detail use the field for real grouping or navigation.
- Verified the web app type contract with `npm run typecheck`.

## Hundred-sixth Applied Slice

This iteration used Planner session metadata in AgentOps without adding a new dense panel:

- Planner recovery rows now show a compact session suffix when a real `session_id` is present on the checkpoint, latest resume job, or resume event.
- Missing session metadata stays invisible, and the full raw session id is not rendered in the recovery lane.
- Added regression coverage so AgentOps keeps the recovery link actionable while showing only the short session attribution.

## Hundred-seventh Applied Slice

This iteration carried the same session attribution into Planner checkpoint detail:

- Unified execution-state now shows a compact session suffix when the checkpoint, latest job, or resume events carry `session_id`.
- The detail view keeps missing session metadata invisible and avoids rendering the full raw session id.
- Added focused coverage so the checkpoint detail page and AgentOps use the same low-density attribution rule.

## Hundred-eighth Applied Slice

This iteration brought compact session attribution into the chat recovery shelf:

- The collapsed Planner recovery summary now includes the same short session suffix when a recoverable checkpoint has `session_id`.
- Expanded recovery rows also show the short session suffix, while the raw session id stays out of the chat surface.
- Resume-job notices can reuse session metadata from the job, job events, or checkpoint, keeping chat, AgentOps, and checkpoint detail aligned.

## Hundred-ninth Applied Slice

This iteration tightened backend task recovery hint rendering:

- Task detail recovery cards now run `summary` and `detail` through the same friendly recovery text pipeline used by execution evidence and task threads.
- Raw runner diagnostics such as timeout, handoff, and EOF details stay out of the overview recovery card.
- Added coverage so structured recovery hints keep their real deep links while masking low-level failure text.

## Hundred-tenth Applied Slice

This iteration carried friendly backend recovery text into AgentOps:

- AgentOps task recovery rows now format backend `recovery_hint` summaries/details and fallback task errors before they reach the recovery queue.
- Raw runner diagnostics such as handoff, timeout, and EOF text stay out of the control tower while concise recovery wording remains visible.
- Structured deep links are preserved, so provider, connector, sandbox, approval, dependency, skill, and tool recovery still route to real control surfaces.

## Hundred-eleventh Applied Slice

This iteration tightened Planner core recovery classification:

- Repeated Planner failures with explicit model-provider evidence now classify as provider recovery instead of generic timeout or repeated-path failures.
- Provider auth, quota, balance, billing, and rate-limit failures point at `/settings/providers?tab=providers` from the Planner self-healing summary.
- Connector rate limits stay on connector recovery, preserving focused links such as `/settings/connectors?focus=github` instead of misrouting to model settings.

## Hundred-twelfth Applied Slice

This iteration carried real recovery links into the task list:

- Mission task cards now derive conservative fallback links for provider, connector, browser, skill, tool, sandbox, approval, and dependency recovery hints when the backend action has no `href`.
- Endpoint-only or generic recovery actions still stay hidden as links, preventing fake task-list buttons.
- Connector and browser failures can now jump from the task list to `/settings/connectors` or `/packs/browser` without waiting for AgentOps.

## Hundred-thirteenth Applied Slice

This iteration refreshed the AgentOps platform-focus panel:

- Removed the now-stale "recovery links regression" focus item after recovery links landed across AgentOps, Planner detail, task detail, task list, and trace surfaces.
- Kept the panel to three short priorities: Workspace/Session policy persistence, grouped recovery incidents, and settings/Pack default-state density.
- Preserved the ordered-list structure and low-density sidebar card without adding new controls or fake navigation.

## Hundred-fourteenth Applied Slice

This iteration grouped repeated AgentOps recovery entries without hiding real incident counts:

- Recovery Queue rows now collapse incidents with the same visible label and recovery `href` into one actionable row, reducing duplicate Pack source and same-entry connector noise.
- Lane chips and the top queue count still use the true raw incident count, so operators can see the blast radius before opening the deep page.
- Overflow text now reports visible grouped rows separately from merged same-entry incidents, keeping links real and the queue easier to scan.

## Hundred-fifteenth Applied Slice

This iteration reduced default Pack-card noise while preserving real source checks:

- Installed Pack cards now hide the redundant source fact in the normal view, leaving the scan row to runtime, trust, and delivery state.
- Maintenance view still shows installed source context and rollback hints, so AgentOps `/packs?maintenance=1` recovery keeps the deeper verification path.
- Installable official/private Pack cards continue to show source origin by default, preserving the real trust check before install.

## Hundred-sixteenth Applied Slice

This iteration made the provider settings recovery landing calmer:

- Healthy provider detail cards now show API key, base URL, model, and breaker state as facts without the extra explanatory hints.
- Provider field hints and the 401/502 reminder appear only when the selected provider is disabled, lacks a key/model, or has an active breaker issue.
- AgentOps and task recovery links to `/settings/providers?tab=providers` still land on a page with concrete repair guidance when the provider actually needs attention.

## Hundred-seventeenth Applied Slice

This iteration connected Chat Planner recovery to the unified execution-state recovery targets:

- The collapsed chat recovery shelf stays quiet, but expanded recovery rows now lazily read `/v1/planner/execution-state` for real primary targets.
- Provider, connector, browser, skill, tool, and dependency recovery links only render when the backend returns a concrete `href`, keeping the shelf free of fake repair buttons.
- Added focused coverage for a connector recovery target so Chat, Planner detail, AgentOps, and task recovery share the same structured recovery path.

## Hundred-eighteenth Applied Slice

This iteration moved Planner resume recovery targets closer to the failed execution itself:

- Resume-plan responses and async resume jobs can now carry `primary_target`, reusing the same provider, connector, browser, skill, tool, and dependency classifier used by execution-state.
- Stored failed resume jobs can derive a target from their retained failed plan result during response sanitization, so older job records still produce a real recovery entry when evidence exists.
- Chat resume-job notices now render that job-provided target directly, while still hiding the link when the backend has no concrete `href`.

## Hundred-nineteenth Applied Slice

This iteration reduced connector settings density in the healthy default state:

- The connector safety overview now hides explanatory detail lines when each lane is healthy, leaving the scan surface to label, value, and status.
- Browser, authorization, attention, and Allowlist details still appear when a lane needs action, preserving the real repair path.
- The healthy footer summary is removed, while blocked states keep a short next-step footer and the existing action button.

## Hundred-twentieth Applied Slice

This iteration reduced Pack card density in the healthy default state:

- Installed and installable Pack cards now hide stable/beta release badges unless the maintenance view is active, while alpha/unknown states remain visible as attention signals.
- Default Pack cards now show two capability labels instead of three, keeping the scan row shorter without changing filters, links, or install actions.
- Maintenance view still exposes release status, source facts, entry paths, and rollback context for recovery and verification.

## Hundred-twenty-first Applied Slice

This iteration refreshed the AgentOps platform-focus card after grouped recovery landed:

- Removed the now-completed grouped-recovery focus item from the sidebar.
- Kept the card to three current platform priorities: Workspace/Session policy persistence, Agent/Workspace capability grouping, and recovery context replay into tasks and Chat.
- Avoided adding fake actions or links; the card remains a compact roadmap signal for the control tower.

## Hundred-twenty-second Applied Slice

This iteration shared task recovery target fallback across task list and task detail:

- Added a common `taskRecoveryTarget` helper for provider, connector, browser, skill, tool, sandbox, approval, and dependency recovery categories.
- Task Detail now renders the same conservative recovery links as Missions when the backend hint has a category but no `href`.
- Endpoint-only or generic recovery actions still render as non-clickable chips, preserving the no-fake-button rule.

## Hundred-twenty-third Applied Slice

This iteration connected AgentOps task recovery to the shared target fallback:

- AgentOps now uses `taskRecoveryTarget` when backend task hints have a known category but omit `primary_action.href`.
- Dependency recovery hints without a backend link now land on the task execution-chain tab instead of the generic task detail route.
- Generic or endpoint-only recovery actions still avoid fake links and continue through the existing conservative inference path.

## Hundred-twenty-fourth Applied Slice

This iteration filled two Planner failed-step recovery gaps:

- Planner execution-state now routes desktop sandbox failures to `/packs/computer-use`.
- Approval or trust-gate failures now route to `/approvals`.
- Existing provider, connector, browser, dependency, skill, and tool recovery targets stay covered by the same failed-step summary test.

## Hundred-twenty-fifth Applied Slice

This iteration made shared task recovery links more specific without adding UI noise:

- `taskRecoveryTarget` now focuses known connector failures such as GitHub, Gmail, Slack, Notion, Linear, Jira, and Google Calendar when backend hints omit `href`.
- Task Detail and Missions now share the focused connector fallback, so a GitHub token failure lands on `/settings/connectors?focus=github`.
- Generic connector hints still land on `/settings/connectors`, and endpoint-only generic actions still avoid fake links.

## Hundred-twenty-sixth Applied Slice

This iteration made AgentOps understand the newer Planner recovery target categories:

- Frontend Planner recovery target types now include `sandbox` and `approval`.
- AgentOps recovery lanes now label Planner sandbox failures as `沙箱` and approval/trust-gate failures as `审批`.
- Planner links still use the backend-provided real targets, such as `/packs/computer-use` and `/approvals`.

## Hundred-twenty-seventh Applied Slice

This iteration aligned Planner recovery fallbacks across Chat, checkpoint detail, and execution trace:

- Chat recovery shelf now anchors sandbox, approval, provider, connector, browser, skill, and tool targets even when the backend omits `href`.
- Planner checkpoint detail and execution trace now use the same conservative target fallback, so those categories stay linked to real surfaces instead of disappearing.
- Dependency still resolves to the dependency view when a plan id is available, preserving the existing deep-link behavior.

## Hundred-twenty-eighth Applied Slice

This iteration made Planner recovery target fallback a shared frontend contract:

- Added `planner-recovery-target` as the single category-to-route helper for Planner recovery links.
- Chat recovery shelf, Planner checkpoint detail, and execution trace now consume the same fallback mapping instead of maintaining local switch blocks.
- Unknown categories without `href` still do not render links, preserving the no-fake-action rule.

## Hundred-twenty-ninth Applied Slice

This iteration made connector recovery focus reusable across task and Planner fallbacks:

- Added `connector-focus` as the shared connector-name-to-settings-focus helper.
- Task recovery and Planner recovery now use the same GitHub, Gmail, Slack, Notion, Linear, Jira, and Google Calendar focus rules when backend hints omit `href`.
- Planner trace details can now focus a connector from retained failure evidence such as failed tools, while generic connector failures still land on `/settings/connectors`.

## Hundred-thirtieth Applied Slice

This iteration reduced repeated Pack-card labels in the default center view:

- Installed and installable Pack cards now hide the header usability chip in the normal view when the same information is already represented by the delivery fact row.
- Experimental packs and maintenance view still show the usability chip, keeping risk and review signals visible when they matter.
- Pack tests now assert the default infrastructure card only shows the `后台支撑` label once, preserving the lower-density scan surface.

## Hundred-thirty-first Applied Slice

This iteration removed another duplicate signal from installable Pack cards:

- Installable Pack cards now rely on `capabilitySurfaceLabels` for frontend/backend surface chips instead of adding separate `独立界面` and `后端能力` chips.
- Official/private source cards still show real source, trust, install, and maintenance details; only the redundant default chip layer was removed.
- Pack tests now assert that an installable backend-capable pack shows `有后端能力` without also rendering the shorter duplicate `后端能力`.

## Hundred-thirty-second Applied Slice

This iteration reduced connector-settings density in the default healthy path:

- The connector detail panel no longer auto-opens the first disconnected app when there is no focused connector or active incident.
- Recovery links and connector errors still open the relevant connector automatically, preserving real recovery paths.
- Capability boundaries, recent events, and usage hints now appear only when selected, focused, connected, or backed by real event/recovery context instead of filling the healthy default view.

## Hundred-thirty-third Applied Slice

This iteration tightened Planner failed-step recovery routing:

- Planner approval recovery now matches real approval, trust-gate, or policy-gate failures instead of broad auth words such as `forbidden`.
- Connector 403 failures such as GitHub scope errors now land on `/settings/connectors?focus=github` instead of `/approvals`.
- Provider 403 failures still land on `/settings/providers?tab=providers`, while true approval-required failures continue to route to `/approvals`.

## Hundred-thirty-fourth Applied Slice

This iteration made provider recovery deep links more precise:

- `/settings/providers?focus=<provider_id>` and related provider query aliases now open the model-services tab and select the matching provider.
- Existing `/settings/providers?tab=...` links still work for routing, presets, mode, and Tori surfaces.
- Provider settings tests now prove a non-default provider can be focused from a recovery URL without showing the wrong tab.

## Hundred-thirty-fifth Applied Slice

This iteration connected provider focus links to frontend recovery fallbacks:

- Added a shared provider focus helper that extracts explicit provider ids such as `provider_id=qwen-backup` without guessing from generic `OpenAI provider` wording.
- Task recovery and Planner trace/detail fallbacks now use `/settings/providers?focus=<provider_id>` when retained failure evidence includes a concrete provider id.
- Generic provider failures still land on `/settings/providers?tab=providers`, preserving conservative routing when the exact provider is unknown.

## Hundred-thirty-sixth Applied Slice

This iteration pushed provider focus recovery back into backend hints:

- Backend task recovery now turns explicit provider evidence such as `provider_id=qwen-backup` into `/settings/providers?focus=qwen-backup`.
- Planner failed-step recovery targets now use the same focused provider route when the failed step carries an explicit provider id.
- Provider id detection avoids generic provider names and plain vendor domains, so unknown or generic model failures still use the conservative provider settings tab.

## Hundred-thirty-seventh Applied Slice

This iteration aligned AgentOps local provider fallback recovery with the shared focus helper:

- AgentOps task recovery now reuses provider focus extraction when older backends omit structured task recovery hints.
- Explicit provider evidence such as `provider_id=qwen-backup` lands on `/settings/providers?focus=qwen-backup` directly from the Recovery Queue.
- Generic provider authentication, quota, and rate-limit incidents still land on `/settings/providers?tab=providers`, preserving conservative routing when no exact provider is known.

## Hundred-thirty-eighth Applied Slice

This iteration aligned AgentOps Planner recovery with the shared Planner target resolver:

- AgentOps now resolves provider, connector, browser, skill, tool, sandbox, approval, and dependency targets even when the backend returns only a recovery category.
- Planner provider failures with retained evidence such as `provider_id=qwen-backup` now deep-link from AgentOps to `/settings/providers?focus=qwen-backup`.
- Unknown Planner target categories still fall back to the checkpoint page, avoiding fake recovery buttons while preserving the execution scene.

## Hundred-thirty-ninth Applied Slice

This iteration refreshed the AgentOps platform-focus card after task, Chat, Planner, and AgentOps recovery links converged:

- Removed the stale recovery-context replay item from the visible roadmap card.
- Kept the focus card to three scan-first items: Workspace/Session policy persistence, Agent/Workspace capability grouping, and incident history with grouping ids.
- Avoided adding new buttons or explanatory copy; the card remains a compact roadmap signal rather than another settings surface.

## Hundred-fortieth Applied Slice

This iteration started the backend contract needed for grouped incident history:

- Task `recovery_hint` now includes a stable `group_key` derived from the recovery category and primary recovery target.
- Repeated failures for the same provider or connector repair path now share the same group key, while task-specific recovery links can remain distinct.
- The work-pack task list exposes the group key alongside existing recovery hints, giving AgentOps and future incident history a durable grouping primitive without adding fake UI.

## Hundred-forty-first Applied Slice

This iteration carried task recovery group keys into AgentOps grouping:

- Recovery Queue items can now carry an optional `groupKey`, with task recovery rows sourced from backend `recovery_hint.group_key`.
- AgentOps groups rows by `groupKey` before falling back to visible label and href, so differently labeled task failures with the same repair target collapse into one actionable row.
- Lane chips still count the true incident count while the queue keeps the scan-first grouped row, preserving blast-radius visibility without adding more text.

## Hundred-forty-second Applied Slice

This iteration let Planner recovery participate in the same incident grouping as task recovery:

- Planner recovery rows now derive a normalized group key from their resolved recovery category and href.
- Browser Planner targets normalize to the connector lane key, while provider, connector, sandbox, approval, skill, tool, and dependency targets keep stable category-based keys.
- AgentOps can now collapse a failed task and a Planner failed execution into one row when they share the same concrete repair target, while lane chips still show the real incident count.

## Hundred-forty-third Applied Slice

This iteration made frontend recovery grouping reusable:

- Extracted recovery target grouping into a shared `recovery-group-key` helper instead of leaving the normalization rules inside AgentOps.
- Locked provider, browser-to-connector, model-to-provider, dependency, approval, connector, and tool grouping behavior with focused helper tests.
- AgentOps now consumes the shared helper, giving future Chat, Task Detail, and incident-history surfaces the same grouping contract without copying UI logic.

## Hundred-forty-fourth Applied Slice

This iteration pushed recovery grouping into Planner execution-state targets:

- Planner recovery targets now include `group_key` alongside category, label, href, and action.
- Backend target generation normalizes browser recovery into the connector group and emits stable keys for provider, connector, dependency, skill, tool, sandbox, and approval targets.
- Frontend API types now expose the field, so AgentOps, Chat, trace, and future incident-history views can consume backend grouping facts instead of recomputing them.

## Hundred-forty-fifth Applied Slice

This iteration made frontend Planner target resolution consume backend group keys:

- `planner-recovery-target` now preserves backend `group_key` when present and only derives a key as a fallback for older backend responses.
- AgentOps now prefers the resolver-provided key for Planner recovery rows, keeping backend grouping facts authoritative while retaining backward compatibility.
- Focused resolver tests cover direct targets, trace-detail targets, browser-to-connector grouping, and legacy fallback generation.

## Hundred-forty-sixth Applied Slice

This iteration continued reducing provider-settings density without changing the recovery surface:

- Shortened the model operations overview, provider list header, Tori state detail, and mode helper copy so the page scans through status and actions first.
- Kept recovery actions real and unchanged: provider deep links, Tori binding, routing, connection testing, and provider deletion still map to existing surfaces.
- Tightened provider-detail failure hints to short action phrases and locked the old long copy out with focused settings-page tests.

## Next Slices

1. Add persisted per-agent workspace policy metadata once richer multi-agent routing is ready.
2. Let AgentOps group capabilities by agent/workspace once multi-agent routing carries policy metadata.
3. Promote repeated connector and Pack source failures into incident history once the backend exposes source health beyond latest snapshots.
4. Turn policy boundary rows into editable guardrails once the backend exposes policy write APIs.
5. Continue removing repeated fields from settings and Pack default states while keeping recovery details conditional on real problems.
6. Carry grouped recovery context into task detail and chat traces when a backend grouping id is available.
