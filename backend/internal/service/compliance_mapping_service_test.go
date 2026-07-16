package service

import (
	"context"
	"testing"
)

func TestMapJurisdiction_EUHighRisk(t *testing.T) {
	svc := NewComplianceMappingService()
	result, err := svc.MapJurisdiction(context.Background(), JurisdictionMappingRequest{
		CompanyRegion: "eu",
		Industry:      "healthcare",
		ServiceType:   "ai_recommendation",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.CompanyRegion != JurisdictionEU {
		t.Fatalf("expected region %s, got %s", JurisdictionEU, result.CompanyRegion)
	}
	if result.RiskLevel != JurisdictionRiskHigh {
		t.Fatalf("expected high risk for healthcare, got %s", result.RiskLevel)
	}
	if !containsString(result.ApplicableRegulations, "GDPR") || !containsString(result.ApplicableRegulations, "EU AI Act") {
		t.Fatalf("expected EU regulations, got %v", result.ApplicableRegulations)
	}
}

func TestMapJurisdiction_ChinaDefaultRisk(t *testing.T) {
	svc := NewComplianceMappingService()
	result, err := svc.MapJurisdiction(context.Background(), JurisdictionMappingRequest{
		CompanyRegion: "China",
		Industry:      "ecommerce",
		ServiceType:   "ai_chatbot",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RiskLevel != JurisdictionRiskMedium {
		t.Fatalf("expected medium risk, got %s", result.RiskLevel)
	}
	if !containsString(result.ApplicableRegulations, "个人信息保护法") {
		t.Fatalf("expected China regulations, got %v", result.ApplicableRegulations)
	}
}

func TestMapJurisdiction_Unsupported(t *testing.T) {
	svc := NewComplianceMappingService()
	if _, err := svc.MapJurisdiction(context.Background(), JurisdictionMappingRequest{CompanyRegion: "Mars"}); err == nil {
		t.Fatal("expected error for unsupported region")
	}
}

func containsString(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}
