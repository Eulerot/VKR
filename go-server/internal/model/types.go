package model

type Machine struct {
	MachineID      int    `json:"machine_id"`
	Model          string `json:"model"`
	PlateNumber    string `json:"plate_number"`
	SerialNumber   string `json:"serial_number"`
	CommissionYear int    `json:"commission_year"`
	Notes          string `json:"notes"`
}

type CurrentRegistryRow struct {
	RowNo          int     `json:"row_no"`
	MachineID      int     `json:"machine_id"`
	PlateNumber    string  `json:"plate_number"`
	SerialNumber   string  `json:"serial_number"`
	Model          string  `json:"model"`
	TechnicalState string  `json:"technical_state"`
	OperationStatus string `json:"operation_status"`
	CurrentHours   float64 `json:"current_hours"`
	Location       string  `json:"location"`
	TechnicalNotes string  `json:"technical_notes"`
	Notes          string  `json:"notes"`
	Remarks        string  `json:"remarks"`
}

type CreateMachineRequest struct {
	Model          string  `json:"model"`
	PlateNumber    *string `json:"plate_number,omitempty"`
	SerialNumber   *string `json:"serial_number,omitempty"`
	CommissionYear *int    `json:"commission_year,omitempty"`
	Notes          *string `json:"notes,omitempty"`
}

type CreateMachineEventRequest struct {
	MachineID       int     `json:"machine_id"`
	EventDate       string  `json:"event_date"` // YYYY-MM-DD
	StartHours      float64 `json:"start_hours"`
	EndHours        float64 `json:"end_hours"`
	OperationStatus *string `json:"operation_status,omitempty"`
	Location        *string `json:"location,omitempty"`
	TechnicalNotes  *string `json:"technical_notes,omitempty"`
}

type CreateRepairActRequest struct {
	MachineID      int     `json:"machine_id"`
	RepairType     string  `json:"repair_type"`
	StartDate      string  `json:"start_date"` // YYYY-MM-DD
	EndDate        string  `json:"end_date"`   // YYYY-MM-DD
	HoursBefore    float64 `json:"hours_before"`
	HoursAfter     float64 `json:"hours_after"`
	StatusAfter    *string `json:"status_after,omitempty"`
	Conclusion     *string `json:"conclusion,omitempty"`
}