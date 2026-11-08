package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	// CompositeRouteReasonAmbiguous marks a model whose explicit account
	// mappings span several provider platforms.
	CompositeRouteReasonAmbiguous = "model is exposed by multiple provider platforms"
	// CompositeRouteReasonOAuthAmbiguous is the ambiguous case where a claimant
	// holds an OAuth / setup-token credential that only the OpenAI gateway can
	// carry, and the model-name fallback would also pick an OpenAI-gateway
	// platform. The unresolved request would land in the Anthropic gateway
	// handler and select those credentials, so this group misconfiguration must
	// be rejected instead of guessing a platform from the model name.
	CompositeRouteReasonOAuthAmbiguous = "model is exposed by multiple provider platforms including OAuth accounts"
)

// OAuthAmbiguous reports whether the request must be rejected because the
// cross-platform ownership involves OpenAI-gateway OAuth credentials.
func (d CompositeRouteDecision) OAuthAmbiguous() bool {
	return !d.Matched && d.Reason == CompositeRouteReasonOAuthAmbiguous
}

// AmbiguousMessage is the client-facing rejection for an OAuthAmbiguous decision.
func (d CompositeRouteDecision) AmbiguousMessage() string {
	return fmt.Sprintf("Model %q is mapped on multiple provider platforms in this group; configure an explicit composite route", d.PublicModel)
}

const (
	compositeRoundRobinCacheSize = 1024
	compositeRouteCursorTTL      = 24 * time.Hour
)

type CompositeRouteCursorCache interface {
	NextCompositeRouteCursor(ctx context.Context, key string, ttl time.Duration) (uint64, error)
	PeekCompositeRouteCursor(ctx context.Context, key string) (uint64, error)
}

type compositeRoundRobinEntry struct {
	cursor   atomic.Uint64
	lastUsed time.Time
}

type CompositeRouteResolver struct {
	repo                   CompositeModelRouteRepository
	modelOwnershipResolver CompositeModelOwnershipResolver
	cursorCache            CompositeRouteCursorCache
	roundRobinMu           sync.Mutex
	roundRobinCounters     map[string]*compositeRoundRobinEntry
}

func NewCompositeRouteResolver(repo CompositeModelRouteRepository) *CompositeRouteResolver {
	return &CompositeRouteResolver{repo: repo}
}

func (r *CompositeRouteResolver) SetModelOwnershipResolver(resolver CompositeModelOwnershipResolver) {
	if r != nil {
		r.modelOwnershipResolver = resolver
	}
}

func ProvideCompositeRouteResolver(repo CompositeModelRouteRepository, cursorCache CompositeRouteCursorCache) *CompositeRouteResolver {
	return &CompositeRouteResolver{repo: repo, cursorCache: cursorCache}
}

// ListExactPublicModels returns enabled, concrete route IDs suitable for a model catalog.
func (r *CompositeRouteResolver) ListExactPublicModels(ctx context.Context, groupID int64, endpoint string, includeSystemOne bool) ([]string, error) {
	if r == nil || r.repo == nil || groupID <= 0 {
		return nil, nil
	}
	routes, err := r.repo.ListByGroup(ctx, groupID, false)
	if err != nil {
		return nil, err
	}
	models := make([]string, 0, len(routes))
	seen := make(map[string]struct{}, len(routes))
	for _, route := range routes {
		if route.Enabled && normalizeCompositeRouteMatchType(route.MatchType) == CompositeRouteMatchExact &&
			(endpoint == "" || normalizeCompositeRouteEndpoint(route.Endpoint) == CompositeRouteEndpointAny || normalizeCompositeRouteEndpoint(route.Endpoint) == endpoint) {
			if model := strings.TrimSpace(route.PublicModel); model != "" {
				// A plain model ID represents an automatically routed model. Keep
				// it visible once even when several providers expose it. Explicit
				// provider-prefixed IDs (for example openai/gpt-5 and kimi/gpt-5)
				// remain separate because their IDs are different.
				if _, ok := seen[model]; ok {
					continue
				}
				seen[model] = struct{}{}
				models = append(models, model)
			}
		}
	}
	if !includeSystemOne {
		models = filterSystemOneRouteModels(routes, models, endpoint)
	}
	return models, nil
}

func (r *CompositeRouteResolver) FilterCodexModels(ctx context.Context, groupID int64, models []string) ([]string, error) {
	if r == nil || r.repo == nil || groupID <= 0 {
		return models, nil
	}
	routes, err := r.repo.ListByGroup(ctx, groupID, false)
	if err != nil {
		return nil, err
	}
	return filterSystemOneRouteModels(routes, models, CompositeRouteEndpointResponses), nil
}

func filterSystemOneRouteModels(routes []CompositeModelRoute, models []string, endpoint string) []string {
	filtered := make([]string, 0, len(models))
	for _, model := range models {
		candidates := bestCompositeRouteCandidates(compositeRouteCandidates(
			routes, model, normalizeCompositeRouteEndpoint(endpoint),
		))
		if hasMixedSystemOneCandidates(candidates) {
			continue
		}
		systemOneOnly := len(candidates) > 0
		for _, candidate := range candidates {
			if candidate.route.TargetPlatform != PlatformTypeSafe {
				systemOneOnly = false
				break
			}
		}
		if !systemOneOnly {
			filtered = append(filtered, model)
		}
	}
	return filtered
}

