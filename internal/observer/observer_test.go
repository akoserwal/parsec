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

func TestNoop_AllProbeMethodsCallable(t *testing.T) {
	obs := Noop()
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

	dp := obs.DataSourceCacheProbe(ctx, "ds")
	dp.CacheHit()
	dp.CacheMiss()
	dp.CacheExpired()
	dp.FetchFailed(errors.New("x"))

	lp := obs.LuaDataSourceProbe(ctx, "lua")
	lp.ScriptLoadFailed(errors.New("x"))
	lp.ScriptExecutionFailed(errors.New("x"))
	lp.InvalidReturnType("number")
	lp.FetchCompleted()

	kp := obs.KeyRotationProbe(ctx)
	kp.RotationCheckFailed(errors.New("x"))
	kp.ActiveKeyCacheUpdateFailed(errors.New("x"))
	kp.RotationCompleted("slot")
	kp.RotationSkippedVersionRace("slot")
	kp.KeyProviderNotFound("p", "s")
	kp.KeyHandleFailed("s", errors.New("x"))
	kp.PublicKeyFailed("s", errors.New("x"))
	kp.ThumbprintFailed("s", errors.New("x"))
	kp.MetadataFailed("s", errors.New("x"))

	pp := obs.KeyProviderProbe(ctx)
	pp.OldKeyDeletionFailed("kid", errors.New("x"))

	tp := obs.TrustValidationProbe(ctx)
	tp.ValidatorFailed("v", trust.CredentialTypeJWT, errors.New("x"))
	tp.AllValidatorsFailed(trust.CredentialTypeBearer, 2, errors.New("x"))
	tp.ValidatorFiltered("v", "actor")
	tp.FilterEvaluationFailed("v", errors.New("x"))

	jp := obs.JWKSCacheProbe(ctx)
	jp.InitialCachePopulationFailed(errors.New("x"))
	jp.CacheRefreshFailed(errors.New("x"))
	jp.KeyConversionFailed("kid", errors.New("x"))

	sp := obs.ServerLifecycleProbe(ctx)
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
		&spyDSCacheObserver{called: &cacheCalled},
		&spyLuaDSObserver{called: &luaCalled},
		&spyKeyRotObserver{called: &keyRotCalled},
		&spyKeyProvObserver{called: &keyProvCalled},
		&spyTrustObserver{called: &trustCalled},
		&spyJWKSObserver{called: &jwksCalled},
		&spySrvLifeObserver{called: &srvCalled},
	)

	ctx := context.Background()

	obs.DataSourceCacheProbe(ctx, "ds")
	if cacheCalled.Load() != 1 {
		t.Errorf("DataSourceCacheProbe: expected cache observer called once, got %d", cacheCalled.Load())
	}

	obs.LuaDataSourceProbe(ctx, "lua")
	if luaCalled.Load() != 1 {
		t.Errorf("LuaDataSourceProbe: expected lua observer called once, got %d", luaCalled.Load())
	}

	obs.KeyRotationProbe(ctx)
	if keyRotCalled.Load() != 1 {
		t.Errorf("KeyRotationProbe: expected key rotation observer called once, got %d", keyRotCalled.Load())
	}

	obs.KeyProviderProbe(ctx)
	if keyProvCalled.Load() != 1 {
		t.Errorf("KeyProviderProbe: expected key provider observer called once, got %d", keyProvCalled.Load())
	}

	obs.TrustValidationProbe(ctx)
	if trustCalled.Load() != 1 {
		t.Errorf("TrustValidationProbe: expected trust observer called once, got %d", trustCalled.Load())
	}

	obs.JWKSCacheProbe(ctx)
	if jwksCalled.Load() != 1 {
		t.Errorf("JWKSCacheProbe: expected JWKS observer called once, got %d", jwksCalled.Load())
	}

	obs.ServerLifecycleProbe(ctx)
	if srvCalled.Load() != 1 {
		t.Errorf("ServerLifecycleProbe: expected server lifecycle observer called once, got %d", srvCalled.Load())
	}
}

