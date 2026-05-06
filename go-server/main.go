package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"sort"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// =========================================================
// TCP JSON protocol
// one JSON object per line:
// {"action":"...","payload":{...}}
// =========================================================

type Request struct {
	Action  string          `json:"action"`
	Payload json.RawMessage `json:"payload"`
}

type Response struct {
	OK    bool        `json:"ok"`
	Data  any         `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

type Server struct {
	db *sql.DB
}

func main() {
	dsn := buildDSN()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("db ping: %v", err)
	}

	port := env("TCP_PORT", "8080")
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	srv := &Server{db: db}
	log.Printf("TCP server started on :%s", port)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("accept: %v", err)
			continue
		}
		go srv.handleConn(conn)
	}
}

func buildDSN() string {
	host := env("DB_HOST", "localhost")
	port := env("DB_PORT", "5432")
	user := env("DB_USER", "postgres")
	pass := env("DB_PASSWORD", "postgres")
	name := env("DB_NAME", "repair_planner")
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", host, port, user, pass, name)
}

func env(key, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return v
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	enc := json.NewEncoder(conn)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			_ = enc.Encode(Response{OK: false, Error: "invalid json: " + err.Error()})
			continue
		}

		resp := s.dispatch(req)
		if err := enc.Encode(resp); err != nil {
			return
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("conn scan error: %v", err)
	}
}

// =========================================================
// Dispatch
// =========================================================

func (s *Server) dispatch(req Request) Response {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	switch req.Action {
	case "ping":
		return ok(map[string]string{"message": "pong"})

	// ---------- 6.1 Машины / события / акты ----------
	case "units.list":
		return s.listQuery(ctx, `SELECT unit_id, unit_symbol, unit_name FROM units ORDER BY unit_id`)

	case "machines.list":
		return s.listQuery(ctx, `
			SELECT machine_id, model, plate_number, serial_number, commission_year, notes
			FROM machines
			ORDER BY machine_id
		`)
	case "machines.upsert":
		var p MachineUpsert
		if err := decode(req.Payload, &p); err != nil {
			return fail(err)
		}
		if err := s.upsertMachine(ctx, p); err != nil {
			return fail(err)
		}
		return ok(map[string]string{"status": "saved"})
	case "machines.delete":
		return fail(errors.New("deleting machines is disabled"))

	case "machine_events.list", "machineevents.list":
		return s.listQuery(ctx, `
			SELECT event_id, event_date, machine_id, driver_name, work_object, start_hours, end_hours,
			       operation_status, location, technical_notes
			FROM machine_events
			ORDER BY event_date DESC, event_id DESC
		`)
	case "machine_events.upsert", "machineevents.upsert":
		var p EventUpsert
		if err := decode(req.Payload, &p); err != nil {
			return fail(err)
		}
		if err := s.upsertMachineEvent(ctx, p); err != nil {
			return fail(err)
		}
		return ok(map[string]string{"status": "saved"})
	case "events.add":
		var p EventUpsert
		if err := decode(req.Payload, &p); err != nil {
			return fail(err)
		}
		p.EventID = nil
		if err := s.upsertMachineEvent(ctx, p); err != nil {
			return fail(err)
		}
		return ok(map[string]string{"status": "saved"})
	case "machine_events.delete", "machineevents.delete":
		var p EventIDPayload
		if err := decode(req.Payload, &p); err != nil {
			return fail(err)
		}
		return s.deleteExec(ctx, `DELETE FROM machine_events WHERE event_id = $1`, p.EventID)

	case "repair_acts.list", "repairacts.list":
		return s.listQuery(ctx, `
			SELECT repair_act_id, machine_id, repair_type, start_date, end_date, hours_before, hours_after, conclusion
			FROM repair_acts
			ORDER BY end_date DESC, repair_act_id DESC
		`)
	case "repair_acts.upsert", "repairacts.upsert":
		var p RepairActUpsert
		if err := decode(req.Payload, &p); err != nil {
			return fail(err)
		}
		if err := s.upsertRepairAct(ctx, p); err != nil {
			return fail(err)
		}
		return ok(map[string]string{"status": "saved"})
	case "acts.add":
		var p RepairActUpsert
		if err := decode(req.Payload, &p); err != nil {
			return fail(err)
		}
		p.RepairActID = nil
		if err := s.upsertRepairAct(ctx, p); err != nil {
			return fail(err)
		}
		return ok(map[string]string{"status": "saved"})
	case "repair_acts.delete", "repairacts.delete":
		var p RepairActIDPayload
		if err := decode(req.Payload, &p); err != nil {
			return fail(err)
		}
		return s.deleteExec(ctx, `DELETE FROM repair_acts WHERE repair_act_id = $1`, p.RepairActID)

	case "registry.get":
		rows, err := s.buildRegistry(ctx)
		if err != nil {
			return fail(err)
		}
		return ok(rows)

	// ---------- 6.2 Заявки / карты / ресурсы / годовой план ----------
	case "repair_requests.list", "repairrequests.list":
		return s.listQuery(ctx, `
			SELECT request_id, request_status, machine_id, model, priority_weight, motohours_at_request,
			       forecast_cost, repair_type, critical_parts_required, required_qualification, desired_month, notes
			FROM repair_requests
			ORDER BY priority_weight DESC, COALESCE(desired_month, 13), request_id
		`)
	case "repair_requests.upsert", "repairrequests.upsert":
		var p RepairRequestUpsert
		if err := decode(req.Payload, &p); err != nil {
			return fail(err)
		}
		if err := s.upsertRepairRequest(ctx, p); err != nil {
			return fail(err)
		}
		return ok(map[string]string{"status": "saved"})
	case "repair_requests.delete", "repairrequests.delete":
		var p RequestIDPayload
		if err := decode(req.Payload, &p); err != nil {
			return fail(err)
		}
		return s.deleteExec(ctx, `DELETE FROM repair_requests WHERE request_id = $1`, p.RequestID)

	case "repair_tech_cards.list", "repairtechcards.list":
		return s.listQuery(ctx, `SELECT rtc.techcard_id, rtc.repair_type, rtc.machine_id, m.model, rtc.labor_hours, rtc.required_qualification, rtc.operations_description, rtc.notes
		FROM repair_tech_cards rtc
		LEFT JOIN machines m ON m.machine_id = rtc.machine_id
		ORDER BY rtc.repair_type, rtc.machine_id
	`)
	case "repair_tech_cards.upsert", "repairtechcards.upsert":
		var p TechCardUpsert
		if err := decode(req.Payload, &p); err != nil {
			return fail(err)
		}
		if err := s.upsertTechCard(ctx, p); err != nil {
			return fail(err)
		}
		return ok(map[string]string{"status": "saved"})
	case "repair_tech_cards.delete", "repairtechcards.delete":
		var p TechCardKeyPayload
		if err := decode(req.Payload, &p); err != nil {
		return fail(err)
		}
		return s.deleteExec(ctx, `DELETE FROM repair_tech_cards WHERE repair_type = $1 AND machine_id = $2`, p.RepairType, p.MachineID)

	case "monthly_resources.list", "monthlyresources.list":
		return s.listQuery(ctx, `
			SELECT resource_id, month_no, available_hours, budget, max_units_in_repair, critical_parts_available, notes
			FROM monthly_resources
			ORDER BY month_no
		`)
	case "monthly_resources.upsert", "monthlyresources.upsert":
		var p MonthlyResourceUpsert
		if err := decode(req.Payload, &p); err != nil {
			return fail(err)
		}
		if err := s.upsertMonthlyResource(ctx, p); err != nil {
			return fail(err)
		}
		return ok(map[string]string{"status": "saved"})
	case "monthly_resources.delete", "monthlyresources.delete":
		var p MonthNoPayload
		if err := decode(req.Payload, &p); err != nil {
			return fail(err)
		}
		return s.deleteExec(ctx, `DELETE FROM monthly_resources WHERE month_no = $1`, p.MonthNo)

	case "repair_plan.list", "repairplan.list":
		return s.listQuery(ctx, `
			SELECT plan_id, request_id, machine_id, model, repair_type, required_qualification,
			       labor_hours, priority_weight, forecast_cost, assigned_month
			FROM repair_plan
			ORDER BY priority_weight DESC, assigned_month, request_id
		`)
	case "annual_plan.list":
		return s.listQuery(ctx, `
			SELECT plan_id, request_id, machine_id, model, repair_type, required_qualification,
			       labor_hours, priority_weight, forecast_cost, assigned_month
			FROM repair_plan
			ORDER BY priority_weight DESC, assigned_month, request_id
		`)
	case "annual_plan.solve":
		var p AnnualPlanRequest
		if err := decode(req.Payload, &p); err != nil {
			return fail(err)
		}
		rows, err := s.solveAnnualPlan(ctx, p.Year)
		if err != nil {
			return fail(err)
		}
		return ok(rows)

	// ---------- 6.3 Материалы ----------
	case "materials.list":
		return s.listQuery(ctx, `
			SELECT m.material_code, m.material_name, m.unit_id, u.unit_symbol
			FROM materials m
			LEFT JOIN units u ON u.unit_id = m.unit_id
			ORDER BY m.material_code
		`)
	case "materials.upsert":
		var p MaterialUpsert
		if err := decode(req.Payload, &p); err != nil {
			return fail(err)
		}
		if err := s.upsertMaterial(ctx, p); err != nil {
			return fail(err)
		}
		return ok(map[string]string{"status": "saved"})
	case "materials.delete":
		var p MaterialCodePayload
		if err := decode(req.Payload, &p); err != nil {
			return fail(err)
		}
		return s.deleteExec(ctx, `DELETE FROM materials WHERE material_code = $1`, p.MaterialCode)

	case "material_norms.list", "materialnorms.list":
		return s.listQuery(ctx, `
			SELECT norm_id, repair_type, model, material_code, consumption_per_repair
			FROM material_norms
			ORDER BY repair_type, model, material_code
		`)
	case "material_norms.upsert", "materialnorms.upsert":
		var p MaterialNormUpsert
		if err := decode(req.Payload, &p); err != nil {
			return fail(err)
		}
		if err := s.upsertMaterialNorm(ctx, p); err != nil {
			return fail(err)
		}
		return ok(map[string]string{"status": "saved"})
	case "material_norms.delete", "materialnorms.delete":
		var p MaterialNormKeyPayload
		if err := decode(req.Payload, &p); err != nil {
			return fail(err)
		}
		return s.deleteExec(ctx, `DELETE FROM material_norms WHERE repair_type = $1 AND model = $2 AND material_code = $3`, p.RepairType, p.Model, p.MaterialCode)

	case "materials.solve":
		var p MaterialDemandRequest
		if err := decode(req.Payload, &p); err != nil {
			return fail(err)
		}
		rows, err := s.solveMaterialDemand(ctx, p.TargetMonth)
		if err != nil {
			return fail(err)
		}
		return ok(rows)
	case "material_demand.list", "materialdemand.list":
		return s.listQuery(ctx, `
			SELECT demand_id, target_month, material_code, demand_quantity, notes
			FROM material_demand
			ORDER BY target_month, material_code
		`)

	// ---------- 6.4 Бригады / доступность / месячный план ----------
	case "brigades.list":
		return s.listQuery(ctx, `
			SELECT brigade_number, team_composition, specialization, qualification, contact, notes
			FROM brigades
			ORDER BY brigade_number
		`)
	case "brigades.upsert":
		var p BrigadeUpsert
		if err := decode(req.Payload, &p); err != nil {
			return fail(err)
		}
		if err := s.upsertBrigade(ctx, p); err != nil {
			return fail(err)
		}
		return ok(map[string]string{"status": "saved"})
	case "brigades.delete":
		var p BrigadeNumberPayload
		if err := decode(req.Payload, &p); err != nil {
			return fail(err)
		}
		return s.deleteExec(ctx, `DELETE FROM brigades WHERE brigade_number = $1`, p.BrigadeNumber)

	case "brigade_availability.list", "brigadeavailability.list":
		return s.listQuery(ctx, `
			SELECT availability_id, brigade_number, available_start, available_end, available_hours,
			       current_assigned_hours, contact, notes
			FROM brigade_availability
			ORDER BY brigade_number, available_start, available_end
		`)
	case "brigade_availability.upsert", "brigadeavailability.upsert":
		var p BrigadeAvailabilityUpsert
		if err := decode(req.Payload, &p); err != nil {
			return fail(err)
		}
		if err := s.upsertBrigadeAvailability(ctx, p); err != nil {
			return fail(err)
		}
		return ok(map[string]string{"status": "saved"})
	case "brigade_availability.delete", "brigadeavailability.delete":
		var p AvailabilityIDPayload
		if err := decode(req.Payload, &p); err != nil {
			return fail(err)
		}
		return s.deleteExec(ctx, `DELETE FROM brigade_availability WHERE availability_id = $1`, p.AvailabilityID)

	case "monthly_repair_plan.list", "monthlyrepairplan.list":
		return s.listQuery(ctx, `
			SELECT monthly_plan_id, request_id, machine_id, model, repair_type, required_specialization,
			       required_qualification, planned_start_date, planned_end_date, labor_hours,
			       priority_weight, readiness_status, notes
			FROM monthly_repair_plan
			ORDER BY priority_weight DESC, planned_start_date, request_id
		`)
	case "monthly_repair_plan.upsert", "monthlyrepairplan.upsert":
		var p MonthlyRepairPlanUpsert
		if err := decode(req.Payload, &p); err != nil {
			return fail(err)
		}
		if err := s.upsertMonthlyRepairPlan(ctx, p); err != nil {
			return fail(err)
		}
		return ok(map[string]string{"status": "saved"})
	case "monthly_repair_plan.delete", "monthlyrepairplan.delete":
		var p RequestIDPayload
		if err := decode(req.Payload, &p); err != nil {
			return fail(err)
		}
		return s.deleteExec(ctx, `DELETE FROM monthly_repair_plan WHERE request_id = $1`, p.RequestID)

	case "repair_assignments.list", "repairassignments.list":
		var p MonthFilterRequest
		if err := decode(req.Payload, &p); err != nil {
			return fail(err)
		}
		rows, err := s.listBrigadeAssignments(ctx, p.Month)
		if err != nil {
			return fail(err)
		}
		return ok(rows)

	case "brigade_assignments.solve":
		var p MonthFilterRequest
		if err := decode(req.Payload, &p); err != nil {
			return fail(err)
		}
		rows, err := s.solveBrigadeAssignments(ctx, p.Month)
		if err != nil {
			return fail(err)
		}
		return ok(rows)

	case "brigade_assignments.list":
		var p MonthFilterRequest
		if err := decode(req.Payload, &p); err != nil {
			return fail(err)
		}
		rows, err := s.listBrigadeAssignments(ctx, p.Month)
		if err != nil {
			return fail(err)
		}
		return ok(rows)

	default:
		return fail(fmt.Errorf("unknown action: %s", req.Action))
	}
}

func ok(data any) Response {
	return Response{OK: true, Data: data}
}

func fail(err error) Response {
	return Response{OK: false, Error: err.Error()}
}

func decode[T any](raw json.RawMessage, dst *T) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, dst)
}

func parseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse("2006-01-02", s)
}

func firstWord(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\t'
	})
	if len(fields) > 0 {
		return strings.TrimSpace(fields[0])
	}
	return s
}

func boolText(s string) bool {
	return strings.EqualFold(strings.TrimSpace(s), "да")
}

func isAllowedSpecialization(s string) bool {
	switch strings.TrimSpace(s) {
	case "слесарь", "электрик", "сварщик", "универсальная":
		return true
	default:
		return false
	}
}

func nullString(v sql.NullString) string {
	if v.Valid {
		return v.String
	}
	return "—"
}

func float64Value(v sql.NullFloat64) float64 {
	if v.Valid {
		return v.Float64
	}
	return 0
}

func intOrMax(v sql.NullInt64, max int) int {
	if v.Valid {
		return int(v.Int64)
	}
	return max
}

func nullDate(t sql.NullTime) any {
	if t.Valid {
		return t.Time
	}
	return nil
}

// =========================================================
// Payload types
// =========================================================

type AnnualPlanRequest struct {
	Year *int `json:"year,omitempty"`
}

type MonthFilterRequest struct {
	Month *int `json:"month,omitempty"`
}

type MachineIDPayload struct {
	MachineID string `json:"machine_id"`
}

type EventIDPayload struct {
	EventID int `json:"event_id"`
}

type RepairActIDPayload struct {
	RepairActID int `json:"repair_act_id"`
}

type RequestIDPayload struct {
	RequestID string `json:"request_id"`
}

type BrigadeNumberPayload struct {
	BrigadeNumber string `json:"brigade_number"`
}

type AvailabilityIDPayload struct {
	AvailabilityID int `json:"availability_id"`
}

type MonthNoPayload struct {
	MonthNo int `json:"month_no"`
}

type MaterialCodePayload struct {
	MaterialCode string `json:"material_code"`
}

type TechCardKeyPayload struct {
	RepairType string `json:"repair_type"`
	MachineID   string `json:"machine_id"`
}

type MaterialNormKeyPayload struct {
	RepairType   string `json:"repair_type"`
	Model        string `json:"model"`
	MaterialCode string `json:"material_code"`
}

type MachineUpsert struct {
	MachineID      string  `json:"machine_id"`
	Model          string  `json:"model"`
	PlateNumber    *string `json:"plate_number"`
	SerialNumber   *string `json:"serial_number"`
	CommissionYear *int    `json:"commission_year"`
	Notes          *string `json:"notes"`
}

type EventUpsert struct {
	EventID         *int    `json:"event_id,omitempty"`
	EventDate       string  `json:"event_date"`
	MachineID       string  `json:"machine_id"`
	DriverName      *string `json:"driver_name"`
	WorkObject      *string `json:"work_object"`
	StartHours      *int    `json:"start_hours"`
	EndHours        *int    `json:"end_hours"`
	OperationStatus *string `json:"operation_status"`
	Location        *string `json:"location"`
	TechnicalNotes  string  `json:"technical_notes"`
}

type RepairActUpsert struct {
	RepairActID *int    `json:"repair_act_id,omitempty"`
	MachineID   string  `json:"machine_id"`
	RepairType  string  `json:"repair_type"`
	StartDate   *string `json:"start_date"`
	EndDate     string  `json:"end_date"`
	HoursBefore *int    `json:"hours_before"`
	HoursAfter  *int    `json:"hours_after"`
	Conclusion  string  `json:"conclusion"`
}

type RepairRequestUpsert struct {
	RequestID             string   `json:"request_id"`
	RequestStatus         string   `json:"request_status"`
	MachineID             string   `json:"machine_id"`
	Model                 string   `json:"model"`
	PriorityWeight        int      `json:"priority_weight"`
	MotoHoursAtRequest    *int     `json:"moto_hours_at_request"`
	ForecastCost          *float64 `json:"forecast_cost"`
	RepairType            string   `json:"repair_type"`
	CriticalPartsRequired string   `json:"critical_parts_required"`
	RequiredQualification int      `json:"required_qualification"`
	DesiredMonth          *int     `json:"desired_month"`
	Notes                 *string  `json:"notes"`
}

type TechCardUpsert struct {
	RepairType            string  `json:"repair_type"`
	MachineID             string  `json:"machine_id"`
	LaborHours            int     `json:"labor_hours"`
	RequiredQualification int     `json:"required_qualification"`
	OperationsDescription *string `json:"operations_description"`
	Notes                 *string `json:"notes"`
}
type MonthlyResourceUpsert struct {
	MonthNo                int      `json:"month_no"`
	AvailableHours         *int     `json:"available_hours"`
	Budget                 *float64 `json:"budget"`
	MaxUnitsInRepair       *int     `json:"max_units_in_repair"`
	CriticalPartsAvailable string   `json:"critical_parts_available"`
	Notes                  *string  `json:"notes"`
}

type MaterialUpsert struct {
	MaterialCode string `json:"material_code"`
	MaterialName string `json:"material_name"`
	UnitID       int    `json:"unit_id"`
}

type MaterialNormUpsert struct {
	RepairType           string  `json:"repair_type"`
	Model                string  `json:"model"`
	MaterialCode         string  `json:"material_code"`
	ConsumptionPerRepair float64 `json:"consumption_per_repair"`
}

type MaterialDemandRequest struct {
	TargetMonth int `json:"target_month"`
}

type BrigadeUpsert struct {
	BrigadeNumber   string  `json:"brigade_number"`
	TeamComposition string  `json:"team_composition"`
	Specialization  string  `json:"specialization"`
	Qualification   int     `json:"qualification"`
	Contact         *string `json:"contact"`
	Notes           *string `json:"notes"`
}

type BrigadeAvailabilityUpsert struct {
	AvailabilityID      *int    `json:"availability_id,omitempty"`
	BrigadeNumber       string  `json:"brigade_number"`
	AvailableStart      string  `json:"available_start"`
	AvailableEnd        string  `json:"available_end"`
	AvailableHours      *int    `json:"available_hours"`
	CurrentAssignedHours *int   `json:"current_assigned_hours"`
	Contact             *string `json:"contact"`
	Notes               *string `json:"notes"`
}

type MonthlyRepairPlanUpsert struct {
	RequestID              string  `json:"request_id"`
	MachineID              string  `json:"machine_id"`
	Model                  string  `json:"model"`
	RepairType             string  `json:"repair_type"`
	RequiredSpecialization string  `json:"required_specialization"`
	RequiredQualification  int     `json:"required_qualification"`
	PlannedStartDate       *string `json:"planned_start_date"`
	PlannedEndDate         *string `json:"planned_end_date"`
	LaborHours             int     `json:"labor_hours"`
	PriorityWeight         int     `json:"priority_weight"`
	ReadinessStatus        string  `json:"readiness_status"`
	Notes                  *string `json:"notes"`
}

// =========================================================
// Output types
// =========================================================

type RegistryRow struct {
	MachineID        string `json:"machine_id"`
	PlateNumber      string `json:"plate_number"`
	SerialNumber     string `json:"serial_number"`
	Model            string `json:"model"`
	TechnicalState   string `json:"technical_state"`
	OperationStatus  string `json:"operation_status"`
	Hours            any    `json:"hours"`
	Location         string `json:"location"`
	LastDocumentType string `json:"last_document_type"`
	LastDocumentDate string `json:"last_document_date"`
}

type AnnualPlanRow struct {
	RequestID             string  `json:"request_id"`
	MachineID             string  `json:"machine_id"`
	Model                 string  `json:"model"`
	RepairType            string  `json:"repair_type"`
	RequiredQualification int     `json:"required_qualification"`
	LaborHours            int     `json:"labor_hours"`
	PriorityWeight        int     `json:"priority_weight"`
	ForecastCost          float64 `json:"forecast_cost"`
	AssignedMonth         int     `json:"assigned_month"`
}

type MaterialDemandRow struct {
	MaterialCode   string  `json:"material_code"`
	MaterialName   string  `json:"material_name"`
	Unit           string  `json:"unit"`
	DemandQuantity float64 `json:"demand_quantity"`
	Notes          string  `json:"notes"`
}

type BrigadeAssignmentRow struct {
	RequestID         string `json:"request_id"`
	MachineID         string `json:"machine_id"`
	Model             string `json:"model"`
	RepairType        string `json:"repair_type"`
	StartDate         string `json:"start_date"`
	EndDate           string `json:"end_date"`
	BrigadeNumber     string `json:"brigade_number"`
	Specialization    string `json:"specialization"`
	PlannedHours      int    `json:"planned_hours"`
	ResponsiblePerson string `json:"responsible_person"`
	AssignmentStatus  string `json:"assignment_status"`
	Notes             string `json:"notes"`
}

// =========================================================
// Generic helpers
// =========================================================

func (s *Server) listQuery(ctx context.Context, query string, args ...any) Response {
	rows, err := s.queryRows(ctx, query, args...)
	if err != nil {
		return fail(err)
	}
	return ok(rows)
}

func (s *Server) deleteExec(ctx context.Context, query string, args ...any) Response {
	if _, err := s.db.ExecContext(ctx, query, args...); err != nil {
		return fail(err)
	}
	return ok(map[string]string{"status": "deleted"})
}

func (s *Server) queryRows(ctx context.Context, query string, args ...any) ([]map[string]any, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	out := make([]map[string]any, 0)
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}

		row := make(map[string]any, len(cols))
		for i, c := range cols {
			row[c] = normalizeDBValue(vals[i])
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func normalizeDBValue(v any) any {
	switch t := v.(type) {
	case nil:
		return nil
	case []byte:
		return string(t)
	case time.Time:
		return t.Format("2006-01-02")
	default:
		return t
	}
}

// =========================================================
// Upsert / delete methods for base tables
// =========================================================

func (s *Server) upsertMachine(ctx context.Context, p MachineUpsert) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO machines (machine_id, model, plate_number, serial_number, commission_year, notes)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (machine_id) DO UPDATE SET
			model = EXCLUDED.model,
			plate_number = EXCLUDED.plate_number,
			serial_number = EXCLUDED.serial_number,
			commission_year = EXCLUDED.commission_year,
			notes = EXCLUDED.notes
	`, p.MachineID, p.Model, p.PlateNumber, p.SerialNumber, p.CommissionYear, p.Notes)
	return err
}

