package s3gen

// BuildPhase represents a stage in the build pipeline.
// Phases execute in order: Discover → Transform → Generate → Finalize
type BuildPhase int

const (
	// PhaseDiscover finds all resources, identifies types, loads metadata
	PhaseDiscover BuildPhase = iota

	// PhaseTransform handles asset transformations (SCSS→CSS, image optimization, etc.)
	PhaseTransform

	// PhaseGenerate produces final outputs (MD→HTML, HTML→HTML, parametric expansion)
	PhaseGenerate

	// PhaseFinalize runs after all content is generated (sitemap, RSS, search index)
	PhaseFinalize
)

func (p BuildPhase) String() string {
	switch p {
	case PhaseDiscover:
		return "Discover"
	case PhaseTransform:
		return "Transform"
	case PhaseGenerate:
		return "Generate"
	case PhaseFinalize:
		return "Finalize"
	default:
		return "Unknown"
	}
}

// BuildContext holds state that persists across phases during a single build.
type BuildContext struct {
	Site         *Site
	CurrentPhase BuildPhase

	// All resources discovered
	Resources []*Resource

	// Resources created in each phase
	CreatedInPhase map[BuildPhase][]*Resource

	// All targets generated (for Finalize phase access)
	GeneratedTargets []*Resource

	// Errors accumulated during build (allows continuing on non-fatal errors)
	Errors []error

	// Hooks for observation
	hooks *HookRegistry
}

// AddError adds an error to the build context.
func (ctx *BuildContext) AddError(err error) {
	if err != nil {
		ctx.Errors = append(ctx.Errors, err)
	}
}

// AddTarget adds a generated target to the context.
func (ctx *BuildContext) AddTarget(target *Resource) {
	ctx.GeneratedTargets = append(ctx.GeneratedTargets, target)
}

// HookRegistry manages lightweight hooks for build observation.
// This is simpler than a full EventBus - just callbacks organized by phase.
type HookRegistry struct {
	onPhaseStart      map[BuildPhase][]func(*BuildContext)
	onPhaseEnd        map[BuildPhase][]func(*BuildContext)
	onResourceProcess []func(*BuildContext, *Resource, []*Resource)
}

// NewHookRegistry creates a new hook registry.
func NewHookRegistry() *HookRegistry {
	return &HookRegistry{
		onPhaseStart: make(map[BuildPhase][]func(*BuildContext)),
		onPhaseEnd:   make(map[BuildPhase][]func(*BuildContext)),
	}
}

// OnPhaseStart registers a callback to run when a phase starts.
func (h *HookRegistry) OnPhaseStart(phase BuildPhase, fn func(*BuildContext)) {
	h.onPhaseStart[phase] = append(h.onPhaseStart[phase], fn)
}

// OnPhaseEnd registers a callback to run when a phase ends.
func (h *HookRegistry) OnPhaseEnd(phase BuildPhase, fn func(*BuildContext)) {
	h.onPhaseEnd[phase] = append(h.onPhaseEnd[phase], fn)
}

// OnResourceProcessed registers a callback to run after each resource is processed.
// The callback receives the source resource and all targets it generated.
func (h *HookRegistry) OnResourceProcessed(fn func(*BuildContext, *Resource, []*Resource)) {
	h.onResourceProcess = append(h.onResourceProcess, fn)
}

// emitPhaseStart calls all registered phase start hooks.
func (h *HookRegistry) emitPhaseStart(ctx *BuildContext) {
	if h == nil {
		return
	}
	for _, fn := range h.onPhaseStart[ctx.CurrentPhase] {
		fn(ctx)
	}
}

// emitPhaseEnd calls all registered phase end hooks.
func (h *HookRegistry) emitPhaseEnd(ctx *BuildContext) {
	if h == nil {
		return
	}
	for _, fn := range h.onPhaseEnd[ctx.CurrentPhase] {
		fn(ctx)
	}
}

// emitResourceProcessed calls all registered resource processed hooks.
func (h *HookRegistry) emitResourceProcessed(ctx *BuildContext, res *Resource, targets []*Resource) {
	if h == nil {
		return
	}
	for _, fn := range h.onResourceProcess {
		fn(ctx, res, targets)
	}
}

// PhaseRule is an optional interface that rules can implement to participate
// in the phase-based build pipeline. Rules that don't implement this interface
// are wrapped in LegacyRuleAdapter and run in PhaseGenerate.
type PhaseRule interface {
	Rule

	// Phase returns which build phase this rule runs in.
	Phase() BuildPhase

	// DependsOn returns glob patterns of files this rule needs as input.
	// Used to order rules within a phase.
	DependsOn() []string

	// Produces returns glob patterns of files this rule creates.
	// Used to order rules within a phase.
	Produces() []string
}

// AssetAction specifies how to handle a co-located asset.
type AssetAction int

const (
	// AssetCopy copies the asset to the destination as-is.
	AssetCopy AssetAction = iota

	// AssetProcess runs the asset through transform rules.
	AssetProcess

	// AssetSkip ignores the asset.
	AssetSkip
)

// AssetMapping describes how to handle a single co-located asset.
type AssetMapping struct {
	Source *Resource
	Dest   string      // Path relative to output directory
	Action AssetAction
}

// AssetAwareRule is an optional interface for rules that handle co-located assets.
// When a content file has assets (images, data files in the same directory),
// rules implementing this interface can specify how those assets should be handled.
type AssetAwareRule interface {
	Rule

	// HandleAssets is called with co-located assets for a resource.
	// Returns mappings describing how each asset should be processed.
	HandleAssets(site *Site, res *Resource, assets []*Resource) ([]AssetMapping, error)
}

// LegacyRuleAdapter wraps a Rule that doesn't implement PhaseRule,
// allowing it to work in the phase-based pipeline.
type LegacyRuleAdapter struct {
	Wrapped Rule
}

// Phase returns PhaseGenerate - legacy rules run in the generate phase.
func (l *LegacyRuleAdapter) Phase() BuildPhase {
	return PhaseGenerate
}

// DependsOn returns nil - legacy rules don't declare dependencies.
func (l *LegacyRuleAdapter) DependsOn() []string {
	return nil
}

// Produces returns nil - legacy rules don't declare what they produce.
func (l *LegacyRuleAdapter) Produces() []string {
	return nil
}

// TargetsFor delegates to the wrapped rule.
func (l *LegacyRuleAdapter) TargetsFor(site *Site, res *Resource) ([]*Resource, []*Resource) {
	return l.Wrapped.TargetsFor(site, res)
}

// Run delegates to the wrapped rule.
func (l *LegacyRuleAdapter) Run(site *Site, inputs []*Resource, targets []*Resource, funcs map[string]any) error {
	return l.Wrapped.Run(site, inputs, targets, funcs)
}
