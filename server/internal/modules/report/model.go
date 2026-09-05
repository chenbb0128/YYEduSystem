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

// DailyExceptions contains actionable records for the staff workbench. It is
// deliberately separate from DailyOverview: overview is for metrics, while
// exceptions carry the identifiers needed to jump to the correct workflow.
type DailyExceptions struct {
	Date   string           `json:"date"`
	Items  []DailyException `json:"items"`
	Counts map[string]int   `json:"counts"`
}

type DailyException struct {
	ID            string `json:"id"`
	Code          string `json:"code"`
	Category      string `json:"category"`
	Severity      string `json:"severity"`
	Label         string `json:"label"`
	Message       string `json:"message"`
	SchoolClassID uint64 `json:"school_class_id,omitempty"`
	ClassName     string `json:"class_name,omitempty"`
	StudentID     uint64 `json:"student_id,omitempty"`
	StudentName   string `json:"student_name,omitempty"`
	OperationID   uint64 `json:"operation_id,omitempty"`
	TaskID        uint64 `json:"task_id,omitempty"`
	Action        string `json:"action"`
	Acknowledged  bool   `json:"acknowledged"`
	AcknowledgedAt string `json:"acknowledged_at,omitempty"`
	AcknowledgedBy string `json:"acknowledged_by,omitempty"`
}

type ClassOverview struct {
	SchoolClassID uint64 `json:"school_class_id"`
	ClassName     string `json:"class_name,omitempty"`
	Operations    int    `json:"operations"`
	Students      int    `json:"students"`
	Resolved      int    `json:"resolved"`
	Abnormal      int    `json:"abnormal"`
}