func (r *CompositeRouteResolver) Resolve(ctx context.Context, groupID int64, model, endpoint string) (CompositeRouteDecision, error) {
	return r.resolve(ctx, groupID, model, endpoint, true)
}

// ResolvePreview resolves a route without advancing production round-robin cursors.
func (r *CompositeRouteResolver) ResolvePreview(ctx context.Context, groupID int64, model, endpoint string) (CompositeRouteDecision, error) {
	return r.resolve(ctx, groupID, model, endpoint, false)
}

func (r *CompositeRouteResolver) resolve(ctx context.Context, groupID int64, model, endpoint string, advanceRoundRobin bool) (CompositeRouteDecision, error) {
	model = strings.TrimSpace(model)
	endpoint = normalizeCompositeRouteEndpoint(endpoint)
	decision := CompositeRouteDecision{
		GroupID:     groupID,
		PublicModel: model,
		Endpoint:    endpoint,
	}
	if model == "" {
		decision.Reason = "model is required"
		return decision, nil
	}

	if r != nil && r.repo != nil && groupID > 0 {
		routes, err := r.repo.ListByGroup(ctx, groupID, false)
		if err != nil {
			return decision, fmt.Errorf("list composite routes: %w", err)
		}
		eligible := bestCompositeRouteCandidates(compositeRouteCandidates(routes, model, endpoint))
		if hasMixedSystemOneCandidates(eligible) {
			return decision, infraerrors.BadRequest(
				"COMPOSITE_ROUTE_CONFIG_INVALID",
				fmt.Sprintf("Composite TypeSafe routes cannot share public model %q with other platforms", model),
			)
		}
		if route, ok := r.selectCompositeRoute(ctx, eligible, compositeRoundRobinKey(groupID, model, endpoint, eligible), advanceRoundRobin); ok {
			upstreamModel := strings.TrimSpace(route.UpstreamModel)
			if upstreamModel == "" {
				upstreamModel = model
			}
			return CompositeRouteDecision{
				Matched:        true,
				Source:         CompositeRouteSourceExplicit,
				GroupID:        groupID,
				PublicModel:    model,
				TargetPlatform: route.TargetPlatform,
				UpstreamModel:  upstreamModel,
				Endpoint:       endpoint,
				Route:          &route,
			}, nil
		}
	}

	if r != nil && r.modelOwnershipResolver != nil && groupID > 0 {
		ownership, err := r.modelOwnershipResolver(ctx, groupID, model)
		if err != nil {
			// A recognizable model can still use the existing detector when the
			// account catalog is temporarily unavailable. Unknown aliases cannot.
			if _, detectable := DetectModelPlatform(model); !detectable {
				return decision, fmt.Errorf("resolve account model ownership: %w", err)
			}
		} else if ownership.Ambiguous {
			decision.Reason = CompositeRouteReasonAmbiguous
			// 未解析的请求由下游 handler 按模型名前缀兜底选平台；只有兜底落在
			// OpenAI 网关族时，OpenAI 网关 OAuth 声明方才会被误送进 Anthropic 管线。
			if guessed, ok := DetectModelPlatform(model); ok && ownership.OpenAIGatewayOAuthClaimed && domain.UsesOpenAIGateway(guessed) {
				decision.Reason = CompositeRouteReasonOAuthAmbiguous
			}
			return decision, nil
		} else if ownership.Matched {
			platform := strings.TrimSpace(ownership.TargetPlatform)
			if !isConcreteRequestPlatform(platform) {
				decision.Reason = "account model ownership has no concrete target platform"
				return decision, nil
			}
			return CompositeRouteDecision{
				Matched:        true,
				Source:         CompositeRouteSourceAccount,
				GroupID:        groupID,
				PublicModel:    model,
				TargetPlatform: platform,
				UpstreamModel:  model,
				Endpoint:       endpoint,
			}, nil
		}
	}

	if platform, ok := DetectModelPlatform(model); ok {
		return CompositeRouteDecision{
			Matched:        true,
			Source:         CompositeRouteSourceDetector,
			GroupID:        groupID,
			PublicModel:    model,
			TargetPlatform: platform,
			UpstreamModel:  model,
			Endpoint:       endpoint,
		}, nil
	}
	decision.Reason = "no explicit route or built-in detector match"
	return decision, nil
}

// selectCompositeRoute round-robins the best equally specific, equal-priority routes.
func (r *CompositeRouteResolver) selectCompositeRoute(ctx context.Context, eligible []compositeRouteCandidate, key string, advanceRoundRobin bool) (CompositeModelRoute, bool) {
	if len(eligible) == 0 {
		return CompositeModelRoute{}, false
	}
	if len(eligible) == 1 || r == nil {
		return eligible[0].route, true
	}

	index := r.roundRobinIndex(ctx, key, advanceRoundRobin)
	return eligible[index%uint64(len(eligible))].route, true
}