func (s *Server) upsertMachineEvent(ctx context.Context, p EventUpsert) error {
	dt, err := parseDate(p.EventDate)
	if err != nil {
		return err
	}
	if p.TechnicalNotes != "исправна" && p.TechnicalNotes != "неисправна" {
		return errors.New("technical_notes must be 'исправна' or 'неисправна'")
	}

	if p.EventID != nil {
		_, err = s.db.ExecContext(ctx, `
			UPDATE machine_events
			SET event_date = $1,
			    machine_id = $2,
			    driver_name = $3,
			    work_object = $4,
			    start_hours = $5,
			    end_hours = $6,
			    operation_status = $7,
			    location = $8,
			    technical_notes = $9
			WHERE event_id = $10
		`, dt, p.MachineID, p.DriverName, p.WorkObject, p.StartHours, p.EndHours, p.OperationStatus, p.Location, p.TechnicalNotes, p.EventID)
		return err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO machine_events
			(event_date, machine_id, driver_name, work_object, start_hours, end_hours, operation_status, location, technical_notes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, dt, p.MachineID, p.DriverName, p.WorkObject, p.StartHours, p.EndHours, p.OperationStatus, p.Location, p.TechnicalNotes)
	return err
}

func (s *Server) upsertRepairAct(ctx context.Context, p RepairActUpsert) error {
	end, err := parseDate(p.EndDate)
	if err != nil {
		return err
	}

	var start any
	if p.StartDate != nil {
		t, err := parseDate(*p.StartDate)
		if err != nil {
			return err
		}
		start = t
	}

	if p.Conclusion != "исправна" && p.Conclusion != "неисправна" {
		return errors.New("conclusion must be 'исправна' or 'неисправна'")
	}

	if p.RepairActID != nil {
		_, err = s.db.ExecContext(ctx, `
			UPDATE repair_acts
			SET machine_id = $1,
			    repair_type = $2,
			    start_date = $3,
			    end_date = $4,
			    hours_before = $5,
			    hours_after = $6,
			    conclusion = $7
			WHERE repair_act_id = $8
		`, p.MachineID, p.RepairType, start, end, p.HoursBefore, p.HoursAfter, p.Conclusion, p.RepairActID)
		return err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO repair_acts
			(machine_id, repair_type, start_date, end_date, hours_before, hours_after, conclusion)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, p.MachineID, p.RepairType, start, end, p.HoursBefore, p.HoursAfter, p.Conclusion)
	return err
}

func (s *Server) upsertRepairRequest(ctx context.Context, p RepairRequestUpsert) error {
	if p.CriticalPartsRequired != "да" && p.CriticalPartsRequired != "нет" {
		return errors.New("critical_parts_required must be 'да' or 'нет'")
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO repair_requests
			(request_id, request_status, machine_id, model, priority_weight, motohours_at_request, forecast_cost,
			 repair_type, critical_parts_required, required_qualification, desired_month, notes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (request_id) DO UPDATE SET
			request_status = EXCLUDED.request_status,
			machine_id = EXCLUDED.machine_id,
			model = EXCLUDED.model,
			priority_weight = EXCLUDED.priority_weight,
			motohours_at_request = EXCLUDED.motohours_at_request,
			forecast_cost = EXCLUDED.forecast_cost,
			repair_type = EXCLUDED.repair_type,
			critical_parts_required = EXCLUDED.critical_parts_required,
			required_qualification = EXCLUDED.required_qualification,
			desired_month = EXCLUDED.desired_month,
			notes = EXCLUDED.notes
	`, p.RequestID, p.RequestStatus, p.MachineID, p.Model, p.PriorityWeight, p.MotoHoursAtRequest, p.ForecastCost, p.RepairType, p.CriticalPartsRequired, p.RequiredQualification, p.DesiredMonth, p.Notes)
	return err
}

func (s *Server) upsertTechCard(ctx context.Context, p TechCardUpsert) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO repair_tech_cards
			(repair_type, machine_id, labor_hours, required_qualification, operations_description, notes)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (repair_type, machine_id) DO UPDATE SET
			labor_hours = EXCLUDED.labor_hours,
			required_qualification = EXCLUDED.required_qualification,
			operations_description = EXCLUDED.operations_description,
			notes = EXCLUDED.notes
	`, p.RepairType, p.MachineID, p.LaborHours, p.RequiredQualification, p.OperationsDescription, p.Notes)
	return err
}

func (s *Server) upsertMonthlyResource(ctx context.Context, p MonthlyResourceUpsert) error {
	if p.CriticalPartsAvailable != "да" && p.CriticalPartsAvailable != "нет" {
		return errors.New("critical_parts_available must be 'да' or 'нет'")
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO monthly_resources
			(month_no, available_hours, budget, max_units_in_repair, critical_parts_available, notes)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (month_no) DO UPDATE SET
			available_hours = EXCLUDED.available_hours,
			budget = EXCLUDED.budget,
			max_units_in_repair = EXCLUDED.max_units_in_repair,
			critical_parts_available = EXCLUDED.critical_parts_available,
			notes = EXCLUDED.notes
	`, p.MonthNo, p.AvailableHours, p.Budget, p.MaxUnitsInRepair, p.CriticalPartsAvailable, p.Notes)
	return err
}

func (s *Server) upsertMaterial(ctx context.Context, p MaterialUpsert) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO materials (material_code, material_name, unit_id)
		VALUES ($1,$2,$3)
		ON CONFLICT (material_code) DO UPDATE SET
			material_name = EXCLUDED.material_name,
			unit_id = EXCLUDED.unit_id
	`, p.MaterialCode, p.MaterialName, p.UnitID)
	return err
}

func (s *Server) upsertMaterialNorm(ctx context.Context, p MaterialNormUpsert) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO material_norms (repair_type, model, material_code, consumption_per_repair)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (repair_type, model, material_code) DO UPDATE SET
			consumption_per_repair = EXCLUDED.consumption_per_repair
	`, p.RepairType, p.Model, p.MaterialCode, p.ConsumptionPerRepair)
	return err
}

func (s *Server) upsertBrigade(ctx context.Context, p BrigadeUpsert) error {
	log.Printf("received required_specialization: '%s'", p.Specialization)

	if !isAllowedSpecialization(p.Specialization) {
		return errors.New("specialization must be: слесарь, электрик, сварщик, универсальная")
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO brigades (brigade_number, team_composition, specialization, qualification, contact, notes)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (brigade_number) DO UPDATE SET
			team_composition = EXCLUDED.team_composition,
			specialization = EXCLUDED.specialization,
			qualification = EXCLUDED.qualification,
			contact = EXCLUDED.contact,
			notes = EXCLUDED.notes
	`, p.BrigadeNumber, p.TeamComposition, p.Specialization, p.Qualification, p.Contact, p.Notes)
	return err
}

