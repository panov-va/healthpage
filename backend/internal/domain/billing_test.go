package domain

import (
	"testing"
	"time"
)

func TestSubscriptionGrantsPremium(t *testing.T) {
	cases := []struct {
		name string
		plan BillingPlan
		st   SubscriptionStatus
		want bool
	}{
		{"premium active", PlanPremium, SubStatusActive, true},
		{"premium past_due grace", PlanPremium, SubStatusPastDue, true},
		{"premium pending", PlanPremium, SubStatusPending, false},
		{"premium canceled", PlanPremium, SubStatusCanceled, false},
		{"free active", PlanFree, SubStatusActive, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := Subscription{Plan: c.plan, Status: c.st}
			if got := s.GrantsPremium(); got != c.want {
				t.Fatalf("GrantsPremium=%v want %v", got, c.want)
			}
		})
	}
}

func TestSubscriptionInTrial(t *testing.T) {
	now := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	future := now.Add(24 * time.Hour)
	past := now.Add(-24 * time.Hour)
	if (Subscription{}).InTrial(now) {
		t.Fatal("no trial_ends_at → not in trial")
	}
	if !(Subscription{TrialEndsAt: &future}).InTrial(now) {
		t.Fatal("future trial end → in trial")
	}
	if (Subscription{TrialEndsAt: &past}).InTrial(now) {
		t.Fatal("past trial end → not in trial")
	}
}

func TestBillingPeriodAdvance(t *testing.T) {
	from := time.Date(2026, 1, 31, 12, 0, 0, 0, time.UTC)
	if got := PeriodMonthly.Advance(from); !got.Equal(from.AddDate(0, 1, 0)) {
		t.Fatalf("monthly advance=%v", got)
	}
	if got := PeriodYearly.Advance(from); !got.Equal(from.AddDate(1, 0, 0)) {
		t.Fatalf("yearly advance=%v", got)
	}
}

func TestBillingEnumsValidity(t *testing.T) {
	if !SubStatusActive.IsValid() || SubscriptionStatus("nope").IsValid() {
		t.Fatal("SubscriptionStatus.IsValid")
	}
	if !PaymentSucceeded.IsValid() || PaymentStatus("nope").IsValid() {
		t.Fatal("PaymentStatus.IsValid")
	}
	if !ProviderYooKassa.IsValid() || PaymentProvider("nope").IsValid() {
		t.Fatal("PaymentProvider.IsValid")
	}
	if !PeriodMonthly.IsValid() || BillingPeriod("nope").IsValid() {
		t.Fatal("BillingPeriod.IsValid")
	}
}

func TestPlanAllows(t *testing.T) {
	premiumOnly := []Feature{
		FeatureCustomDomain, FeaturePrivatePages, FeatureCustomSMTP, FeatureWhiteLabel, FeaturePrioritySupport,
	}
	for _, f := range premiumOnly {
		if PlanAllows(PlanFree, f) {
			t.Fatalf("free must NOT allow %s", f)
		}
		if !PlanAllows(PlanPremium, f) {
			t.Fatalf("premium must allow %s", f)
		}
	}
	// Незарегистрированная (не-premium) возможность доступна на любом тарифе.
	if !PlanAllows(PlanFree, Feature("components")) {
		t.Fatal("non-premium feature must be allowed on free")
	}
}
