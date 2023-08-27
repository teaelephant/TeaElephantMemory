package model

import "github.com/teaelephant/TeaElephantMemory/common"

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