func (s *Server) upsertBrigadeAvailability(ctx context.Context, p BrigadeAvailabilityUpsert) error {

	start, err := parseDate(p.AvailableStart)
	if err != nil {
		return err
	}
	end, err := parseDate(p.AvailableEnd)
	if err != nil {
		return err
	}

	if p.AvailabilityID != nil {
		_, err = s.db.ExecContext(ctx, `
			UPDATE brigade_availability
			SET brigade_number = $1,
			    available_start = $2,
			    available_end = $3,
			    available_hours = $4,
			    current_assigned_hours = COALESCE($5, 0),
			    contact = $6,
			    notes = $7
			WHERE availability_id = $8
		`, p.BrigadeNumber, start, end, p.AvailableHours, p.CurrentAssignedHours, p.Contact, p.Notes, p.AvailabilityID)
		return err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO brigade_availability
			(brigade_number, available_start, available_end, available_hours, current_assigned_hours, contact, notes)
		VALUES ($1,$2,$3,$4,COALESCE($5,0),$6,$7)
	`, p.BrigadeNumber, start, end, p.AvailableHours, p.CurrentAssignedHours, p.Contact, p.Notes)
	return err
}

func (s *Server) upsertMonthlyRepairPlan(ctx context.Context, p MonthlyRepairPlanUpsert) error {
	log.Printf("received required_specialization: '%s'", p.RequiredSpecialization)
	log.Printf("hex: % x", []byte(p.RequiredSpecialization))
	log.Printf("length: %d", len(p.RequiredSpecialization))
	if !isAllowedSpecialization(p.RequiredSpecialization) {
		return errors.New("required_specialization must be: слесарь, электрик, сварщик, универсальная")
	}

	var start any
	if p.PlannedStartDate != nil && strings.TrimSpace(*p.PlannedStartDate) != "" {
		t, err := parseDate(*p.PlannedStartDate)
		if err != nil {
			return err
		}
		start = t
	}

	var end any
	if p.PlannedEndDate != nil && strings.TrimSpace(*p.PlannedEndDate) != "" {
		t, err := parseDate(*p.PlannedEndDate)
		if err != nil {
			return err
		}
		end = t
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO monthly_repair_plan
			(request_id, machine_id, model, repair_type, required_specialization, required_qualification,
			 planned_start_date, planned_end_date, labor_hours, priority_weight, readiness_status, notes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (request_id) DO UPDATE SET
			machine_id = EXCLUDED.machine_id,
			model = EXCLUDED.model,
			repair_type = EXCLUDED.repair_type,
			required_specialization = EXCLUDED.required_specialization,
			required_qualification = EXCLUDED.required_qualification,
			planned_start_date = EXCLUDED.planned_start_date,
			planned_end_date = EXCLUDED.planned_end_date,
			labor_hours = EXCLUDED.labor_hours,
			priority_weight = EXCLUDED.priority_weight,
			readiness_status = EXCLUDED.readiness_status,
			notes = EXCLUDED.notes
	`, p.RequestID, p.MachineID, p.Model, p.RepairType, p.RequiredSpecialization, p.RequiredQualification, start, end, p.LaborHours, p.PriorityWeight, p.ReadinessStatus, p.Notes)
	return err
}

