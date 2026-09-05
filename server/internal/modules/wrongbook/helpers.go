package wrongbook

import "time"

func defaultPaperTitle(now time.Time) string {
	return now.Format("2006-01-02") + " 错题复习卷"
}

func defaultPaperSource(value string) string {
	switch value {
	case PaperSourceParent, PaperSourceSystem, PaperSourceTeacher:
		return value
	default:
		return PaperSourceTeacher
	}
}

func defaultGeneratedBy(value string) string {
	switch value {
	case GeneratedByParent, GeneratedByStaff, GeneratedBySystem:
		return value
	default:
		return GeneratedByStaff
	}
}
