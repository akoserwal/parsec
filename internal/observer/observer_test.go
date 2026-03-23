package observer

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/project-kessel/parsec/internal/datasource"
	"github.com/project-kessel/parsec/internal/keys"
	"github.com/project-kessel/parsec/internal/server"
	"github.com/project-kessel/parsec/internal/service"
	"github.com/project-kessel/parsec/internal/trust"
)

func TestNoOp_AllProbeMethodsCallable(t *testing.T) {
	obs := NoOp()
	ctx := context.Background()

	ctx2, tip := obs.TokenIssuanceStarted(ctx, nil, nil, "", nil)
	if ctx2 != ctx {
		t.Error("expected same context back from noop TokenIssuanceStarted")
	}
	tip.TokenTypeIssuanceStarted("t")
	tip.TokenTypeIssuanceSucceeded("t", nil)
	tip.TokenTypeIssuanceFailed("t", errors.New("x"))
	tip.IssuerNotFound("t", errors.New("x"))
	tip.End()

	_, tep := obs.TokenExchangeStarted(ctx, "", "", "", "")
	tep.ActorValidationSucceeded(nil)
	tep.ActorValidationFailed(errors.New("x"))
	tep.RequestContextParsed(nil)
	tep.RequestContextParseFailed(errors.New("x"))
	tep.SubjectTokenValidationSucceeded(nil)
	tep.SubjectTokenValidationFailed(errors.New("x"))
	tep.End()

	_, acp := obs.AuthzCheckStarted(ctx)
	acp.RequestAttributesParsed(nil)
	acp.ActorValidationSucceeded(nil)
	acp.ActorValidationFailed(errors.New("x"))
	acp.SubjectCredentialExtracted(nil, nil)
	acp.SubjectCredentialExtractionFailed(errors.New("x"))
	acp.SubjectValidationSucceeded(nil)
	acp.SubjectValidationFailed(errors.New("x"))
	acp.End()

	_, dp := obs.CacheFetchStarted(ctx, "ds")
	dp.CacheHit()
	dp.CacheMiss()
	dp.CacheExpired()
	dp.FetchFailed(errors.New("x"))

	_, lp := obs.LuaFetchStarted(ctx, "lua")
	lp.ScriptLoadFailed(errors.New("x"))
	lp.ScriptExecutionFailed(errors.New("x"))
	lp.InvalidReturnType("number")
	lp.FetchCompleted()

	_, kp := obs.RotationCheckStarted(ctx)
	kp.RotationCheckFailed(errors.New("x"))
	kp.ActiveKeyCacheUpdateFailed(errors.New("x"))
	kp.RotationCompleted("slot")
	kp.RotationSkippedVersionRace("slot")
	kp.KeyProviderNotFound("p", "s")
	kp.KeyHandleFailed("s", errors.New("x"))
	kp.PublicKeyFailed("s", errors.New("x"))
	kp.ThumbprintFailed("s", errors.New("x"))
	kp.MetadataFailed("s", errors.New("x"))

	_, pp := obs.KeyProvisionStarted(ctx)
	pp.OldKeyDeletionFailed("kid", errors.New("x"))

	_, tp := obs.ValidationStarted(ctx)
	tp.ValidatorFailed("v", trust.CredentialTypeJWT, errors.New("x"))
	tp.AllValidatorsFailed(trust.CredentialTypeBearer, 2, errors.New("x"))
	tp.ValidatorFiltered("v", "actor")
	tp.FilterEvaluationFailed("v", errors.New("x"))

	_, jp := obs.CacheRefreshStarted(ctx)
	jp.InitialCachePopulationFailed(errors.New("x"))
	jp.CacheRefreshFailed(errors.New("x"))
	jp.KeyConversionFailed("kid", errors.New("x"))

	_, sp := obs.ServeStarted(ctx)
	sp.GRPCServeFailed(errors.New("x"))
	sp.HTTPServeFailed(errors.New("x"))
}