// =========================================================
// 6.1 — реестр состояния машин
// =========================================================

type machineEventRow struct {
	EventDate       time.Time
	OperationStatus sql.NullString
	Location        sql.NullString
	EndHours        sql.NullInt64
	TechnicalNotes  string
}

type repairActRow struct {
	EndDate    time.Time
	HoursAfter sql.NullInt64
	Conclusion string
}

func (s *Server) buildRegistry(ctx context.Context) ([]RegistryRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT machine_id, model, plate_number, serial_number
		FROM machines
		ORDER BY machine_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RegistryRow
	for rows.Next() {
		var machineID, model, plate, serial sql.NullString
		if err := rows.Scan(&machineID, &model, &plate, &serial); err != nil {
			return nil, err
		}

		event, evDate, hasEvent, err := s.lastMachineEvent(ctx, machineID.String)
		if err != nil {
			return nil, err
		}
		act, actDate, hasAct, err := s.lastRepairAct(ctx, machineID.String)
		if err != nil {
			return nil, err
		}

		row := RegistryRow{
			MachineID:        machineID.String,
			PlateNumber:      nullString(plate),
			SerialNumber:     nullString(serial),
			Model:            nullString(model),
			TechnicalState:   "—",
			OperationStatus:  "—",
			Hours:            "—",
			Location:         "—",
			LastDocumentType: "—",
			LastDocumentDate: "—",
		}

		switch {
		case hasAct && (!hasEvent || actDate.After(evDate) || actDate.Equal(evDate)):
			row.LastDocumentType = "repair_acts"
			row.LastDocumentDate = actDate.Format("2006-01-02")
			row.TechnicalState = act.Conclusion
			row.OperationStatus = "простой"
			if act.HoursAfter.Valid {
				row.Hours = act.HoursAfter.Int64
			}
			row.Location = "ангар"
		case hasEvent:
			row.LastDocumentType = "machine_events"
			row.LastDocumentDate = evDate.Format("2006-01-02")
			row.TechnicalState = event.TechnicalNotes
			row.OperationStatus = nullString(event.OperationStatus)
			if event.EndHours.Valid {
				row.Hours = event.EndHours.Int64
			}
			row.Location = nullString(event.Location)
		}

		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Server) lastMachineEvent(ctx context.Context, machineID string) (machineEventRow, time.Time, bool, error) {
	var e machineEventRow
	row := s.db.QueryRowContext(ctx, `
		SELECT event_date, operation_status, location, end_hours, technical_notes
		FROM machine_events
		WHERE machine_id = $1
		ORDER BY event_date DESC, event_id DESC
		LIMIT 1
	`, machineID)
	if err := row.Scan(&e.EventDate, &e.OperationStatus, &e.Location, &e.EndHours, &e.TechnicalNotes); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return machineEventRow{}, time.Time{}, false, nil
		}
		return machineEventRow{}, time.Time{}, false, err
	}
	return e, e.EventDate, true, nil
}

