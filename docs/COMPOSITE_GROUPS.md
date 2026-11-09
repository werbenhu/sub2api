# Multi-Platform Groups

Multi-platform groups let one API key serve models from several account
platforms. The internal platform value remains `composite` for compatibility;
there is no additional group hierarchy and no need to create separate concrete
provider groups first. The platform field is a multi-select: choosing one
concrete platform creates a regular group, while choosing several creates a
multi-platform group. The selected platforms are stored in `platforms` and
restrict account bindings and route targets. Legacy multi-platform groups with
an empty `platforms` list continue to allow every concrete platform.
Editing other settings preserves that open list, including support for future
platforms. Changing the platform selection explicitly restricts the group to
the selected platforms. Simple mode also supports platform selection, account
bindings, and composite model routes. It keeps its existing platform pool
scheduling and skips billing checks.

All groups use subscription quota. Administrators assign or renew subscriptions
directly. Wallet deduction, online payment, redemption codes, and referral
billing are not part of this workflow.

An API key can store an ordered `group_ids` list in addition to the legacy
`group_id`. New default keys collect every active subscription group. Each
request resolves its model to one actual group, then uses that group's routes,
pricing, limits and subscription. The legacy field remains as the compatibility
default; old keys require no migration of their stored data.

## Supported Providers

A multi-platform group can contain accounts from:

- Anthropic
- Gemini
- OpenAI
- Antigravity
- Grok
- Kimi
- Zhipu GLM
- DeepSeek
- MiniMax

The selected account's concrete platform is recorded in usage logs and used for
account selection, platform attribution, and statistics. This keeps same-named
models distinguishable in operational data even when clients use one public
model name.

## Model Routes

Administrators can manage routes from the group list or through:

- `GET /api/v1/admin/groups/:id/composite-routes`
- `POST /api/v1/admin/groups/:id/composite-routes`
- `PUT /api/v1/admin/groups/:id/composite-routes/:route_id`
- `DELETE /api/v1/admin/groups/:id/composite-routes/:route_id`
- `POST /api/v1/admin/groups/:id/composite-routes/preview`

Each route contains:

- `public_model`: model identifier sent by the client.
- `match_type`: `exact` or `prefix`; legacy empty values mean `exact`.
- `target_platform`: concrete account platform.
- `upstream_model`: model sent upstream; empty reuses the public model.
- `endpoint`: `any`, `messages`, `count_tokens`, `responses`,
  `chat_completions`, `embeddings`, `images`, or `gemini`.
- `priority`: lower values win after match specificity.
- `enabled`: disabled routes remain visible to administrators but are not used.

Saving validates endpoint compatibility: Gemini native routes target Gemini or
Antigravity, embeddings target OpenAI, and images target OpenAI or Grok.

Matching order is:

1. Exact match before prefix match.
2. Endpoint-specific route before `any`.
3. Longer prefix before shorter prefix.
4. Lower `priority` value.
5. Round-robin among routes tied on all preceding fields.

Round-robin cursors are independent per group, public model, endpoint, and
candidate route set. Redis provides an atomic cursor shared by all instances,
with a 24-hour idle expiry. If Redis is unavailable, each process falls back to
its bounded 1024-entry LRU. Preview reads the current shared candidate without
advancing the cursor; concurrent traffic can still change the next real selection.

## Same-Named Models

Routes may share a public model:

| Public model | Target | Behavior |
| --- | --- | --- |
| `gpt-5` | OpenAI and Kimi | One catalog entry; tied routes share traffic |
| `openai/gpt-5` | OpenAI | Separate catalog entry |
| `kimi/gpt-5` | Kimi | Separate catalog entry |

Use provider-prefixed public names when users must select a platform explicitly.
Use the same unprefixed name with tied routes when the gateway should merge the
catalog entry and distribute requests.

TypeSafe cannot be tied with other platforms for the same public model because
it only accepts `/v1/systemone`. Such a conflict returns HTTP 400 and is hidden
from Codex catalogs. Codex capability metadata also fails closed when tied
routes have different concrete targets or upstream models.

## Request Processing

1. Authenticate the API key and validate its subscription.
2. Extract the model from the JSON body, URL, WebSocket `response.create`, or
   Live `session.model`.
3. Resolve an explicit route, then legacy account model mapping, then built-in
   model detection.
4. Write the concrete target platform and upstream model into the request
   context and rewrite the outgoing model.
5. Select and retry accounts within the selected platform.
6. Record usage and increment subscription quota using the group's pricing.

Route selection currently happens once per request. Lower-priority routes are
not automatic cross-platform failovers.

For JSON endpoints, the gateway rewrites the request `model`. Native Gemini paths
resolve the model in `/v1beta/models/{model}:...`. Live and Responses WebSocket
requests use the `responses` route domain; changing a WebSocket turn's public
model requires reconnecting.

## Built-In Detection

Without an explicit route, common names map as follows:

- `claude-*` and `anthropic/claude-*` → Anthropic
- `gemini-*` and `google/gemini-*` → Gemini
- `gpt-*`, `o*`, `codex-*`, `text-embedding-*`, `dall-e-*`, and `openai/*` → OpenAI
- `grok-*` and `xai/grok-*` → Grok
- `kimi-*`, `moonshot-*`, and `k3` → Kimi
- `glm-*` → Zhipu GLM
- `deepseek-*` → DeepSeek
- `minimax-*`, `abab*`, and `minimax/*` → MiniMax

Unknown or ambiguous names fail closed.

## Pricing and Subscriptions

Configure model prices on the group. In a multi-platform group, each pricing
entry selects a concrete `platform`; `composite` is not a valid pricing target.
Existing channel prices are copied into groups by migration 250 when a group has
no custom prices. Group prices take precedence over legacy channel and built-in
prices.

Administrators can bind any active group when assigning a subscription. Daily,
weekly, and monthly subscription limits are evaluated against the group, while
usage statistics continue to report the actual account platform.

## Current Limits

- No automatic retry on a different platform after route selection.
- No configurable weighted or failover-only policy beyond priority and tied
  round-robin.
- Redis failures degrade round-robin coordination to per-process cursors.