func TestCompose_DelegatesToCorrectSubObserver(t *testing.T) {
	var (
		cacheCalled   atomic.Int32
		luaCalled     atomic.Int32
		keyRotCalled  atomic.Int32
		keyProvCalled atomic.Int32
		trustCalled   atomic.Int32
		jwksCalled    atomic.Int32
		srvCalled     atomic.Int32
	)

	obs := Compose(
		service.NoOpObserver(),
		struct {
			datasource.CacheObserver
			datasource.LuaObserver
		}{&spyDSCacheObserver{called: &cacheCalled}, &spyLuaDSObserver{called: &luaCalled}},
		struct {
			keys.RotationObserver
			keys.ProviderObserver
		}{&spyKeyRotObserver{called: &keyRotCalled}, &spyKeyProvObserver{called: &keyProvCalled}},
		&spyTrustObserver{called: &trustCalled},
		struct {
			server.JWKSObserver
			server.LifecycleObserver
		}{&spyJWKSObserver{called: &jwksCalled}, &spySrvLifeObserver{called: &srvCalled}},
	)

	ctx := context.Background()

	obs.CacheFetchStarted(ctx, "ds")
	if cacheCalled.Load() != 1 {
		t.Errorf("CacheFetchStarted: expected cache observer called once, got %d", cacheCalled.Load())
	}

	obs.LuaFetchStarted(ctx, "lua")
	if luaCalled.Load() != 1 {
		t.Errorf("LuaFetchStarted: expected lua observer called once, got %d", luaCalled.Load())
	}

	obs.RotationCheckStarted(ctx)
	if keyRotCalled.Load() != 1 {
		t.Errorf("RotationCheckStarted: expected key rotation observer called once, got %d", keyRotCalled.Load())
	}

	obs.KeyProvisionStarted(ctx)
	if keyProvCalled.Load() != 1 {
		t.Errorf("KeyProvisionStarted: expected key provider observer called once, got %d", keyProvCalled.Load())
	}

	obs.ValidationStarted(ctx)
	if trustCalled.Load() != 1 {
		t.Errorf("ValidationStarted: expected trust observer called once, got %d", trustCalled.Load())
	}

	obs.CacheRefreshStarted(ctx)
	if jwksCalled.Load() != 1 {
		t.Errorf("CacheRefreshStarted: expected JWKS observer called once, got %d", jwksCalled.Load())
	}

	obs.ServeStarted(ctx)
	if srvCalled.Load() != 1 {
		t.Errorf("ServeStarted: expected server lifecycle observer called once, got %d", srvCalled.Load())
	}
}

func TestCompositeAll_SingleChild_ReturnsSame(t *testing.T) {
	child := NoOp()
	result := CompositeAll([]Observer{child})
	if result != child {
		t.Error("CompositeAll with 1 child should return that child directly")
	}
}

func TestCompositeAll_FansOutAllInfraTypes(t *testing.T) {
	var (
		cache1, cache2     atomic.Int32
		lua1, lua2         atomic.Int32
		keyRot1, keyRot2   atomic.Int32
		keyProv1, keyProv2 atomic.Int32
		trust1, trust2     atomic.Int32
		jwks1, jwks2       atomic.Int32
		srv1, srv2         atomic.Int32
	)

	child1 := Compose(
		service.NoOpObserver(),
		struct {
			datasource.CacheObserver
			datasource.LuaObserver
		}{&spyDSCacheObserver{called: &cache1}, &spyLuaDSObserver{called: &lua1}},
		struct {
			keys.RotationObserver
			keys.ProviderObserver
		}{&spyKeyRotObserver{called: &keyRot1}, &spyKeyProvObserver{called: &keyProv1}},
		&spyTrustObserver{called: &trust1},
		struct {
			server.JWKSObserver
			server.LifecycleObserver
		}{&spyJWKSObserver{called: &jwks1}, &spySrvLifeObserver{called: &srv1}},
	)
	child2 := Compose(
		service.NoOpObserver(),
		struct {
			datasource.CacheObserver
			datasource.LuaObserver
		}{&spyDSCacheObserver{called: &cache2}, &spyLuaDSObserver{called: &lua2}},
		struct {
			keys.RotationObserver
			keys.ProviderObserver
		}{&spyKeyRotObserver{called: &keyRot2}, &spyKeyProvObserver{called: &keyProv2}},
		&spyTrustObserver{called: &trust2},
		struct {
			server.JWKSObserver
			server.LifecycleObserver
		}{&spyJWKSObserver{called: &jwks2}, &spySrvLifeObserver{called: &srv2}},
	)

	composite := CompositeAll([]Observer{child1, child2})
	ctx := context.Background()

	composite.CacheFetchStarted(ctx, "ds")
	composite.LuaFetchStarted(ctx, "lua")
	composite.RotationCheckStarted(ctx)
	composite.KeyProvisionStarted(ctx)
	composite.ValidationStarted(ctx)
	composite.CacheRefreshStarted(ctx)
	composite.ServeStarted(ctx)

	for _, tc := range []struct {
		name   string
		c1, c2 *atomic.Int32
	}{
		{"DataSourceCache", &cache1, &cache2},
		{"LuaDataSource", &lua1, &lua2},
		{"KeyRotation", &keyRot1, &keyRot2},
		{"KeyProvider", &keyProv1, &keyProv2},
		{"Trust", &trust1, &trust2},
		{"JWKS", &jwks1, &jwks2},
		{"ServerLifecycle", &srv1, &srv2},
	} {
		if tc.c1.Load() != 1 {
			t.Errorf("%s: child1 expected 1 call, got %d", tc.name, tc.c1.Load())
		}
		if tc.c2.Load() != 1 {
			t.Errorf("%s: child2 expected 1 call, got %d", tc.name, tc.c2.Load())
		}
	}
}