func (s *Server) lastRepairAct(ctx context.Context, machineID string) (repairActRow, time.Time, bool, error) {
	var a repairActRow
	row := s.db.QueryRowContext(ctx, `
		SELECT end_date, hours_after, conclusion
		FROM repair_acts
		WHERE machine_id = $1
		ORDER BY end_date DESC, repair_act_id DESC
		LIMIT 1
	`, machineID)
	if err := row.Scan(&a.EndDate, &a.HoursAfter, &a.Conclusion); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repairActRow{}, time.Time{}, false, nil
		}
		return repairActRow{}, time.Time{}, false, err
	}
	return a, a.EndDate, true, nil
}

// =========================================================
// 6.2 — годовое планирование ремонтов
// =========================================================

type annualReq struct {
	RequestID             string
	MachineID             string
	Model                 string
	RepairType            string
	PriorityWeight        int
	MotoHoursAtRequest    sql.NullInt64
	ForecastCost          sql.NullFloat64
	CriticalPartsRequired string
	RequiredQualification int
	DesiredMonth          sql.NullInt64
	Notes                 sql.NullString
}

type techCard struct {
	RepairType            string
	MachineID             string
	LaborHours            int
	RequiredQualification int
}

type monthRes struct {
	MonthNo                int
	AvailableHours         int
	Budget                 float64
	MaxUnitsInRepair       int
	CriticalPartsAvailable string
}

type annualAssignment struct {
	RequestID             string
	MachineID             string
	Model                 string
	RepairType            string
	RequiredQualification int
	LaborHours            int
	PriorityWeight        int
	ForecastCost          float64
	AssignedMonth         int
}

