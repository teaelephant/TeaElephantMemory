// Package model contains GraphQL models and helpers for API v2.
package model

import "github.com/teaelephant/TeaElephantMemory/common"

// FromCommon converts a common.NotificationType to the GraphQL NotificationType.
func (t *NotificationType) FromCommon(data common.NotificationType) {
	switch data {
	case common.NotificationTypeTeaExpiration:
		*t = NotificationTypeTeaExpiration
	case common.NotificationTypeTeaRecommendation:
		*t = NotificationTypeTeaRecommendation
	default:
		*t = NotificationTypeUnknown
	}
}
