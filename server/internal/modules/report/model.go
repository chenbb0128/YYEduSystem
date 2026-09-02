package report

type DailyOverview struct {
	Date                 string           `json:"date"`
	Pickup               PickupOverview   `json:"pickup"`
	Homework             HomeworkOverview `json:"homework"`
	MealPlans            int              `json:"meal_plans"`
	MealRecorded         bool             `json:"meal_recorded"`
	PendingApplications  int              `json:"pending_applications"`
	PendingLeaveRequests int              `json:"pending_leave_requests"`
	SummaryStatus        string           `json:"summary_status,omitempty"`
	Anomalies            []Anomaly        `json:"anomalies"`
	Classes              []ClassOverview  `json:"classes"`
}

type PickupOverview struct {
	Operations   int            `json:"operations"`
	Students     int            `json:"students"`
	Resolved     int            `json:"resolved"`
	PhotoMissing int            `json:"photo_missing"`
	Statuses     map[string]int `json:"statuses"`
}

type HomeworkOverview struct {
	Tasks        int            `json:"tasks"`
	Students     int            `json:"students"`
	Completed    int            `json:"completed"`
	Incomplete   int            `json:"incomplete"`
	NotSubmitted int            `json:"not_submitted"`
	Statuses     map[string]int `json:"statuses"`
}

type Anomaly struct {
	Code  string `json:"code"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

type ClassOverview struct {
	SchoolClassID uint64 `json:"school_class_id"`
	ClassName     string `json:"class_name,omitempty"`
	Operations    int    `json:"operations"`
	Students      int    `json:"students"`
	Resolved      int    `json:"resolved"`
	Abnormal      int    `json:"abnormal"`
}