func (s *Server) solveAnnualPlan(ctx context.Context, year *int) ([]AnnualPlanRow, error) {
	_ = year // year is accepted by the client/UI, but the plan is stored without year in this schema.

	reqs, err := s.loadAnnualRequests(ctx)
	if err != nil {
		return nil, err
	}
	if len(reqs) == 0 {
		return nil, nil
	}

	tech, err := s.loadTechCards(ctx)
	if err != nil {
		return nil, err
	}
	res, err := s.loadMonthlyResources(ctx)
	if err != nil {
		return nil, err
	}

	sort.Slice(reqs, func(i, j int) bool {
		di := intOrMax(reqs[i].DesiredMonth, 13)
		dj := intOrMax(reqs[j].DesiredMonth, 13)
		if reqs[i].PriorityWeight != reqs[j].PriorityWeight {
			return reqs[i].PriorityWeight > reqs[j].PriorityWeight
		}
		if di != dj {
			return di < dj
		}
		return reqs[i].RequestID < reqs[j].RequestID
	})

	assignments, err := solveAnnualAssignments(reqs, tech, res)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM repair_plan`); err != nil {
		return nil, err
	}

	out := make([]AnnualPlanRow, 0, len(assignments))
	for _, a := range assignments {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO repair_plan
				(request_id, machine_id, model, repair_type, required_qualification, labor_hours, priority_weight, forecast_cost, assigned_month)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT (request_id) DO UPDATE SET
				machine_id = EXCLUDED.machine_id,
				model = EXCLUDED.model,
				repair_type = EXCLUDED.repair_type,
				required_qualification = EXCLUDED.required_qualification,
				labor_hours = EXCLUDED.labor_hours,
				priority_weight = EXCLUDED.priority_weight,
				forecast_cost = EXCLUDED.forecast_cost,
				assigned_month = EXCLUDED.assigned_month
		`, a.RequestID, a.MachineID, a.Model, a.RepairType, a.RequiredQualification, a.LaborHours, a.PriorityWeight, a.ForecastCost, a.AssignedMonth); err != nil {
			return nil, err
		}
		out = append(out, AnnualPlanRow{
			RequestID:             a.RequestID,
			MachineID:             a.MachineID,
			Model:                 a.Model,
			RepairType:            a.RepairType,
			RequiredQualification: a.RequiredQualification,
			LaborHours:            a.LaborHours,
			PriorityWeight:        a.PriorityWeight,
			ForecastCost:          a.ForecastCost,
			AssignedMonth:         a.AssignedMonth,
		})
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Server) loadAnnualRequests(ctx context.Context) ([]annualReq, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT request_id, machine_id, model, repair_type, priority_weight, motohours_at_request, forecast_cost,
		       critical_parts_required, required_qualification, desired_month, notes
		FROM repair_requests
		WHERE request_status = 'новая'
		ORDER BY priority_weight DESC, COALESCE(desired_month, 13), request_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]annualReq, 0)
	for rows.Next() {
		var r annualReq
		if err := rows.Scan(&r.RequestID, &r.MachineID, &r.Model, &r.RepairType, &r.PriorityWeight, &r.MotoHoursAtRequest, &r.ForecastCost, &r.CriticalPartsRequired, &r.RequiredQualification, &r.DesiredMonth, &r.Notes); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Server) loadTechCards(ctx context.Context) (map[string]techCard, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT repair_type, machine_id, labor_hours, required_qualification
		FROM repair_tech_cards
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := map[string]techCard{}
	for rows.Next() {
		var t techCard
		if err := rows.Scan(&t.RepairType, &t.MachineID, &t.LaborHours, &t.RequiredQualification); err != nil {
			return nil, err
		}
		m[key2(t.RepairType, t.MachineID)] = t
	}
	return m, rows.Err()
}

func (s *Server) loadMonthlyResources(ctx context.Context) (map[int]monthRes, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT month_no, COALESCE(available_hours,0), COALESCE(budget,0), COALESCE(max_units_in_repair,0), critical_parts_available
		FROM monthly_resources
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := map[int]monthRes{}
	for rows.Next() {
		var r monthRes
		if err := rows.Scan(&r.MonthNo, &r.AvailableHours, &r.Budget, &r.MaxUnitsInRepair, &r.CriticalPartsAvailable); err != nil {
			return nil, err
		}
		m[r.MonthNo] = r
	}
	return m, rows.Err()
}

func solveAnnualAssignments(reqs []annualReq, tech map[string]techCard, res map[int]monthRes) ([]annualAssignment, error) {
	hours := make(map[int]int, 12)
	budget := make(map[int]float64, 12)
	units := make(map[int]int, 12)

	for i := 1; i <= 12; i++ {
		r := res[i]
		hours[i] = r.AvailableHours
		budget[i] = r.Budget
		units[i] = r.MaxUnitsInRepair
	}

	bestScore, bestCount, bestMonthSum := -1, -1, 1<<30
	var best []int

	suffix := make([]int, len(reqs)+1)
	for i := len(reqs) - 1; i >= 0; i-- {
		suffix[i] = suffix[i+1] + reqs[i].PriorityWeight
	}

	var dfs func(i, score, count, monthSum int, assigns []int)
	dfs = func(i, score, count, monthSum int, assigns []int) {
		if score+suffix[i] < bestScore {
			return
		}

		if i == len(reqs) {
			if score > bestScore ||
				(score == bestScore && count > bestCount) ||
				(score == bestScore && count == bestCount && monthSum < bestMonthSum) {
				bestScore, bestCount, bestMonthSum = score, count, monthSum
				best = append([]int(nil), assigns...)
			}
			return
		}

		r := reqs[i]
		tc, ok := tech[key2(r.RepairType, r.MachineID)]
		if !ok || tc.RequiredQualification < r.RequiredQualification {
			assigns = append(assigns, -1)
			dfs(i+1, score, count, monthSum, assigns)
			return
		}

		assigns = append(assigns, -1)
		dfs(i+1, score, count, monthSum, assigns)
		assigns = assigns[:len(assigns)-1]

		startMonth := 1
		if r.DesiredMonth.Valid && r.DesiredMonth.Int64 >= 1 && r.DesiredMonth.Int64 <= 12 {
			startMonth = int(r.DesiredMonth.Int64)
		}

		reqCost := float64Value(r.ForecastCost)

		for m := startMonth; m <= 12; m++ {
			rr, ok := res[m]
			if !ok || rr.MonthNo == 0 {
				continue
			}

			if tc.LaborHours > hours[m] {
				continue
			}
			if reqCost > budget[m] {
				continue
			}
			if units[m] <= 0 {
				continue
			}
			if boolText(r.CriticalPartsRequired) && !boolText(rr.CriticalPartsAvailable) {
				continue
			}

			hours[m] -= tc.LaborHours
			budget[m] -= reqCost
			units[m]--

			assigns = append(assigns, m)
			dfs(i+1, score+r.PriorityWeight, count+1, monthSum+m, assigns)
			assigns = assigns[:len(assigns)-1]

			hours[m] += tc.LaborHours
			budget[m] += reqCost
			units[m]++
		}
	}

	dfs(0, 0, 0, 0, nil)

	if best == nil {
		return nil, nil
	}

	out := make([]annualAssignment, 0, len(best))
	for i, m := range best {
		if m <= 0 {
			continue
		}

		tc, ok := tech[key2(reqs[i].RepairType, reqs[i].MachineID)]
		if !ok {
			continue
		}

		out = append(out, annualAssignment{
			RequestID:             reqs[i].RequestID,
			MachineID:             reqs[i].MachineID,
			Model:                 reqs[i].Model,
			RepairType:            reqs[i].RepairType,
			RequiredQualification: reqs[i].RequiredQualification,
			LaborHours:            tc.LaborHours,
			PriorityWeight:        reqs[i].PriorityWeight,
			ForecastCost:          float64Value(reqs[i].ForecastCost),
			AssignedMonth:         m,
		})
	}

	return out, nil
}

func key2(a, b string) string { return a + "||" + b }

