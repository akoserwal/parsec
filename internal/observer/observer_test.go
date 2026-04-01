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

	{
		ctx2, p := obs.TokenIssuanceStarted(ctx, nil, nil, "", nil)
		if ctx2 != ctx {
			t.Error("expected same context back from noop TokenIssuanceStarted")
		}
		p.TokenTypeIssuanceStarted("t")
		p.TokenTypeIssuanceSucceeded("t", nil)
		p.TokenTypeIssuanceFailed("t", errors.New("x"))
		p.IssuerNotFound("t", errors.New("x"))
		p.End()
	}
	{
		_, p := obs.TokenExchangeStarted(ctx, "", "", "", "")
		p.ActorValidationSucceeded(nil)
		p.ActorValidationFailed(errors.New("x"))
		p.RequestContextParsed(nil)
		p.RequestContextParseFailed(errors.New("x"))
		p.SubjectTokenValidationSucceeded(nil)
		p.SubjectTokenValidationFailed(errors.New("x"))
		p.End()
	}
	{
		_, p := obs.AuthzCheckStarted(ctx)
		p.RequestAttributesParsed(nil)
		p.ActorValidationSucceeded(nil)
		p.ActorValidationFailed(errors.New("x"))
		p.SubjectCredentialExtracted(nil, nil)
		p.SubjectCredentialExtractionFailed(errors.New("x"))
		p.SubjectValidationSucceeded(nil)
		p.SubjectValidationFailed(errors.New("x"))
		p.End()
	}
	{
		_, p := obs.CacheFetchStarted(ctx, "ds")
		p.CacheHit()
		p.CacheMiss()
		p.CacheExpired()
		p.FetchFailed(errors.New("x"))
	}
	{
		_, p := obs.LuaFetchStarted(ctx, "lua")
		p.ScriptLoadFailed(errors.New("x"))
		p.ScriptExecutionFailed(errors.New("x"))
		p.InvalidReturnType("number")
		p.FetchCompleted()
		p.FetchCompletedNil()
		p.ResultConversionFailed(errors.New("x"))
	}
	{
		_, p := obs.RotationCheckStarted(ctx)
		p.RotationCheckFailed(errors.New("x"))
		p.RotationCompleted("slot")
		p.RotationSkippedVersionRace("slot")
		p.End()
	}
	{
		_, p := obs.KeyCacheUpdateStarted(ctx)
		p.KeyCacheUpdateFailed(errors.New("x"))
		p.KeyProviderNotFound("p", "s")
		p.KeyHandleFailed("s", errors.New("x"))
		p.PublicKeyFailed("s", errors.New("x"))
		p.ThumbprintFailed("s", errors.New("x"))
		p.MetadataFailed("s", errors.New("x"))
	}
	{
		_, p := obs.KeyProvisionStarted(ctx)
		p.OldKeyDeletionFailed("kid", errors.New("x"))
	}
	{
		_, p := obs.ValidationStarted(ctx)
		p.ValidatorFailed("v", trust.CredentialTypeJWT, errors.New("x"))
		p.AllValidatorsFailed(trust.CredentialTypeBearer, 2, errors.New("x"))
		p.ValidatorFiltered("v", "actor")
		p.FilterEvaluationFailed("v", errors.New("x"))
	}
	{
		_, p := obs.InitPopulationStarted(ctx)
		p.InitialCachePopulationFailed(errors.New("x"))
		p.End()
	}
	{
		_, p := obs.CacheRefreshStarted(ctx)
		p.CacheRefreshFailed(errors.New("x"))
		p.KeyConversionFailed("kid", errors.New("x"))
	}
	obs.GRPCServeFailed(errors.New("x"))
	obs.HTTPServeFailed(errors.New("x"))
	{
		_, p := obs.StopStarted(ctx)
		p.End()
	}
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

	obs.StopStarted(ctx)
	if srvCalled.Load() != 1 {
		t.Errorf("StopStarted: expected server lifecycle observer called once, got %d", srvCalled.Load())
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
	composite.StopStarted(ctx)

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

func TestCompositeAll_FansOutServiceTypes(t *testing.T) {
	var (
		tokenIss1, tokenIss2   atomic.Int32
		tokenExch1, tokenExch2 atomic.Int32
		authz1, authz2         atomic.Int32
	)

	child1 := Compose(
		&spyServiceObserver{
			tokenIssuanceCalled: &tokenIss1,
			tokenExchangeCalled: &tokenExch1,
			authzCheckCalled:    &authz1,
		},
		datasource.NoOpObserver{},
		keys.NoOpObserver{},
		trust.NoOpObserver{},
		server.NoOpObserver{},
	)
	child2 := Compose(
		&spyServiceObserver{
			tokenIssuanceCalled: &tokenIss2,
			tokenExchangeCalled: &tokenExch2,
			authzCheckCalled:    &authz2,
		},
		datasource.NoOpObserver{},
		keys.NoOpObserver{},
		trust.NoOpObserver{},
		server.NoOpObserver{},
	)

	composite := CompositeAll([]Observer{child1, child2})
	ctx := context.Background()

	composite.TokenIssuanceStarted(ctx, nil, nil, "", nil)
	composite.TokenExchangeStarted(ctx, "", "", "", "")
	composite.AuthzCheckStarted(ctx)

	for _, tc := range []struct {
		name   string
		c1, c2 *atomic.Int32
	}{
		{"TokenIssuance", &tokenIss1, &tokenIss2},
		{"TokenExchange", &tokenExch1, &tokenExch2},
		{"AuthzCheck", &authz1, &authz2},
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
	_, p := composite.CacheFetchStarted(context.Background(), "ds")
	p.CacheHit()

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

type spyKeyRotObserver struct {
	keys.NoOpRotationObserver
	called *atomic.Int32
}

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

type spyJWKSObserver struct {
	server.NoOpJWKSObserver
	called *atomic.Int32
}

func (s *spyJWKSObserver) CacheRefreshStarted(_ context.Context) (context.Context, server.CacheRefreshProbe) {
	s.called.Add(1)
	return server.NoOpObserver{}.CacheRefreshStarted(context.Background())
}

type spySrvLifeObserver struct {
	server.NoOpLifecycleObserver
	called *atomic.Int32
}

func (s *spySrvLifeObserver) StopStarted(_ context.Context) (context.Context, server.StopProbe) {
	s.called.Add(1)
	return server.NoOpObserver{}.StopStarted(context.Background())
}

type spyServiceObserver struct {
	service.NoOpServiceObserver
	tokenIssuanceCalled *atomic.Int32
	tokenExchangeCalled *atomic.Int32
	authzCheckCalled    *atomic.Int32
}

func (s *spyServiceObserver) TokenIssuanceStarted(ctx context.Context, _ *trust.Result, _ *trust.Result, _ string, _ []service.TokenType) (context.Context, service.TokenIssuanceProbe) {
	s.tokenIssuanceCalled.Add(1)
	return ctx, &service.NoOpTokenIssuanceProbe{}
}

func (s *spyServiceObserver) TokenExchangeStarted(ctx context.Context, _ string, _ string, _ string, _ string) (context.Context, service.TokenExchangeProbe) {
	s.tokenExchangeCalled.Add(1)
	return ctx, &service.NoOpTokenExchangeProbe{}
}

func (s *spyServiceObserver) AuthzCheckStarted(ctx context.Context) (context.Context, service.AuthzCheckProbe) {
	s.authzCheckCalled.Add(1)
	return ctx, &service.NoOpAuthzCheckProbe{}
}