func (r *CompositeRouteResolver) roundRobinIndex(ctx context.Context, key string, advance bool) uint64 {
	if r.cursorCache != nil {
		var index uint64
		var err error
		if advance {
			index, err = r.cursorCache.NextCompositeRouteCursor(ctx, key, compositeRouteCursorTTL)
		} else {
			index, err = r.cursorCache.PeekCompositeRouteCursor(ctx, key)
		}
		if err == nil {
			return index
		}
	}
	r.roundRobinMu.Lock()
	entry := r.roundRobinCounters[key]
	if entry == nil {
		if !advance {
			r.roundRobinMu.Unlock()
			return 0
		}
		if r.roundRobinCounters == nil {
			r.roundRobinCounters = make(map[string]*compositeRoundRobinEntry)
		}
		if len(r.roundRobinCounters) >= compositeRoundRobinCacheSize {
			oldestKey := ""
			var oldest time.Time
			for candidateKey, candidate := range r.roundRobinCounters {
				if oldestKey == "" || candidate.lastUsed.Before(oldest) {
					oldestKey = candidateKey
					oldest = candidate.lastUsed
				}
			}
			delete(r.roundRobinCounters, oldestKey)
		}
		entry = &compositeRoundRobinEntry{}
		r.roundRobinCounters[key] = entry
	}
	if !advance {
		index := entry.cursor.Load()
		r.roundRobinMu.Unlock()
		return index
	}
	entry.lastUsed = time.Now()
	r.roundRobinMu.Unlock()
	return entry.cursor.Add(1) - 1
}

func compositeRoundRobinKey(groupID int64, model, endpoint string, eligible []compositeRouteCandidate) string {
	var key strings.Builder
	fmt.Fprintf(&key, "%d|%d|%s|%s", groupID, len(model), model, endpoint)
	for _, candidate := range eligible {
		fmt.Fprintf(&key, "|%d", candidate.route.ID)
	}
	return key.String()
}

func hasMixedSystemOneCandidates(candidates []compositeRouteCandidate) bool {
	hasSystemOne := false
	hasOtherPlatform := false
	for _, candidate := range candidates {
		if candidate.route.TargetPlatform == PlatformTypeSafe {
			hasSystemOne = true
		} else {
			hasOtherPlatform = true
		}
	}
	return hasSystemOne && hasOtherPlatform
}

type compositeRouteCandidate struct {
	route          CompositeModelRoute
	matchStrength  int
	endpointWeight int
	prefixLen      int
}

func bestCompositeRouteCandidates(candidates []compositeRouteCandidate) []compositeRouteCandidate {
	if len(candidates) == 0 {
		return nil
	}
	best := candidates[0]
	eligible := candidates[:1]
	for _, candidate := range candidates[1:] {
		if candidate.matchStrength != best.matchStrength ||
			candidate.endpointWeight != best.endpointWeight ||
			candidate.prefixLen != best.prefixLen ||
			candidate.route.Priority != best.route.Priority {
			break
		}
		eligible = append(eligible, candidate)
	}
	return eligible
}

func compositeRouteCandidates(routes []CompositeModelRoute, model, endpoint string) []compositeRouteCandidate {
	if len(routes) == 0 {
		return nil
	}

	candidates := make([]compositeRouteCandidate, 0, len(routes))
	for _, route := range routes {
		route.Endpoint = normalizeCompositeRouteEndpoint(route.Endpoint)
		if route.Endpoint != endpoint && route.Endpoint != CompositeRouteEndpointAny {
			continue
		}
		route.MatchType = normalizeCompositeRouteMatchType(route.MatchType)
		publicModel := strings.TrimSpace(route.PublicModel)
		if publicModel == "" {
			continue
		}

		matchStrength := 0
		prefixLen := len(publicModel)
		switch route.MatchType {
		case CompositeRouteMatchExact:
			if publicModel != model {
				continue
			}
			matchStrength = 2
		case CompositeRouteMatchPrefix:
			if !strings.HasPrefix(model, publicModel) {
				continue
			}
			matchStrength = 1
		default:
			continue
		}
		endpointWeight := 0
		if route.Endpoint == endpoint {
			endpointWeight = 1
		}
		candidates = append(candidates, compositeRouteCandidate{
			route:          route,
			matchStrength:  matchStrength,
			endpointWeight: endpointWeight,
			prefixLen:      prefixLen,
		})
	}
	if len(candidates) == 0 {
		return nil
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.matchStrength != b.matchStrength {
			return a.matchStrength > b.matchStrength
		}
		if a.endpointWeight != b.endpointWeight {
			return a.endpointWeight > b.endpointWeight
		}
		if a.prefixLen != b.prefixLen {
			return a.prefixLen > b.prefixLen
		}
		if a.route.Priority != b.route.Priority {
			return a.route.Priority < b.route.Priority
		}
		return a.route.ID < b.route.ID
	})
	return candidates
}