func (s *Server) listAnnualPlan(ctx context.Context) ([]AnnualPlanRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT request_id, machine_id, model, repair_type, required_qualification, labor_hours, priority_weight, forecast_cost, assigned_month
		FROM repair_plan
		ORDER BY priority_weight DESC, assigned_month, request_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]AnnualPlanRow, 0)
	for rows.Next() {
		var r AnnualPlanRow
		if err := rows.Scan(&r.RequestID, &r.MachineID, &r.Model, &r.RepairType, &r.RequiredQualification, &r.LaborHours, &r.PriorityWeight, &r.ForecastCost, &r.AssignedMonth); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// =========================================================
// 6.3 — материалы
// =========================================================

type repairPlanRow struct {
	RequestID     string
	MachineID     string
	Model         string
	RepairType    string
	AssignedMonth int
}

type materialNormRow struct {
	RepairType   string
	Model        string
	MaterialCode string
	Consumption  float64
}

type materialRow struct {
	MaterialCode string
	MaterialName string
	UnitID       int
	UnitSymbol   string
}

func (s *Server) solveMaterialDemand(ctx context.Context, targetMonth int) ([]MaterialDemandRow, error) {
	if targetMonth < 1 || targetMonth > 12 {
		return nil, errors.New("target_month must be 1..12")
	}

	plans, err := s.loadPlansByMonth(ctx, targetMonth)
	if err != nil {
		return nil, err
	}
	norms, err := s.loadMaterialNorms(ctx)
	if err != nil {
		return nil, err
	}
	materials, err := s.loadMaterials(ctx)
	if err != nil {
		return nil, err
	}

	type agg struct {
		name string
		unit string
		qty  float64
	}
	acc := map[string]*agg{}

	for _, p := range plans {
		for _, n := range norms[key2(p.RepairType, p.Model)] {
			mat, ok := materials[n.MaterialCode]
			if !ok {
				continue
			}
			a := acc[n.MaterialCode]
			if a == nil {
				a = &agg{name: mat.MaterialName, unit: mat.UnitSymbol}
				acc[n.MaterialCode] = a
			}
			a.qty += n.Consumption
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM material_demand WHERE target_month = $1`, targetMonth); err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(acc))
	for k := range acc {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]MaterialDemandRow, 0, len(keys))
	for _, code := range keys {
		a := acc[code]
		notes := "расчет по годовому плану ремонтов"
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO material_demand (target_month, material_code, demand_quantity, notes)
			VALUES ($1,$2,$3,$4)
			ON CONFLICT (target_month, material_code) DO UPDATE SET
				demand_quantity = EXCLUDED.demand_quantity,
				notes = EXCLUDED.notes
		`, targetMonth, code, a.qty, notes); err != nil {
			return nil, err
		}
		out = append(out, MaterialDemandRow{
			MaterialCode:   code,
			MaterialName:   a.name,
			Unit:           a.unit,
			DemandQuantity: a.qty,
			Notes:          notes,
		})
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Server) loadPlansByMonth(ctx context.Context, month int) ([]repairPlanRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT request_id, machine_id, model, repair_type, assigned_month
		FROM repair_plan
		WHERE assigned_month = $1
	`, month)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]repairPlanRow, 0)
	for rows.Next() {
		var r repairPlanRow
		if err := rows.Scan(&r.RequestID, &r.MachineID, &r.Model, &r.RepairType, &r.AssignedMonth); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Server) loadMaterialNorms(ctx context.Context) (map[string][]materialNormRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT repair_type, model, material_code, consumption_per_repair
		FROM material_norms
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string][]materialNormRow{}
	for rows.Next() {
		var r materialNormRow
		if err := rows.Scan(&r.RepairType, &r.Model, &r.MaterialCode, &r.Consumption); err != nil {
			return nil, err
		}
		out[key2(r.RepairType, r.Model)] = append(out[key2(r.RepairType, r.Model)], r)
	}
	return out, rows.Err()
}

func (s *Server) loadMaterials(ctx context.Context) (map[string]materialRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.material_code, m.material_name, m.unit_id, COALESCE(u.unit_symbol, '')
		FROM materials m
		LEFT JOIN units u ON u.unit_id = m.unit_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]materialRow{}
	for rows.Next() {
		var r materialRow
		if err := rows.Scan(&r.MaterialCode, &r.MaterialName, &r.UnitID, &r.UnitSymbol); err != nil {
			return nil, err
		}
		out[r.MaterialCode] = r
	}
	return out, rows.Err()
}

// =========================================================
// 6.4 — назначение бригад
// =========================================================

type monthlyPlanRow struct {
	RequestID              string
	MachineID              string
	Model                  string
	RepairType             string
	RequiredSpecialization string
	RequiredQualification  int
	PlannedStartDate       sql.NullTime
	PlannedEndDate         sql.NullTime
	LaborHours             int
	PriorityWeight         int
	ReadinessStatus        string
	Notes                  sql.NullString
}

type brigadeRow struct {
	BrigadeNumber   string
	TeamComposition string
	Specialization  string
	Qualification   int
	Contact         sql.NullString
	Notes           sql.NullString
}

type availabilityRow struct {
	AvailabilityID       int
	BrigadeNumber        string
	AvailableStart       time.Time
	AvailableEnd         time.Time
	AvailableHours       sql.NullInt64
	CurrentAssignedHours int
	Contact              sql.NullString
	Notes                sql.NullString
}

func (s *Server) solveBrigadeAssignments(ctx context.Context, month *int) ([]BrigadeAssignmentRow, error) {
	plans, err := s.loadMonthlyPlans(ctx, month)
	if err != nil {
		return nil, err
	}
	brigades, err := s.loadBrigades(ctx)
	if err != nil {
		return nil, err
	}
	avail, err := s.loadAvailabilities(ctx)
	if err != nil {
		return nil, err
	}

	sort.Slice(plans, func(i, j int) bool {
		if plans[i].PriorityWeight != plans[j].PriorityWeight {
			return plans[i].PriorityWeight > plans[j].PriorityWeight
		}
		if plans[i].PlannedStartDate.Valid && plans[j].PlannedStartDate.Valid && !plans[i].PlannedStartDate.Time.Equal(plans[j].PlannedStartDate.Time) {
			return plans[i].PlannedStartDate.Time.Before(plans[j].PlannedStartDate.Time)
		}
		return plans[i].RequestID < plans[j].RequestID
	})

	// build candidate lists
	type candidate struct {
		AvailID int
		Score   int
	}
	cands := make([][]candidate, len(plans))
	for i, p := range plans {
		for _, a := range avail {
			b := brigades[a.BrigadeNumber]
			if b.BrigadeNumber == "" {
				continue
			}
			if !brigadeMatches(p.RequiredSpecialization, p.RequiredQualification, b.Specialization, b.Qualification) {
				continue
			}
			if p.PlannedStartDate.Valid && a.AvailableStart.After(p.PlannedStartDate.Time) {
				continue
			}
			if p.PlannedEndDate.Valid && a.AvailableEnd.Before(p.PlannedEndDate.Time) {
				continue
			}
			rest := 0
			if a.AvailableHours.Valid {
				rest = int(a.AvailableHours.Int64) - a.CurrentAssignedHours
			}
			if rest < p.LaborHours {
				continue
			}
			score := rest - p.LaborHours
			cands[i] = append(cands[i], candidate{AvailID: a.AvailabilityID, Score: score})
		}
		sort.Slice(cands[i], func(a, b int) bool {
			if cands[i][a].Score != cands[i][b].Score {
				return cands[i][a].Score < cands[i][b].Score
			}
			return cands[i][a].AvailID < cands[i][b].AvailID
		})
	}

	remaining := make(map[int]int, len(avail))
	for _, a := range avail {
		if a.AvailableHours.Valid {
			remaining[a.AvailabilityID] = int(a.AvailableHours.Int64) - a.CurrentAssignedHours
		} else {
			remaining[a.AvailabilityID] = 0
		}
	}

	bestScore, bestCount, bestUsed := -1, -1, 1<<30
	var best map[int]int

	suffix := make([]int, len(plans)+1)
	for i := len(plans) - 1; i >= 0; i-- {
		suffix[i] = suffix[i+1] + plans[i].PriorityWeight
	}

	var dfs func(i, score, count, used int, assigned map[int]int)
	dfs = func(i, score, count, used int, assigned map[int]int) {
		if score+suffix[i] < bestScore {
			return
		}
		if i == len(plans) {
			if score > bestScore || (score == bestScore && count > bestCount) || (score == bestScore && count == bestCount && used < bestUsed) {
				bestScore, bestCount, bestUsed = score, count, used
				best = make(map[int]int, len(assigned))
				for k, v := range assigned {
					best[k] = v
				}
			}
			return
		}

		// skip
		dfs(i+1, score, count, used, assigned)

		p := plans[i]
		for _, c := range cands[i] {
			if remaining[c.AvailID] < p.LaborHours {
				continue
			}
			remaining[c.AvailID] -= p.LaborHours
			assigned[i] = c.AvailID
			dfs(i+1, score+p.PriorityWeight, count+1, used+c.Score, assigned)
			delete(assigned, i)
			remaining[c.AvailID] += p.LaborHours
		}
	}

	dfs(0, 0, 0, 0, map[int]int{})

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM repair_assignments`); err != nil {
		return nil, err
	}

	out := make([]BrigadeAssignmentRow, 0)
	for i, p := range plans {
		aid, ok := best[i]
		if !ok {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO repair_assignments
					(request_id, machine_id, start_date, end_date, brigade_number, planned_hours, responsible_person, assignment_status, notes)
				VALUES ($1,$2,NULL,NULL,NULL,NULL,NULL,'не назначена',$3)
				ON CONFLICT (request_id) DO UPDATE SET
					machine_id = EXCLUDED.machine_id,
					start_date = EXCLUDED.start_date,
					end_date = EXCLUDED.end_date,
					brigade_number = EXCLUDED.brigade_number,
					planned_hours = EXCLUDED.planned_hours,
					responsible_person = EXCLUDED.responsible_person,
					assignment_status = EXCLUDED.assignment_status,
					notes = EXCLUDED.notes
			`, p.RequestID, p.MachineID, noteForUnassigned(p)); err != nil {
				return nil, err
			}
			out = append(out, BrigadeAssignmentRow{
				RequestID:        p.RequestID,
				MachineID:        p.MachineID,
				Model:            p.Model,
				RepairType:       p.RepairType,
				AssignmentStatus: "не назначена",
				Notes:            noteForUnassigned(p),
			})
			continue
		}

		a := availByID(avail, aid)
		b := brigades[a.BrigadeNumber]
		resp := firstWord(b.TeamComposition)

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO repair_assignments
				(request_id, machine_id, start_date, end_date, brigade_number, planned_hours, responsible_person, assignment_status, notes)
			VALUES ($1,$2,$3,$4,$5,$6,$7,'назначена',$8)
			ON CONFLICT (request_id) DO UPDATE SET
				machine_id = EXCLUDED.machine_id,
				start_date = EXCLUDED.start_date,
				end_date = EXCLUDED.end_date,
				brigade_number = EXCLUDED.brigade_number,
				planned_hours = EXCLUDED.planned_hours,
				responsible_person = EXCLUDED.responsible_person,
				assignment_status = EXCLUDED.assignment_status,
				notes = EXCLUDED.notes
		`, p.RequestID, p.MachineID, nullDate(p.PlannedStartDate), nullDate(p.PlannedEndDate), a.BrigadeNumber, p.LaborHours, resp, nil); err != nil {
			return nil, err
		}

		start := ""
		end := ""
		if p.PlannedStartDate.Valid {
			start = p.PlannedStartDate.Time.Format("2006-01-02")
		}
		if p.PlannedEndDate.Valid {
			end = p.PlannedEndDate.Time.Format("2006-01-02")
		}
		out = append(out, BrigadeAssignmentRow{
			RequestID:         p.RequestID,
			MachineID:         p.MachineID,
			Model:             p.Model,
			RepairType:        p.RepairType,
			StartDate:         start,
			EndDate:           end,
			BrigadeNumber:     a.BrigadeNumber,

			Specialization:    b.Specialization,
			PlannedHours:      p.LaborHours,
			ResponsiblePerson: resp,
			AssignmentStatus:  "назначена",
			Notes:             "",
		})
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func noteForUnassigned(p monthlyPlanRow) string {
	return "не назначена: нет подходящей бригады или не хватает времени/квалификации"
}

func (s *Server) loadMonthlyPlans(ctx context.Context, month *int) ([]monthlyPlanRow, error) {
	base := `
		SELECT request_id, machine_id, model, repair_type, required_specialization, required_qualification,
		       planned_start_date, planned_end_date, labor_hours, priority_weight, readiness_status, notes
		FROM monthly_repair_plan
		WHERE readiness_status = 'готова'
	`
	args := make([]any, 0)
	if month != nil && *month >= 1 && *month <= 12 {
		base += ` AND planned_start_date IS NOT NULL AND EXTRACT(MONTH FROM planned_start_date) = $1`
		args = append(args, *month)
	}
	base += ` ORDER BY priority_weight DESC, planned_start_date, request_id`

	rows, err := s.db.QueryContext(ctx, base, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]monthlyPlanRow, 0)
	for rows.Next() {
		var r monthlyPlanRow
		if err := rows.Scan(&r.RequestID, &r.MachineID, &r.Model, &r.RepairType, &r.RequiredSpecialization, &r.RequiredQualification, &r.PlannedStartDate, &r.PlannedEndDate, &r.LaborHours, &r.PriorityWeight, &r.ReadinessStatus, &r.Notes); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}


func (s *Server) loadBrigades(ctx context.Context) (map[string]brigadeRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT brigade_number, team_composition, specialization, qualification, contact, notes
		FROM brigades
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]brigadeRow{}
	for rows.Next() {
		var r brigadeRow
		if err := rows.Scan(&r.BrigadeNumber, &r.TeamComposition, &r.Specialization, &r.Qualification, &r.Contact, &r.Notes); err != nil {
			return nil, err
		}
		out[r.BrigadeNumber] = r
	}
	return out, rows.Err()
}

func (s *Server) loadAvailabilities(ctx context.Context) ([]availabilityRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT availability_id, brigade_number, available_start, available_end, available_hours, current_assigned_hours, contact, notes
		FROM brigade_availability
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]availabilityRow, 0)
	for rows.Next() {
		var r availabilityRow
		if err := rows.Scan(&r.AvailabilityID, &r.BrigadeNumber, &r.AvailableStart, &r.AvailableEnd, &r.AvailableHours, &r.CurrentAssignedHours, &r.Contact, &r.Notes); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func brigadeMatches(reqSpec string, reqQual int, spec string, qual int) bool {
	if spec != reqSpec && spec != "универсальная" {
		return false
	}
	return qual >= reqQual
}

func availByID(list []availabilityRow, id int) availabilityRow {
	for _, v := range list {
		if v.AvailabilityID == id {
			return v
		}
	}
	return availabilityRow{}
}

func (s *Server) listBrigadeAssignments(ctx context.Context, month *int) ([]BrigadeAssignmentRow, error) {
	query := `
		SELECT a.request_id, a.machine_id, mp.model, mp.repair_type,
		       COALESCE(a.start_date::text, ''), COALESCE(a.end_date::text, ''),
		       COALESCE(a.brigade_number, ''), COALESCE(b.specialization, ''), COALESCE(a.planned_hours, 0),
		       COALESCE(a.responsible_person, ''), COALESCE(a.assignment_status, ''), COALESCE(a.notes, '')
		FROM repair_assignments a
		LEFT JOIN monthly_repair_plan mp ON mp.request_id = a.request_id
		LEFT JOIN brigades b ON b.brigade_number = a.brigade_number
	`
	args := make([]any, 0)
	if month != nil && *month >= 1 && *month <= 12 {
		query += ` WHERE mp.planned_start_date IS NOT NULL AND EXTRACT(MONTH FROM mp.planned_start_date) = $1`
		args = append(args, *month)
	}
	query += ` ORDER BY a.request_id`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]BrigadeAssignmentRow, 0)
	for rows.Next() {
		var r BrigadeAssignmentRow
		if err := rows.Scan(&r.RequestID, &r.MachineID, &r.Model, &r.RepairType, &r.StartDate, &r.EndDate, &r.BrigadeNumber, &r.Specialization, &r.PlannedHours, &r.ResponsiblePerson, &r.AssignmentStatus, &r.Notes); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// =========================================================
// Small aliases for client convenience
// =========================================================

func (s *Server) listAnnualPlanAlias(ctx context.Context) ([]AnnualPlanRow, error) { return s.listAnnualPlan(ctx) }