func TestCompositeAll_SingleChild_ReturnsSame(t *testing.T) {
	child := Noop()
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
		&spyDSCacheObserver{called: &cache1},
		&spyLuaDSObserver{called: &lua1},
		&spyKeyRotObserver{called: &keyRot1},
		&spyKeyProvObserver{called: &keyProv1},
		&spyTrustObserver{called: &trust1},
		&spyJWKSObserver{called: &jwks1},
		&spySrvLifeObserver{called: &srv1},
	)
	child2 := Compose(
		service.NoOpObserver(),
		&spyDSCacheObserver{called: &cache2},
		&spyLuaDSObserver{called: &lua2},
		&spyKeyRotObserver{called: &keyRot2},
		&spyKeyProvObserver{called: &keyProv2},
		&spyTrustObserver{called: &trust2},
		&spyJWKSObserver{called: &jwks2},
		&spySrvLifeObserver{called: &srv2},
	)

	composite := CompositeAll([]Observer{child1, child2})
	ctx := context.Background()

	composite.DataSourceCacheProbe(ctx, "ds")
	composite.LuaDataSourceProbe(ctx, "lua")
	composite.KeyRotationProbe(ctx)
	composite.KeyProviderProbe(ctx)
	composite.TrustValidationProbe(ctx)
	composite.JWKSCacheProbe(ctx)
	composite.ServerLifecycleProbe(ctx)

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
		&spyDSCacheObserver{called: new(atomic.Int32), hitCalled: &hits1},
		datasource.NoopObserver{},
		keys.NoopObserver{},
		keys.NoopObserver{},
		trust.NoopObserver{},
		server.NoopObserver{},
		server.NoopObserver{},
	)
	child2 := Compose(
		service.NoOpObserver(),
		&spyDSCacheObserver{called: new(atomic.Int32), hitCalled: &hits2},
		datasource.NoopObserver{},
		keys.NoopObserver{},
		keys.NoopObserver{},
		trust.NoopObserver{},
		server.NoopObserver{},
		server.NoopObserver{},
	)

	composite := CompositeAll([]Observer{child1, child2})
	probe := composite.DataSourceCacheProbe(context.Background(), "ds")
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

func (s *spyDSCacheObserver) DataSourceCacheProbe(_ context.Context, _ string) datasource.DataSourceCacheProbe {
	s.called.Add(1)
	return &spyDSCacheProbe{hitCalled: s.hitCalled}
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

type spyLuaDSObserver struct{ called *atomic.Int32 }

func (s *spyLuaDSObserver) LuaDataSourceProbe(_ context.Context, _ string) datasource.LuaDataSourceProbe {
	s.called.Add(1)
	return datasource.NoopObserver{}.LuaDataSourceProbe(context.Background(), "")
}

type spyKeyRotObserver struct{ called *atomic.Int32 }

func (s *spyKeyRotObserver) KeyRotationProbe(_ context.Context) keys.KeyRotationProbe {
	s.called.Add(1)
	return keys.NoopObserver{}.KeyRotationProbe(context.Background())
}

type spyKeyProvObserver struct{ called *atomic.Int32 }

func (s *spyKeyProvObserver) KeyProviderProbe(_ context.Context) keys.KeyProviderProbe {
	s.called.Add(1)
	return keys.NoopObserver{}.KeyProviderProbe(context.Background())
}

type spyTrustObserver struct{ called *atomic.Int32 }

func (s *spyTrustObserver) TrustValidationProbe(_ context.Context) trust.TrustValidationProbe {
	s.called.Add(1)
	return trust.NoopObserver{}.TrustValidationProbe(context.Background())
}

type spyJWKSObserver struct{ called *atomic.Int32 }

func (s *spyJWKSObserver) JWKSCacheProbe(_ context.Context) server.JWKSCacheProbe {
	s.called.Add(1)
	return server.NoopObserver{}.JWKSCacheProbe(context.Background())
}

type spySrvLifeObserver struct{ called *atomic.Int32 }

func (s *spySrvLifeObserver) ServerLifecycleProbe(_ context.Context) server.ServerLifecycleProbe {
	s.called.Add(1)
	return server.NoopObserver{}.ServerLifecycleProbe(context.Background())
}