func TestCompositeAll_MultiProbe_FansOutEvents(t *testing.T) {
	var hits1, hits2 atomic.Int32

	child1 := Compose(
		service.NoOpObserver(),
		struct {
			datasource.CacheObserver
			datasource.LuaObserver
		}{&spyDSCacheObserver{called: new(atomic.Int32), hitCalled: &hits1}, datasource.NoOpObserver{}},
		keys.NoOpObserver{},
		trust.NoOpObserver{},
		server.NoOpObserver{},
	)
	child2 := Compose(
		service.NoOpObserver(),
		struct {
			datasource.CacheObserver
			datasource.LuaObserver
		}{&spyDSCacheObserver{called: new(atomic.Int32), hitCalled: &hits2}, datasource.NoOpObserver{}},
		keys.NoOpObserver{},
		trust.NoOpObserver{},
		server.NoOpObserver{},
	)

	composite := CompositeAll([]Observer{child1, child2})
	_, probe := composite.CacheFetchStarted(context.Background(), "ds")
	probe.CacheHit()

	if hits1.Load() != 1 {
		t.Errorf("child1 CacheHit: expected 1, got %d", hits1.Load())
	}
	if hits2.Load() != 1 {
		t.Errorf("child2 CacheHit: expected 1, got %d", hits2.Load())
	}
}

// --- spy implementations for testing delegation ---

type spyDSCacheObserver struct {
	called    *atomic.Int32
	hitCalled *atomic.Int32
}

func (s *spyDSCacheObserver) CacheFetchStarted(ctx context.Context, _ string) (context.Context, datasource.CacheFetchProbe) {
	s.called.Add(1)
	return ctx, &spyDSCacheProbe{hitCalled: s.hitCalled}
}

type spyDSCacheProbe struct{ hitCalled *atomic.Int32 }

func (s *spyDSCacheProbe) CacheHit() {
	if s.hitCalled != nil {
		s.hitCalled.Add(1)
	}
}
func (s *spyDSCacheProbe) CacheMiss()        {}
func (s *spyDSCacheProbe) CacheExpired()     {}
func (s *spyDSCacheProbe) FetchFailed(error) {}
func (s *spyDSCacheProbe) End()              {}

type spyLuaDSObserver struct{ called *atomic.Int32 }

func (s *spyLuaDSObserver) LuaFetchStarted(_ context.Context, _ string) (context.Context, datasource.LuaFetchProbe) {
	s.called.Add(1)
	return datasource.NoOpObserver{}.LuaFetchStarted(context.Background(), "")
}

type spyKeyRotObserver struct{ called *atomic.Int32 }

func (s *spyKeyRotObserver) RotationCheckStarted(_ context.Context) (context.Context, keys.RotationCheckProbe) {
	s.called.Add(1)
	return keys.NoOpObserver{}.RotationCheckStarted(context.Background())
}

type spyKeyProvObserver struct{ called *atomic.Int32 }

func (s *spyKeyProvObserver) KeyProvisionStarted(_ context.Context) (context.Context, keys.KeyProvisionProbe) {
	s.called.Add(1)
	return keys.NoOpObserver{}.KeyProvisionStarted(context.Background())
}

type spyTrustObserver struct{ called *atomic.Int32 }

func (s *spyTrustObserver) ValidationStarted(_ context.Context) (context.Context, trust.ValidationProbe) {
	s.called.Add(1)
	return trust.NoOpObserver{}.ValidationStarted(context.Background())
}

type spyJWKSObserver struct{ called *atomic.Int32 }

func (s *spyJWKSObserver) CacheRefreshStarted(_ context.Context) (context.Context, server.CacheRefreshProbe) {
	s.called.Add(1)
	return server.NoOpObserver{}.CacheRefreshStarted(context.Background())
}

type spySrvLifeObserver struct{ called *atomic.Int32 }

func (s *spySrvLifeObserver) ServeStarted(_ context.Context) (context.Context, server.ServeProbe) {
	s.called.Add(1)
	return server.NoOpObserver{}.ServeStarted(context.Background())
}
