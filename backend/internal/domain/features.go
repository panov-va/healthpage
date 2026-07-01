package domain

// ── Feature-flags тарифов (DESIGN §10; этап 6.7) ──
//
// Все ограничения тарифа выражены через единый набор флагов, а не разбросаны по коду
// (DESIGN §10 [ТРЕБОВАНИЕ]). Гейтинг premium-фич делается через PlanAllows: при попытке
// включить premium-возможность на тарифе Free API возвращает 403.

// Feature — премиальная возможность, ограничиваемая тарифом.
type Feature string

const (
	FeatureCustomDomain    Feature = "custom_domain"    // собственный домен (CNAME + TLS)
	FeaturePrivatePages    Feature = "private_pages"    // приватные страницы (пароль / список email)
	FeatureCustomSMTP      Feature = "custom_smtp"      // свой SMTP / собственный From
	FeatureWhiteLabel      Feature = "white_label"      // скрытие «Работает на …»
	FeaturePrioritySupport Feature = "priority_support" // приоритетная поддержка (организационно)
)

// premiumFeatures — возможности, доступные только на Premium (DESIGN §10).
// Всё, что не в наборе, доступно на любом тарифе (компоненты/инциденты/подписчики/брендинг/команда).
var premiumFeatures = map[Feature]bool{
	FeatureCustomDomain:    true,
	FeaturePrivatePages:    true,
	FeatureCustomSMTP:      true,
	FeatureWhiteLabel:      true,
	FeaturePrioritySupport: true,
}

// PlanAllows сообщает, доступна ли возможность на данном тарифе.
// Premium-возможности требуют PlanPremium; остальные доступны всегда.
func PlanAllows(plan BillingPlan, f Feature) bool {
	if premiumFeatures[f] {
		return plan == PlanPremium
	}
	return true
}
