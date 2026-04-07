package dto

import "encoding/json"

type Request struct {
	Action string          `json:"action"`
	Data   json.RawMessage `json:"data,omitempty"`
}

type Response struct {
	OK     bool   `json:"ok"`
	Action string `json:"action"`
	Error  string `json:"error,omitempty"`
	Data   any    `json:"data,omitempty"`
}

type MachineInput struct {
	Model          string  `json:"model"`
	PlateNumber    *string `json:"plate_number,omitempty"`
	SerialNumber   *string `json:"serial_number,omitempty"`
	CommissionYear *int    `json:"commission_year,omitempty"`
	Notes          *string `json:"notes,omitempty"`
}

type MachineEventInput struct {
	MachineID       int     `json:"machine_id"`
	EventDate       string  `json:"event_date"`
	StartHours      float64 `json:"start_hours"`
	EndHours        float64 `json:"end_hours"`
	OperationStatus *string `json:"operation_status,omitempty"`
	Location        *string `json:"location,omitempty"`
	TechnicalNotes  *string `json:"technical_notes,omitempty"`
}

type RepairActInput struct {
	MachineID     int     `json:"machine_id"`
	RepairType    string  `json:"repair_type"`
	StartDate     string  `json:"start_date"`
	EndDate       string  `json:"end_date"`
	HoursBefore   float64 `json:"hours_before"`
	HoursAfter    float64 `json:"hours_after"`
	StatusAfter   *string `json:"status_after,omitempty"`
	Conclusion    *string `json:"conclusion,omitempty"`
}

type RepairRequestInput struct {
	RequestStatus         *string `json:"request_status,omitempty"`
	MachineID             int     `json:"machine_id"`
	PriorityWeight        int     `json:"priority_weight"`
	MotoHoursAtRequest    float64 `json:"moto_hours_at_request"`
	ForecastCost          *float64 `json:"forecast_cost,omitempty"`
	RepairType            string  `json:"repair_type"`
	CriticalPartsRequired bool    `json:"critical_parts_required"`
	RequiredQualification *int    `json:"required_qualification,omitempty"`
	DesiredMonth          *string `json:"desired_month,omitempty"` // YYYY-MM-01
	Notes                 *string `json:"notes,omitempty"`
}

type TechCardInput struct {
	RepairType            string  `json:"repair_type"`
	Model                 string  `json:"model"`
	LaborHours            float64 `json:"labor_hours"`
	RequiredSpecialization string `json:"required_specialization"`
	RequiredQualification int     `json:"required_qualification"`
	OperationsDescription *string `json:"operations_description,omitempty"`
	Notes                 *string `json:"notes,omitempty"`
}

type MonthlyResourceInput struct {
	Month                  string  `json:"month"` // YYYY-MM-01
	AvailableHours         float64 `json:"available_hours"`
	Budget                 float64 `json:"budget"`
	MaxUnitsInRepair       int     `json:"max_units_in_repair"`
	CriticalPartsAvailable bool    `json:"critical_parts_available"`
	Notes                  *string `json:"notes,omitempty"`
}

type RepairPlanInput struct {
	RequestID        int     `json:"request_id"`
	AssignedMonth    string  `json:"assigned_month"` // YYYY-MM-01
	PlannedStartDate *string `json:"planned_start_date,omitempty"`
	PlannedEndDate   *string `json:"planned_end_date,omitempty"`
	PartsStatus      *string `json:"parts_status,omitempty"`
	AssignmentStatus *string `json:"assignment_status,omitempty"`
	RejectionReason  *string `json:"rejection_reason,omitempty"`
}

type MaterialNormInput struct {
	RepairType           string  `json:"repair_type"`
	Model                string  `json:"model"`
	MaterialName         string  `json:"material_name"`
	MaterialCode         string  `json:"material_code"`
	Unit                 string  `json:"unit"`
	ConsumptionPerRepair float64 `json:"consumption_per_repair"`
	Notes                *string `json:"notes,omitempty"`
}

type BrigadeInput struct {
	BrigadeNumber        int     `json:"brigade_number"`
	TeamComposition      *string `json:"team_composition,omitempty"`
	Specialization       string  `json:"specialization"`
	Qualification        int     `json:"qualification"`
	AvailableStart       string  `json:"available_start"` // YYYY-MM-DD
	AvailableEnd         string  `json:"available_end"`   // YYYY-MM-DD
	AvailableHours       float64 `json:"available_hours"`
	CurrentAssignedHours *float64 `json:"current_assigned_hours,omitempty"`
	Contact              *string `json:"contact,omitempty"`
	Notes                *string `json:"notes,omitempty"`
}

type AssignmentInput struct {
	RequestID         int     `json:"request_id"`
	BrigadeNumber     int     `json:"brigade_number"`
	StartDate         *string `json:"start_date,omitempty"`
	EndDate           *string `json:"end_date,omitempty"`
	PlannedHours      float64 `json:"planned_hours"`
	ResponsiblePerson *string `json:"responsible_person,omitempty"`
	AssignmentStatus  *string `json:"assignment_status,omitempty"`
	Notes             *string `json:"notes,omitempty"`
}